package blob

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"go.adoublef/eyeoh/internal/runtime/debug"
)

type Downloader struct {
	bucket string
	m      *manager.Downloader // just have this as a global atomic?
}

func (d *Downloader) Download(ctx context.Context, id uuid.UUID) (rc io.ReadCloser, mime string, err error) {
	s := strings.Replace(id.String(), "-", "", 4)
	uri := path.Join("_blob", s[:2], s[2:4], s[4:])
	// if the body is not used then this seems to not return an error
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		o := &s3.GetObjectInput{
			Key:    &uri,
			Bucket: &d.bucket,
			// PartNumber *string
			// Range *string
		}
		nw, err := d.m.Download(ctx, &writerAt{pw}, o)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		debug.Printf(`%d, err := d.m.Download(ctx, &writerAt{pw}, o)`, nw)
	}()
	// using [io.Pipe] will not actually start reading until its used
	// so we need to read some parts first so that the error from s3 manager
	// can be determined before the caller needs it (this seems to trick the endpoint into returning a 200 OK)
	// given we are reading some bytes, may as well attempt to grab the mime type too.
	br := newReadCloser(pr)
	p, err := br.Peek(512)
	// a file smaller than 512 should still be valid from this point on
	if err != nil && err != io.EOF {
		ec := br.Close()
		if ec != nil {
			err = errors.Join(ec)
		}
		return nil, "", Error(err)
	}
	debug.Printf(`p, _ := br.Peek(512)`)
	debug.Printf(`%d := len(p)`, len(p))
	return br, http.DetectContentType(p), nil
}

func NewDownloader(bucket string, c manager.DownloadAPIClient) *Downloader {
	d := manager.NewDownloader(c, func(u *manager.Downloader) {
		u.PartSize = 1 << 24 // 16MB
		// https://dev.to/flowup/using-io-reader-io-writer-in-go-to-stream-data-3i7b
		u.Concurrency = 1 // force sequential writes
	})
	return &Downloader{bucket, d}
}

type readCloser struct {
	*bufio.Reader
	io.Closer
}

func newReadCloser(r io.ReadCloser) *readCloser {
	return &readCloser{
		Reader: bufio.NewReader(r),
		Closer: r,
	}
}
