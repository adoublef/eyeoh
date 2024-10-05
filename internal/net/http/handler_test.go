package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/toxics"
	"github.com/google/uuid"
	. "go.adoublef/up/internal/net/http"
	"go.adoublef/up/internal/testing/is"
)

func Test_handleCreateFolder(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		body := `{"name":"src"}`

		res, err := c.Do(ctx, "POST /mkdir/files", strings.NewReader(body), ctJSON, acceptAll)
		is.OK(t, err) // return file upload response
		is.Equal(t, res.StatusCode, http.StatusOK)
	})
}

func Test_handleFileUpload(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.PostFormFile(ctx, "POST /touch/files", "testdata/hello.txt")
		is.OK(t, err) // return file upload response
		is.Equal(t, res.StatusCode, http.StatusOK)
	})
}

func Test_handleFileDownload(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.PostFormFile(ctx, "POST /touch/files", "testdata/hello.txt")
		is.OK(t, err) // return upload response
		is.Equal(t, res.StatusCode, http.StatusOK)

		var completed struct {
			ID string `json:"fileId"`
		}
		err = json.NewDecoder(res.Body).Decode(&completed)
		is.OK(t, err) // decode json payload

		res, err = c.Do(ctx, "GET /files/"+completed.ID, nil, acceptAll)
		is.OK(t, err) // return download response
		is.Equal(t, res.StatusCode, http.StatusOK)

		n, err := io.Copy(io.Discard, res.Body)
		is.OK(t, err) // read content into discard
		is.OK(t, res.Body.Close())
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
