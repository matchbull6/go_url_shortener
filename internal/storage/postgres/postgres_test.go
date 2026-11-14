package postgres_test

import (
	"os"
	"testing"

	"github.com/matchbull6/go_url_shortener/internal/storage"
	"github.com/matchbull6/go_url_shortener/internal/storage/postgres"
	"github.com/matchbull6/go_url_shortener/internal/storage/storagetest"
)

var _ storage.Storage = (*postgres.Store)(nil)

func TestStore(t *testing.T) {
	store, err := postgres.New(t.Context(), testDatabaseURL(t))
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	t.Cleanup(store.Close)

	storagetest.Run(t, func(t *testing.T) storage.Storage {
		t.Helper()
		if err := store.Truncate(t.Context()); err != nil {
			t.Fatalf("truncate links: %v", err)
		}
		return store
	})
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping postgres tests")
	}
	return url
}
