// Package handler exposes the HTTP surface for the shortener and redirector.
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/toainguyen/url-shortener/internal/model"
)

// ShortenService is the write-path dependency of the shorten handler.
type ShortenService interface {
	Create(ctx context.Context, longURL string) (*model.URL, error)
}

// RedirectService is the read-path dependency of the redirect handler.
type RedirectService interface {
	Resolve(ctx context.Context, shortURL string) (string, error)
}

// errorResponse is the JSON body returned for error statuses.
type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
