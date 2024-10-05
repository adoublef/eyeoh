package http

import (
	"net/http"
	"strconv"
	"time"

	"go.adoublef/up/internal/runtime/debug"
	"golang.org/x/time/rate"
)

var tooManyRequestsHandler = &statusHandler{
	code: http.StatusTooManyRequests,
	s:    `too many requests. please try again later`,
}

// LimitHandler returns a [http.Handler] than offers rate limiting of http request.
func LimitHandler(h http.Handler, burst int, ttl time.Duration) http.Handler {
	var l = rate.NewLimiter(rate.Every(ttl), burst)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("RateLimit-Limit", strconv.FormatFloat(float64(l.Limit()), 'f', 0, 64))
		w.Header().Add("RateLimit-Reset", "1") // note: 1 is fixed
		w.Header().Add("RateLimit-Remaining", strconv.FormatFloat(l.Tokens(), 'f', 0, 64))
		// throttle safe requests and limit non-safe requests
		if !l.Allow() {
			tooManyRequestsHandler.ServeHTTP(w, r)
			return
		}
		defer debug.Printf("LimitHandler: %0.f = l.Tokens()", l.Tokens())
		h.ServeHTTP(w, r)
	})
}
