package core

import (
	"encoding/json"
	"net/http"
	"strings"
)

func WriteJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func WantsJSON(r *http.Request) bool {
	a := r.Header.Get("Accept")
	return strings.Contains(a, "application/json")
}

func IsBrowserForm(r *http.Request) bool {
	return !WantsJSON(r) && strings.Contains(r.Header.Get("Content-Type"), "multipart/")
}
