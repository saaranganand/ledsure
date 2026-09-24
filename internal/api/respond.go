// implements ledsure's HTTP surface
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// encode with the given status code
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("api: failed to encode response body", "error", err)
	}
}
