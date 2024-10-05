package http_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	. "go.adoublef/up/internal/net/http"
	"go.adoublef/up/internal/testing/is"
)

func Test_LimitHandler(t *testing.T) {
	// for i in {1..6}; do curl http://localhost:8080/ping; done
	t.Run("OK", func(t *testing.T) {
		tc, ctx := newLimitClient(t, 1, 200*time.Millisecond), context.Background()

		res, err := tc.Do(ctx, "GET /ping", nil)
		is.OK(t, err)
		is.Equal(t, res.StatusCode, http.StatusNoContent) // got;want

		res, err = tc.Do(ctx, "GET /ping", nil)
		is.OK(t, err)
		is.Equal(t, res.StatusCode, http.StatusTooManyRequests) // got;want

		<-time.After(200 * time.Millisecond)

		res, err = tc.Do(ctx, "GET /ping", nil)
		is.OK(t, err)
		is.Equal(t, res.StatusCode, http.StatusNoContent)
	})
}

func newLimitClient(tb testing.TB, burst int, ttl time.Duration) *TestClient {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return newTestClient(tb, LimitHandler(mux, burst, ttl))
}
