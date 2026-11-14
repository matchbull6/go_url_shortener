// Command go_url_shortener is an HTTP service that shortens URLs.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/matchbull6/go_url_shortener/internal/httpapi"
	"github.com/matchbull6/go_url_shortener/internal/shortener"
	"github.com/matchbull6/go_url_shortener/internal/storage"
	"github.com/matchbull6/go_url_shortener/internal/storage/memory"
	"github.com/matchbull6/go_url_shortener/internal/storage/postgres"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := getenv("ADDR", ":8080")
	baseURL := getenv("BASE_URL", "http://localhost:8080")
	storageType := getenv("STORAGE", "memory")
	codeLength, err := strconv.Atoi(getenv("CODE_LENGTH", strconv.Itoa(shortener.DefaultCodeLength)))
	if err != nil {
		slog.Error("invalid CODE_LENGTH", "err", err)
		os.Exit(1)
	}

	var store storage.Storage
	switch storageType {
	case "memory":
		store = memory.New()
	case "postgres":
		dbURL := getenv("DATABASE_URL", "")
		if dbURL == "" {
			slog.Error("DATABASE_URL is required when STORAGE=postgres")
			os.Exit(1)
		}
		pg, err := postgres.New(context.Background(), dbURL)
		if err != nil {
			slog.Error("connect to postgres", "err", err)
			os.Exit(1)
		}
		defer pg.Close()
		store = pg
	default:
		slog.Error("unknown STORAGE", "value", storageType)
		os.Exit(1)
	}

	service, err := shortener.New(store, codeLength)
	if err != nil {
		slog.Error("invalid config", "err", err)
		os.Exit(1)
	}
	server := &http.Server{
		Addr:    addr,
		Handler: httpapi.New(service, baseURL),
	}

	slog.Info("listening", "addr", addr, "base_url", baseURL)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
