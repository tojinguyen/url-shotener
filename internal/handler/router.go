package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/toainguyen/url-shortener/internal/config"
)

// Router builds the chi router for the given app mode. Shorten and redirect may
// be nil when the corresponding mode is not active.
func Router(mode config.AppMode, shorten *ShortenHandler, redirect *RedirectHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if (mode == config.ModeShortener || mode == config.ModeAll) && shorten != nil {
		r.Post("/api/v1/shorten", shorten.Handle)
	}

	if (mode == config.ModeRedirector || mode == config.ModeAll) && redirect != nil {
		r.Get("/{short_url}", redirect.Handle)
	}

	return r
}
