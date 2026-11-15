// Command go_url_shortener is an HTTP service that shortens URLs.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	addr := getenv("ADDR", ":8080")
	baseURL := getenv("BASE_URL", "http://localhost:8080")
	storageType := getenv("STORAGE", "memory")
	codeLength, err := strconv.Atoi(getenv("CODE_LENGTH", strconv.Itoa(shortener.DefaultCodeLength)))
	if err != nil {
		return fmt.Errorf("invalid CODE_LENGTH: %w", err)
	}

	var store storage.Storage
	switch storageType {
	case "memory":
		store = memory.New()
	case "postgres":
		dbURL := getenv("DATABASE_URL", "")
		if dbURL == "" {
			return errors.New("DATABASE_URL is required when STORAGE=postgres")
		}
		startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		pg, err := postgres.New(startCtx, dbURL)
		cancel()
		if err != nil {
			return fmt.Errorf("connect to postgres: %w", err)
		}
		defer pg.Close()
		store = pg
	default:
		return fmt.Errorf("unknown STORAGE %q", storageType)
	}

	service, err := shortener.New(store, codeLength)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	server := &http.Server{
		Addr:    addr,
		Handler: httpapi.New(service, baseURL),
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr, "base_url", baseURL)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server failed: %w", err)
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server failed: %w", err)
	}
	slog.Info("stopped")
	return nil
}
