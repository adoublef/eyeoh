package http

import (
	"errors"
	"net/http"

	dberrors "go.adoublef/up/internal/database/errors"
	"go.adoublef/up/internal/runtime/debug"
)

// Error replies to the request with the specified error.
// If err implements [statusHandler] we use it's [statusHandler.ServeHTTP] method instead.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	if sh, ok := err.(statusHandler); ok {
		sh.ServeHTTP(w, r)
		return
	}
	sh := statusHandler{code: http.StatusInternalServerError}
	switch {
	case errors.Is(err, dberrors.ErrNotExist):
		// append path? or this will be handled by tempo anyway
		sh.code = http.StatusNotFound
	}
	sh.ServeHTTP(w, r)
	debug.Printf("net/http: %T, %v = err", err, err)
}
