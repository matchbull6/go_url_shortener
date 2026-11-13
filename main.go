// Command go_url_shortener is an HTTP service that shortens URLs.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/matchbull6/go_url_shortener/internal/httpapi"
	"github.com/matchbull6/go_url_shortener/internal/shortener"
	"github.com/matchbull6/go_url_shortener/internal/storage/memory"
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
	_ = storageType

	store := memory.New()
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
