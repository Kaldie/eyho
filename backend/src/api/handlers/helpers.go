package handlers

import (
	"encoding/json"
	"net/http"
)

// errorResponse is the standard JSON error envelope used by all handlers.
type errorResponse struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

// writeJSON serialises v as JSON and sets the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError is a convenience wrapper for returning a structured error.
func writeError(w http.ResponseWriter, status int, errCode, field string) {
	writeJSON(w, status, errorResponse{Error: errCode, Field: field})
}
