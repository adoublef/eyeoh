package http

import (
	"encoding/json"
	"net/http"

	"go.adoublef/eyeoh/internal/runtime/debug"
)

func respond[V any](w http.ResponseWriter, _ *http.Request, v V) {
	err := json.NewEncoder(w).Encode(v)
	debug.Printf(`%v = json.NewEncoder(w).Encode(%#v)`, err, v)
}
