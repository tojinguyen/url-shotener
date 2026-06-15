package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/toainguyen/url-shortener/internal/model"
	"github.com/toainguyen/url-shortener/internal/service"
)

// ShortenHandler serves POST /api/v1/shorten.
type ShortenHandler struct {
	svc ShortenService
}

// NewShortenHandler constructs a ShortenHandler.
func NewShortenHandler(svc ShortenService) *ShortenHandler {
	return &ShortenHandler{svc: svc}
}

// Handle decodes the request, creates the mapping, and returns 201 with metadata.
func (h *ShortenHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req model.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if !validURL(req.LongURL) {
		writeError(w, http.StatusBadRequest, "long_url must be a valid absolute http(s) URL")
		return
	}

	doc, err := h.svc.Create(r.Context(), req.LongURL)
	if err != nil {
		if errors.Is(err, service.ErrAliasExhausted) {
			writeError(w, http.StatusInternalServerError, "could not generate a unique alias")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, model.ShortenResponse{
		ShortURL: doc.ShortURL,
		LongURL:  doc.LongURL,
		ExpireAt: doc.ExpireAt,
	})
}

// validURL performs basic sanity checking on a candidate destination.
func validURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
