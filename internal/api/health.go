package api

import "net/http"

// the body returned by GET /healthz
type healthResponse struct {
	Status string `json:"status"`
}

// reports liveness only
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}
