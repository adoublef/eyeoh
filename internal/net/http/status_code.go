package http

import (
	"io"
	"net/http"
	"strconv"
	"strings"
)

// statusHandler implments error, [fmt.Stringer] & [http.Handler]
type statusHandler struct {
	code int
	s    string
}

func (sh statusHandler) Error() string {
	// should be lowercase?
	return sh.String()
}

func (sh statusHandler) String() string {
	// do once? may not be neccessary tho
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(sh.code))
	sb.WriteRune(' ')
	sb.WriteString(sh.StatusText())
	if sh.s != "" {
		sb.WriteString(": ")
		sb.WriteString(sh.s)
	}
	// <code> <as_text>: <optional>
	return sb.String()
}

// StatusText returns a text for the HTTP status code.
func (sh statusHandler) StatusText() string {
	return http.StatusText(sh.code)
}

func (sh statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// carry [context.Context] throughout the lifetime of this handler?
	if sh.code >= 400 {
		http.Error(w, sh.StatusText(), sh.code)
		return
	}
	// also handle redirects if applicable
	if sc := sh.code; sc >= 300 && sc < 400 {
		// s is assumed to be a url
		http.Redirect(w, r, sh.s, sh.code)
		return
	}
	// dont really care about [sh.s]
	w.WriteHeader(sh.code)
	io.WriteString(w, sh.StatusText())
}
