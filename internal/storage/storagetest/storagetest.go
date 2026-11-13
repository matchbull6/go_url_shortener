// Package storagetest holds contract tests that every storage.Storage backend
// must pass, the same pattern as testing/fstest.
package storagetest

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/matchbull6/go_url_shortener/internal/storage"
)

func Run(t *testing.T, newStorage func(t *testing.T) storage.Storage) {
	t.Run("SaveThenGet", func(t *testing.T) { testSaveThenGet(t, newStorage(t)) })
	t.Run("GetNotFound", func(t *testing.T) { testGetNotFound(t, newStorage(t)) })
	t.Run("SaveExistingCodeKeepsOriginalURL", func(t *testing.T) { testSaveExistingCode(t, newStorage(t)) })
	t.Run("ConcurrentSaveSameCode", func(t *testing.T) { testConcurrentSaveSameCode(t, newStorage) })
}

func testSaveThenGet(t *testing.T, s storage.Storage) {
	const code, url = "abc123", "https://go.dev"

	if err := s.Save(code, url); err != nil {
		t.Fatalf("Save(%q, %q) error = %v, want nil", code, url, err)
	}

	got, err := s.Get(code)
	if err != nil {
		t.Fatalf("Get(%q) error = %v, want nil", code, err)
	}
	if got != url {
		t.Errorf("Get(%q) = %q, want %q", code, got, url)
	}
}

func testGetNotFound(t *testing.T, s storage.Storage) {
	const code = "missing"

	got, err := s.Get(code)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get(%q) error = %v, want %v", code, err, storage.ErrNotFound)
	}
	if got != "" {
		t.Errorf("Get(%q) = %q, want empty string", code, got)
	}
}

func testSaveExistingCode(t *testing.T, s storage.Storage) {
	const code, first, second = "abc123", "https://go.dev", "https://example.com"

	if err := s.Save(code, first); err != nil {
		t.Fatalf("Save(%q, %q) error = %v, want nil", code, first, err)
	}

	err := s.Save(code, second)
	if !errors.Is(err, storage.ErrExists) {
		t.Errorf("second Save(%q, %q) error = %v, want %v", code, second, err, storage.ErrExists)
	}

	got, err := s.Get(code)
	if err != nil {
		t.Fatalf("Get(%q) error = %v, want nil", code, err)
	}
	if got != first {
		t.Errorf("Get(%q) = %q, want original %q (rejected Save must not overwrite)", code, got, first)
	}
}

// Catches check-then-write races (TOCTOU) that go test -race cannot see.
func testConcurrentSaveSameCode(t *testing.T, newStorage func(t *testing.T) storage.Storage) {
	const (
		code       = "abc123"
		rounds     = 200
		goroutines = 50
	)

	for round := range rounds {
		s := newStorage(t)
		start := make(chan struct{})

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			winners []string
		)
		for i := range goroutines {
			url := fmt.Sprintf("https://example.com/%d", i)
			wg.Go(func() {
				<-start
				err := s.Save(code, url)
				switch {
				case err == nil:
					mu.Lock()
					winners = append(winners, url)
					mu.Unlock()
				case !errors.Is(err, storage.ErrExists):
					t.Errorf("Save(%q, %q) error = %v, want nil or %v", code, url, err, storage.ErrExists)
				}
			})
		}
		close(start)
		wg.Wait()

		if len(winners) != 1 {
			t.Fatalf("round %d: Save succeeded %d times, want exactly 1 (winners: %q)", round, len(winners), winners)
		}
		got, err := s.Get(code)
		if err != nil {
			t.Fatalf("round %d: Get(%q) error = %v, want nil", round, code, err)
		}
		if got != winners[0] {
			t.Fatalf("round %d: Get(%q) = %q, want %q (the only successful Save)", round, code, got, winners[0])
		}
	}
}
