package http

import (
	"errors"
	"net/http"

	"go.adoublef/eyeoh/internal/blob"
	dberrors "go.adoublef/eyeoh/internal/database/errors"
	"go.adoublef/eyeoh/internal/runtime/debug"
)

// Error replies to the request with the specified error.
// If err implements [statusHandler] we use it's [statusHandler.ServeHTTP] method instead.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	debug.Printf("net/http: %T, %v = err", err, err)
	if sh, ok := err.(statusHandler); ok {
		sh.ServeHTTP(w, r)
		return
	}
	sh := statusHandler{code: http.StatusInternalServerError}
	switch {
	// use-case? file created but failure occured so the header exists but the blob data does not.
	// look into solving this in relation to [blob.ErrNotExist]
	case errors.Is(err, dberrors.ErrNotExist) || errors.Is(err, blob.ErrNotExist):
		// append path? or this will be handled by tempo anyway
		sh.code = http.StatusNotFound
	case errors.Is(err, dberrors.ErrExist):
		sh.code = http.StatusConflict
	}
	sh.ServeHTTP(w, r)
}
