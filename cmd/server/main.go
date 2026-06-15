// Command server runs the URL shortener in shortener, redirector, or all mode,
// selected via the APP_MODE environment variable.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/toainguyen/url-shortener/internal/config"
	"github.com/toainguyen/url-shortener/internal/handler"
	"github.com/toainguyen/url-shortener/internal/service"
	"github.com/toainguyen/url-shortener/internal/store"
	"github.com/toainguyen/url-shortener/internal/util"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := log.New(os.Stdout, "[url-shortener] ", log.LstdFlags|log.LUTC)
	logger.Printf("starting in mode=%s port=%s ttl_days=%d", cfg.AppMode, cfg.Port, cfg.DefaultTTLDays)

	// Bounded context for all startup I/O.
	initCtx, cancelInit := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInit()

	mongoClient, err := store.NewMongo(initCtx, cfg.MongoURI)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), store.DisconnectTimeout)
		defer cancel()
		_ = mongoClient.Disconnect(ctx)
	}()

	coll := store.URLCollection(mongoClient, cfg.MongoDB)
	if err := store.EnsureIndexes(initCtx, coll); err != nil {
		return err
	}

	cache, err := store.NewRedis(initCtx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		return err
	}
	defer func() { _ = cache.Close() }()

	repo := store.NewMongoRepository(coll)

	var (
		shortenHandler  *handler.ShortenHandler
		redirectHandler *handler.RedirectHandler
	)

	if cfg.AppMode == config.ModeShortener || cfg.AppMode == config.ModeAll {
		bloom := util.NewBloomFilter(cfg.BloomCapacity, cfg.BloomFPRate)
		if err := bloom.Warm(initCtx, coll); err != nil {
			return err
		}
		logger.Printf("bloom filter warmed (capacity=%d fp=%g)", cfg.BloomCapacity, cfg.BloomFPRate)
		shortener := service.NewShortener(repo, cache, bloom, cfg.DefaultTTLDays)
		shortenHandler = handler.NewShortenHandler(shortener)
	}

	if cfg.AppMode == config.ModeRedirector || cfg.AppMode == config.ModeAll {
		redirector := service.NewRedirector(repo, cache)
		redirectHandler = handler.NewRedirectHandler(redirector)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.Router(cfg.AppMode, shortenHandler, redirectHandler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Printf("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Printf("stopped cleanly")
	return nil
}
