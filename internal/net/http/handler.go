package http

import (
	"net/http"
	"time"

	"go.adoublef/up/internal/fs"
	olog "go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

const scopeName = "go.adoublef/up/internal/net/http"

var (
	tracer = otel.Tracer(scopeName)
	_      = otel.Meter(scopeName)
	_      = olog.NewLogger(scopeName)
)

const (
	DefaultReadTimeout    = 100 * time.Second // Cloudflare's default read request timeout of 100s
	DefaultWriteTimeout   = 30 * time.Second  // Cloudflare's default write request timeout of 30s
	DefaultIdleTimeout    = 900 * time.Second // Cloudflare's default write request timeout of 900s
	DefaultMaxHeaderBytes = 32 * (1 << 10)
	DefaultMaxBytes       = 1 << 20 // Cloudflare's free tier limits of 100mb
)

type Server = http.Server

var ErrServerClosed = http.ErrServerClosed

func Handler(burst int, ttl time.Duration, fsys *fs.FS) http.Handler {
	mux := http.NewServeMux()
	handleFunc := func(pattern string, h http.Handler) {
		h = otelhttp.WithRouteTag(pattern, h)
		mux.Handle(pattern, h)
	}

	// only accepts JSON
	// set some defaults for JSON endpoints too
	JSON := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mustValue[string](r.Context(), ContentTypOfferKey) != ContentTypJSON {
				notAcceptableHandler.ServeHTTP(w, r)
				return
			}
			// use [Decode] to set payload size constraints
			// or in here?
			h.ServeHTTP(w, r)
		})
	}

	handleFunc("GET /ready", statusHandler{code: http.StatusOK})

	handleFunc("POST /touch/files", handleFileUpload(fsys))
	handleFunc("POST /mkdir/files", JSON(handleCreateFolder(fsys)))
	// GET /info/files/{files}
	handleFunc("GET /files/{file}", handleDownloadFile(fsys))
	// todo: MOVE
	// todo: COPY
	// todo: REMOVE

	h := AcceptHandler(mux)
	h = LimitHandler(h, burst, ttl)
	h = otelhttp.NewHandler(h, "Http")
	return h
}
