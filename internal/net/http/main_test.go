package http_test

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/cockroachdb"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"go.adoublef/eyeoh/internal/blob"
	"go.adoublef/eyeoh/internal/database/crdb"
	"go.adoublef/eyeoh/internal/fs"
	. "go.adoublef/eyeoh/internal/net/http"
	"go.adoublef/eyeoh/internal/net/http/httputil"
	"go.adoublef/eyeoh/internal/net/nettest"
	"go.adoublef/eyeoh/internal/testing/is"
	"go.adoublef/eyeoh/internal/testing/texttest"
)

//go:embed all:testdata/*
var embedFS embed.FS

// TestClient is configured for use within tests.
type TestClient struct {
	*http.Client
	*nettest.Proxy
	testing.TB
}

// PostFormFile issues a multipart POST to the specified URL, with a file as the request body.
func (tc *TestClient) PostFormFile(ctx context.Context, pattern string, filename string) (*http.Response, error) {
	f, err := embedFS.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to return file stat: %v", err)
	}

	pr, pw := io.Pipe()
	defer pr.Close()

	mw := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		defer mw.Close()

		part, err := mw.CreateFormFile("file", fi.Name())
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		// upload a single file
		// can this be extended to upload a directory?
		n, err := io.CopyN(part, f, fi.Size())
		tc.Logf(`%d, %v := io.CopyN(part, f, fi.Size())`, n, err)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	o := func(r *http.Request) {
		// set contentType
		r.Header.Set("Content-Type", mw.FormDataContentType())
		// set accept
		r.Header.Set("Accept", "*/*")
		// set encoding?
	}
	return tc.Do(ctx, pattern, pr, o)
}

// Do sends an HTTP request and returns an HTTP response. The pattern follows similar rules to [http.ServeMux] in Go1.23.
// Options can be applied to modify the [http.Request] before sending it.
func (tc *TestClient) Do(ctx context.Context, pattern string, body io.Reader, opts ...func(*http.Request)) (*http.Response, error) {
	method, _, path, err := httputil.ParsePattern(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pattern: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to return request: %v", err)
	}
	tc.Logf(`req, err := http.NewRequestWithContext(ctx, %q, %q, body)`, method, path)
	for _, o := range opts {
		o(req)
	}
	res, err := tc.Client.Do(req)
	tc.Logf(`res, %v := tc.Client.Do(req)`, err)
	return res, err
}

// newTestClient returns a new [TestClient] with the [httptest.Server] setup to near
// production levels. This server sits behind a proxy that can simulate network failures.
func newTestClient(tb testing.TB, h http.Handler) *TestClient {
	tb.Helper()

	ts := httptest.NewUnstartedServer(h)
	ts.Config.MaxHeaderBytes = DefaultMaxHeaderBytes
	// note: the client panics if readTimeout is less than the test timeout
	// is this a non-issue?
	ts.Config.ReadTimeout = DefaultReadTimeout
	ts.Config.WriteTimeout = DefaultWriteTimeout
	ts.Config.IdleTimeout = DefaultIdleTimeout
	ts.StartTLS()

	proxy := nettest.NewProxy("HTTP_"+tb.Name(), strings.TrimPrefix(ts.URL, "https://"))
	if tp, ok := ts.Client().Transport.(*http.Transport); ok {
		tp.DisableCompression = true
	}
	tc := nettest.WithTransport(ts.Client(), "https://"+proxy.Listen())
	return &TestClient{tc, proxy, tb}
}

// newTestFS returns a [fs.FS] for use within tests.
// Each dependency sits behind a proxy that can simulate network failures.
func newTestFS(tb testing.TB) *fs.FS {
	tb.Helper()
	ctx := context.Background()

	minioURL, err := compose.minio.ConnectionString(ctx)
	is.OK(tb, err) // return minio connetion string

	var (
		bucket = texttest.Bucket(61) // random
		region = "auto"

		user = compose.minio.Username
		pass = compose.minio.Password
		cred = credentials.NewStaticCredentialsProvider(user, pass, "")
	)

	conf, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region), config.WithCredentialsProvider(cred))
	is.OK(tb, err) // return minio configuration

	// Create S3 service c
	// add toxiproxy (MINIO_ + tb.Name)
	c := s3.NewFromConfig(conf, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://" + minioURL)
		o.UsePathStyle = true
	})

	// Create a new bucket using the CreateBucket call.
	// note: StatusCode: 409, BucketAlreadyOwnedByYou
	// note: StatusCode: 507, XMinioStorageFull
	p := &s3.CreateBucketInput{
		Bucket: &bucket,
	}
	_, err = c.CreateBucket(ctx, p)
	is.OK(tb, err) // create bucket

	crdbDSN, err := compose.crdb.ConnectionString(ctx)
	is.OK(tb, err) // return cockroachdb connection string

	is.OK(tb, crdb.Up(ctx, crdbDSN)) // run migration scripts
	tb.Cleanup(func() { is.OK(tb, crdb.Down(ctx, crdbDSN)) })

	crdbURL, err := url.Parse(crdbDSN)
	is.OK(tb, err) // parse cockroachdb url

	proxyCRDB := nettest.NewProxy("CRDB_"+tb.Name(), crdbURL.Host)
	tb.Cleanup(func() { is.OK(tb, proxyCRDB.Close()) })
	// postgres://<username>:<password>@<host>:<port>/<database>?<parameters>
	_, port, _ := strings.Cut(proxyCRDB.Listen(), ":")
	crdbURL.Host = "localhost:" + port
	/* create db connection */
	pool, err := pgxpool.New(ctx, crdbURL.String())
	is.OK(tb, err)
	tb.Cleanup(func() { pool.Close() })

	return &fs.FS{
		DB:         &fs.DB{RWC: pool},
		Uploader:   blob.NewUploader(bucket, c),
		Downloader: blob.NewDownloader(bucket, c),
	}
}

// compose is a global handler for containers required.
var compose struct {
	minio *minio.MinioContainer
	crdb  *cockroachdb.CockroachDBContainer
}

func TestMain(m *testing.M) {
	err := setup(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	err = cleanup(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}

// setup initialises containers within the pacakge.
func setup(ctx context.Context) (err error) {
	compose.minio, err = minio.Run(ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z")
	if err != nil {
		return err
	}
	compose.crdb, err = cockroachdb.Run(ctx, "cockroachdb/cockroach:v22.2.3")
	if err != nil {
		return
	}
	return
}

// cleanup stops all running containers for the pacakge.
func cleanup(ctx context.Context) (err error) {
	var cc = []testcontainers.Container{compose.minio, compose.crdb}
	for _, c := range cc {
		if c != nil {
			err = errors.Join(c.Terminate(ctx))
		}
	}
	return err
}
