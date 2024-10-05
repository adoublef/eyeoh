package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.adoublef/eyeoh/internal/runtime/debug"
)

var errRequestBodyEOF = statusHandler{
	code: http.StatusUnauthorized,
	s:    "request body could not be read properly",
}

var errRequestJSON = statusHandler{
	code: http.StatusUnsupportedMediaType,
	s:    "request body could not be read properly",
}

var errRequestJSONStream = statusHandler{
	code: http.StatusBadRequest,
	s:    "request body contains more than a single JSON object",
}

var errRequestTimeout = statusHandler{
	code: http.StatusRequestTimeout,
	s:    "failed to process request in time, please try again",
}

var errRequestJSONSyntax = func(format string, v ...any) error {
	return statusHandler{code: http.StatusBadRequest, s: fmt.Sprintf(format, v...)}
}

var errRequestTooLarge = func(format string, v ...any) error {
	return statusHandler{code: http.StatusRequestEntityTooLarge, s: fmt.Sprintf(format, v...)}
}

// Decode reads the next JSON-encoded value from a [http.Request] and returns the value is valid.
//
// If sz or d is set, the max bytes and read deadline of the [http.Request] can be modified, respectively.
func Decode[V any](w http.ResponseWriter, r *http.Request, sz int, d time.Duration) (V, error) {
	var v V
	if r.Body == nil {
		return v, errRequestBodyEOF
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !(mt == ContentTypJSON) {
		return v, errRequestJSON
	}
	if sz > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, int64(sz))
		debug.Printf("r.Body = http.MaxBytesReader(w, r.Body, %d)", sz)
	}

	if d > 0 {
		rc := http.NewResponseController(w)
		err = rc.SetReadDeadline(time.Now().Add(d))
		debug.Printf("%s := rc.SetReadDeadline(time.Now().Add(%v))", err, d)
		if err != nil {
			// note: if action not allowed, should maybe wrap this
			return v, err
		}
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // important
	if err := dec.Decode(&v); err != nil {
		debug.Printf("%v := dec.Decode(&v)", err)
		var zero V
		switch {
		// In some circumstances Decode() may also return an
		// io.ErrUnexpectedEOF error for syntax errors in the JSON. There
		// is an open issue regarding this at
		// https://github.com/golang/go/issues/25956.
		case errors.As(err, new(*json.SyntaxError)):
			se := err.(*json.SyntaxError)
			ch, _ := utf8.DecodeRune([]byte(se.Error()[19:]))
			return zero, errRequestJSONSyntax("invalid character '%c' at position %d", ch, se.Offset)
		case errors.As(err, new(*json.UnmarshalTypeError)):
			e := err.(*json.UnmarshalTypeError)
			return zero, errRequestJSONSyntax("unexpected %s for field %q at position %d", e.Value, e.Field, e.Offset)
		// There is an open issue at https://github.com/golang/go/issues/29035
		// regarding turning this into a sentinel error.
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			return zero, errRequestJSONSyntax("unknown field %s", err.Error()[20:])
		// An io.EOF error is returned by Decode() if the request body is empty.
		case errors.Is(err, io.EOF):
			return zero, errRequestBodyEOF
		case errors.As(err, new(*http.MaxBytesError)):
			return zero, errRequestTooLarge("maximum allowed request size is %s", strconv.Itoa(sz))
		case errors.As(err, new(*net.OpError)):
			return zero, errRequestTimeout
		// Otherwise default to logging the error and sending a 500 Internal
		// Server Error response. May want to wrap this error.
		default:
			return zero, errRequestJSONSyntax("encoding error: %v", err)
		}
	}
	// note: log error as this will not be returned to the client
	// Call decode again, using a pointer to an empty anonymous struct as
	// the destination. If the request body only contained a single JSON
	// object this will return an io.EOF error. So if we get anything else,
	// we know that there is additional data in the request body.
	if err = dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		// fixme: 4xx
		return *new(V), errRequestJSONStream
	}
	return v, nil
}
