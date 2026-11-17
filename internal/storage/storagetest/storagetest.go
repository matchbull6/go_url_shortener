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
	t.Run("VisitCountsClicks", func(t *testing.T) { testVisitCountsClicks(t, newStorage(t)) })
	t.Run("GetDoesNotCountClicks", func(t *testing.T) { testGetDoesNotCountClicks(t, newStorage(t)) })
	t.Run("VisitNotFound", func(t *testing.T) { testVisitNotFound(t, newStorage(t)) })

	t.Run("ConcurrentVisits", func(t *testing.T) { testConcurrentVisits(t, newStorage(t)) })
	t.Run("ConcurrentSaveSameCode", func(t *testing.T) { testConcurrentSaveSameCode(t, newStorage) })
}

func testSaveThenGet(t *testing.T, s storage.Storage) {
	const (
		code = "abc123"
		url  = "https://go.dev"
	)

	ctx := t.Context()

	if err := s.Save(ctx, code, url); err != nil {
		t.Fatalf("Save(%q, %q) error = %v, want nil", code, url, err)
	}

	got, err := s.Get(ctx, code)
	if err != nil {
		t.Fatalf("Get(%q) error = %v, want nil", code, err)
	}
	if got.URL != url {
		t.Errorf("Get(%q) = %q, want %q", code, got.URL, url)
	}
	if got.Code != code {
		t.Errorf("Get(%q).Code = %q, want %q", code, got.Code, code)
	}
	if got.Clicks != 0 {
		t.Errorf("Get(%q).Clicks = %d, want 0 for a new link", code, got.Clicks)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("Get(%q).CreatedAt is zero, want it set on Save", code)
	}
}

func testGetNotFound(t *testing.T, s storage.Storage) {
	const code = "missing"

	ctx := t.Context()

	got, err := s.Get(ctx, code)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get(%q) error = %v, want %v", code, err, storage.ErrNotFound)
	}
	if got != (storage.Link{}) {
		t.Errorf("Get(%q) = %+v, want zero Link", code, got)
	}
}

func testSaveExistingCode(t *testing.T, s storage.Storage) {
	const code, first, second = "abc123", "https://go.dev", "https://example.com"

	ctx := t.Context()

	if err := s.Save(ctx, code, first); err != nil {
		t.Fatalf("Save(%q, %q) error = %v, want nil", code, first, err)
	}

	err := s.Save(ctx, code, second)
	if !errors.Is(err, storage.ErrExists) {
		t.Errorf("second Save(%q, %q) error = %v, want %v", code, second, err, storage.ErrExists)
	}

	got, err := s.Get(ctx, code)
	if err != nil {
		t.Fatalf("Get(%q) error = %v, want nil", code, err)
	}
	if got.URL != first {
		t.Errorf("Get(%q) = %q, want original %q (rejected Save must not overwrite)", code, got.URL, first)
	}
}

func testVisitCountsClicks(t *testing.T, s storage.Storage) {
	const (
		code   = "abc123"
		url    = "https://go.dev"
		visits = 3
	)

	ctx := t.Context()

	if err := s.Save(ctx, code, url); err != nil {
		t.Fatalf("Save(%q, %q) error = %v, want nil", code, url, err)
	}

	for range visits {
		got, err := s.Visit(ctx, code)
		if err != nil {
			t.Fatalf("Visit(%q) error = %v, want nil", code, err)
		}
		if got != url {
			t.Errorf("Visit(%q) = %q, want %q", code, got, url)
		}
	}

	link, err := s.Get(ctx, code)
	if err != nil {
		t.Fatalf("Get(%q) error = %v, want nil", code, err)
	}
	if link.Clicks != visits {
		t.Errorf("Get(%q).Clicks = %d, want %d", code, link.Clicks, visits)
	}
}

func testGetDoesNotCountClicks(t *testing.T, s storage.Storage) {
	const (
		code = "abc123"
		url  = "https://go.dev"
	)

	ctx := t.Context()

	if err := s.Save(ctx, code, url); err != nil {
		t.Fatalf("Save(%q, %q) error = %v, want nil", code, url, err)
	}

	for range 3 {
		link, err := s.Get(ctx, code)
		if err != nil {
			t.Fatalf("Get(%q) error = %v, want nil", code, err)
		}
		if link.Clicks != 0 {
			t.Errorf("Get(%q).Clicks = %d, want zero", code, link.Clicks)
		}
	}
}

func testVisitNotFound(t *testing.T, s storage.Storage) {
	const code = "missing"

	ctx := t.Context()

	got, err := s.Visit(ctx, code)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Visit(%q) error = %v, want %v", code, err, storage.ErrNotFound)
	}
	if got != "" {
		t.Errorf("Visit(%q) = %q, want empty string", code, got)
	}
}

// Catches lost updates (read-modify-write in Go instead of the store) that go test -race cannot see in a database.
func testConcurrentVisits(t *testing.T, s storage.Storage) {
	const (
		code       = "abc123"
		url        = "https://go.dev"
		goroutines = 50
		perG       = 20
	)

	ctx := t.Context()
	if err := s.Save(ctx, code, url); err != nil {
		t.Fatalf("Save(%q, %q) error = %v, want nil", code, url, err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			<-start
			for range perG {
				if _, err := s.Visit(ctx, code); err != nil {
					t.Errorf("Visit(%q) error = %v, want nil", code, err)
					return
				}
			}
		})
	}
	close(start)
	wg.Wait()

	link, err := s.Get(ctx, code)
	if err != nil {
		t.Fatalf("Get(%q) error = %v, want nil", code, err)
	}
	if link.Clicks != goroutines*perG {
		t.Errorf("Get(%q).Clicks = %d, want %d (lost updates)", code, link.Clicks, goroutines*perG)
	}
}

// Catches check-then-write races (TOCTOU) that go test -race cannot see.
func testConcurrentSaveSameCode(t *testing.T, newStorage func(t *testing.T) storage.Storage) {
	const (
		code       = "abc123"
		rounds     = 200
		goroutines = 50
	)

	ctx := t.Context()

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
				err := s.Save(ctx, code, url)
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
		got, err := s.Get(ctx, code)
		if err != nil {
			t.Fatalf("round %d: Get(%q) error = %v, want nil", round, code, err)
		}
		if got.URL != winners[0] {
			t.Fatalf("round %d: Get(%q) = %q, want %q (the only successful Save)", round, code, got.URL, winners[0])
		}
	}
}
