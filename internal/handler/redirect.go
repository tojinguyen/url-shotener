package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/toainguyen/url-shortener/internal/model"
	"github.com/toainguyen/url-shortener/internal/service"
)

// RedirectHandler serves GET /{short_url}.
type RedirectHandler struct {
	svc RedirectService
}

// NewRedirectHandler constructs a RedirectHandler.
func NewRedirectHandler(svc RedirectService) *RedirectHandler {
	return &RedirectHandler{svc: svc}
}

// Handle resolves the short key and issues a 302 redirect to the long URL.
func (h *RedirectHandler) Handle(w http.ResponseWriter, r *http.Request) {
	shortURL := chi.URLParam(r, "short_url")
	if len(shortURL) != model.ShortKeyLength {
		http.NotFound(w, r)
		return
	}

	longURL, err := h.svc.Resolve(r.Context(), shortURL)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.Redirect(w, r, longURL, http.StatusFound)
}
