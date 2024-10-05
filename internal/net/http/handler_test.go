package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/toxics"
	"github.com/google/uuid"
	. "go.adoublef/eyeoh/internal/net/http"
	"go.adoublef/eyeoh/internal/testing/is"
)

func Test_handleFileInfo(t *testing.T) {
	t.Run("IsDir", func(t *testing.T) {
		// create the file
		c, ctx := newClient(t), context.Background()

		var fileIDs = make(chan string, 1)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()

			res, err := c.PostFormFile(ctx, "POST /touch/files", "testdata/hello.txt")
			is.OK(t, err) // return file upload response
			is.Equal(t, res.StatusCode, http.StatusOK)

			var file struct {
				ID string `json:"fileId"`
			}
			err = json.NewDecoder(res.Body).Decode(&file)
			is.OK(t, err) // decode json payload
			is.OK(t, res.Body.Close())

			fileIDs <- file.ID
		}()
		// create a folder
		go func() {
			defer wg.Done()

			res, err := c.Do(ctx, "POST /mkdir/files", strings.NewReader(`{"name":"src"}`), ctJSON, acceptAll)
			is.OK(t, err) // return file upload response
			is.Equal(t, res.StatusCode, http.StatusOK)

			var file struct {
				ID string `json:"folderId"`
			}
			err = json.NewDecoder(res.Body).Decode(&file)
			is.OK(t, err) // decode json payload
			is.OK(t, res.Body.Close())

			fileIDs <- file.ID
		}()

		go func() {
			wg.Wait()
			close(fileIDs)
		}()

		var isDir atomic.Int64
		wg.Add(2)
		for fileID := range fileIDs {
			go func() {
				defer wg.Done()

				res, err := c.Do(ctx, "GET /info/files/"+fileID, nil, ctJSON, acceptAll)
				is.OK(t, err) // return download response
				is.Equal(t, res.StatusCode, http.StatusOK)

				var info struct {
					IsDir bool `json:"isDir"`
				}
				err = json.NewDecoder(res.Body).Decode(&info)
				is.OK(t, err) // decode json payload
				is.OK(t, res.Body.Close())
				// better way to do this?
				if info.IsDir {
					isDir.Add(1)
				}
			}()
		}
		wg.Wait()
		is.Equal(t, isDir.Load(), 1)
	})

	t.Run("ErrNotFound", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.Do(ctx, "GET /info/files/"+uuid.NewString(), nil, ctJSON, acceptAll)
		is.OK(t, err) // return download response
		is.Equal(t, res.StatusCode, http.StatusNotFound)
	})
}

func Test_handleCreateFolder(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		body := `{"name":"src"}`

		res, err := c.Do(ctx, "POST /mkdir/files", strings.NewReader(body), ctJSON, acceptAll)
		is.OK(t, err) // return file upload response
		is.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("ErrBadName", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		body := `{"name":""}`

		res, err := c.Do(ctx, "POST /mkdir/files", strings.NewReader(body), ctJSON, acceptAll)
		is.OK(t, err) // return file upload response
		is.Equal(t, res.StatusCode, http.StatusBadRequest)
	})

	// cannot download a folder? needs to use the archive endpoint

	t.Run("ErrForbidden", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.Do(ctx, "POST /mkdir/files", strings.NewReader(`{"name":"src"}`), ctJSON, acceptAll)
		is.OK(t, err) // return file upload response
		is.Equal(t, res.StatusCode, http.StatusOK)

		var file struct {
			ID string `json:"folderId"`
		}
		err = json.NewDecoder(res.Body).Decode(&file)
		is.OK(t, err) // decode json payload
		is.OK(t, res.Body.Close())

		res, err = c.Do(ctx, "GET /files/"+file.ID, nil, acceptAll)
		is.OK(t, err)                                     // return download response
		is.Equal(t, res.StatusCode, http.StatusForbidden) // cannot use endpoint to download a directory
	})
}

func Test_handleFileUpload(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.PostFormFile(ctx, "POST /touch/files", "testdata/hello.txt")
		is.OK(t, err) // return file upload response
		is.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("ErrExist", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		// only one request should pass
		var ok, conflict atomic.Int64
		var wg sync.WaitGroup
		wg.Add(2)
		for range 2 {
			go func() {
				defer wg.Done()
				res, err := c.PostFormFile(ctx, "POST /touch/files", "testdata/hello.txt")
				is.OK(t, err) // return file upload response
				switch res.StatusCode {
				case http.StatusOK:
					ok.Add(1)
				// error status code should be conflict
				case http.StatusConflict:
					conflict.Add(1)
				default:
					t.Logf("%d = res.StatusCode", res.StatusCode)
				}
			}()
		}
		wg.Wait()

		is.Equal(t, ok.Load(), 1)
		is.Equal(t, conflict.Load(), 1)
	})
}

func Test_handleFileDownload(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.PostFormFile(ctx, "POST /touch/files", "testdata/hello.txt")
		is.OK(t, err) // return upload response
		is.Equal(t, res.StatusCode, http.StatusOK)

		var file struct {
			ID string `json:"fileId"`
		}
		err = json.NewDecoder(res.Body).Decode(&file)
		is.OK(t, err) // decode json payload

		res, err = c.Do(ctx, "GET /files/"+file.ID, nil, acceptAll)
		is.OK(t, err) // return download response
		is.Equal(t, res.StatusCode, http.StatusOK)

		n, err := io.Copy(io.Discard, res.Body)
		is.OK(t, err) // read content into discard
		is.OK(t, res.Body.Close())
		is.Equal(t, n, 14) // got;want
		t.Logf(`%d, _ := io.Copy(io.Discard, res.Body)`, n)
	})

	t.Run("ErrNotFound", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.Do(ctx, "GET /files/"+uuid.NewString(), nil, acceptAll)
		is.OK(t, err) // return download response
		is.Equal(t, res.StatusCode, http.StatusNotFound)
	})
}

var acceptAll = func(r *http.Request) { r.Header.Set("Accept", "*/*") }

func Test_handleReady(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.Do(ctx, "GET /ready", nil, acceptAll)
		is.OK(t, err) // return echo response
		is.Equal(t, res.StatusCode, http.StatusOK)
	})
}

func newClient(tb testing.TB) *TestClient {
	tb.Helper()

	var (
		fsys = newTestFS(tb)
	)
	// high burst, short ttl
	tc := newTestClient(tb, Handler(10, 200*time.Millisecond, fsys))
	// https://speed.cloudflare.com/
	bu, err := tc.AddToxic("bandwidth", true, &toxics.BandwidthToxic{Rate: 72.8 * 1000})
	is.OK(tb, err) // return bandwidth upstream toxic
	lu, err := tc.AddToxic("latency", true, &toxics.LatencyToxic{Latency: 150, Jitter: 42})
	is.OK(tb, err) // return bandwidth upstream toxic
	bd, err := tc.AddToxic("bandwidth", false, &toxics.BandwidthToxic{Rate: 18.4 * 1000})
	is.OK(tb, err) // return bandwidth upstream toxic
	ld, err := tc.AddToxic("latency", false, &toxics.LatencyToxic{Latency: 30, Jitter: 8})
	is.OK(tb, err) // return bandwidth upstream toxic

	tb.Cleanup(func() {
		for _, name := range []string{bu, lu, bd, ld} {
			err := tc.RemoveToxic(name)
			tb.Logf(`%v := tc.RemoveToxic(%q)`, err, name)
		}
	})

	return tc
}
