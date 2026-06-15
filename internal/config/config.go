// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AppMode selects which HTTP surface the binary exposes.
type AppMode string

const (
	// ModeShortener exposes only the write path (POST /api/v1/shorten).
	ModeShortener AppMode = "shortener"
	// ModeRedirector exposes only the read path (GET /{short_url}).
	ModeRedirector AppMode = "redirector"
	// ModeAll exposes both surfaces; convenient for local development.
	ModeAll AppMode = "all"
)

// Config holds all tunable settings for a running instance.
type Config struct {
	AppMode       AppMode
	Port          string
	MongoURI      string
	MongoDB       string
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// DefaultTTLDays is applied to every newly created short URL.
	DefaultTTLDays int

	// Bloom filter sizing (shortener/all mode only).
	BloomCapacity uint
	BloomFPRate   float64
}

// Load reads configuration from the environment, applying sane defaults.
// It returns an error only when a supplied value is malformed.
func Load() (*Config, error) {
	cfg := &Config{
		AppMode:       AppMode(getEnv("APP_MODE", string(ModeAll))),
		Port:          getEnv("PORT", "8080"),
		MongoURI:      getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:       getEnv("MONGO_DB", "urlshortener"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
	}

	switch cfg.AppMode {
	case ModeShortener, ModeRedirector, ModeAll:
	default:
		return nil, fmt.Errorf("invalid APP_MODE %q (want shortener|redirector|all)", cfg.AppMode)
	}

	var err error
	if cfg.RedisDB, err = getEnvInt("REDIS_DB", 0); err != nil {
		return nil, err
	}
	if cfg.DefaultTTLDays, err = getEnvInt("DEFAULT_TTL_DAYS", 30); err != nil {
		return nil, err
	}
	if cfg.DefaultTTLDays <= 0 {
		return nil, fmt.Errorf("DEFAULT_TTL_DAYS must be positive, got %d", cfg.DefaultTTLDays)
	}

	capacity, err := getEnvInt("BLOOM_CAPACITY", 10_000_000)
	if err != nil {
		return nil, err
	}
	cfg.BloomCapacity = uint(capacity)

	if cfg.BloomFPRate, err = getEnvFloat("BLOOM_FP_RATE", 0.001); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("env %s: %w", key, err)
	}
	return n, nil
}

func getEnvFloat(key string, fallback float64) (float64, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0, fmt.Errorf("env %s: %w", key, err)
	}
	return f, nil
}
