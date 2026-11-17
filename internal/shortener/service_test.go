package shortener_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/matchbull6/go_url_shortener/internal/shortener"
	"github.com/matchbull6/go_url_shortener/internal/storage"
)

// Every other test replaces newCode, so this is the only one that checks
// New wires in the real generator.
func TestNew_UsesRealGenerator(t *testing.T) {
	const url = "https://go.dev"
	const length = 5

	ctx := t.Context()
	fake := newFakeStore()
	service, err := shortener.New(fake, length)
	if err != nil {
		t.Fatalf("New(store, %d) error = %v, want nil", length, err)
	}

	code, err := service.Shorten(ctx, url)
	if err != nil {
		t.Fatalf("Shorten(%q) error = %v, want nil", url, err)
	}
	if len(code) != length {
		t.Errorf("Shorten(%q) = %q, len %d, want len %d", url, code, len(code), length)
	}
	if got := fake.links[code].URL; got != url {
		t.Errorf("store[%q] = %q after Shorten, want %q", code, got, url)
	}
}

func TestNew_CodeLengthBounds(t *testing.T) {
	tests := []struct {
		length  int
		wantErr bool
	}{
		{shortener.MinCodeLength - 1, true},
		{shortener.MinCodeLength, false},
		{shortener.MaxCodeLength, false},
		{shortener.MaxCodeLength + 1, true},
		{0, true},
		{-1, true},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.length), func(t *testing.T) {
			s, err := shortener.New(newFakeStore(), tt.length)
			if tt.wantErr {
				if err == nil {
					t.Errorf("New(store, %d) error = nil, want error", tt.length)
				}
				if s != nil {
					t.Errorf("New(store, %d) returned a Service along with an error", tt.length)
				}
				return
			}
			if err != nil {
				t.Errorf("New(store, %d) error = %v, want nil", tt.length, err)
			}
		})
	}
}

func TestShorten_Success(t *testing.T) {
	const fixedCode = "fixed01"
	const goodURL = "https://go.dev"

	ctx := t.Context()
	fake := newFakeStore()
	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return fixedCode, nil
	})

	code, err := service.Shorten(ctx, goodURL)
	if err != nil {
		t.Fatalf("Shorten(%q) error = %v, want nil", goodURL, err)
	}
	if code != fixedCode {
		t.Errorf("Shorten(%q) = %q, want %q", goodURL, code, fixedCode)
	}
	if calls != 1 {
		t.Errorf("newCode called %d times, want 1", calls)
	}
	if got := fake.links[fixedCode].URL; got != goodURL {
		t.Errorf("store[%q] = %q after Shorten, want %q", fixedCode, got, goodURL)
	}
	if len(fake.links) != 1 {
		t.Errorf("store has %d links, want 1: %v", len(fake.links), fake.links)
	}
}

func TestShorten_InvalidURL(t *testing.T) {
	const badURL = "javascript:alert(1)"

	ctx := t.Context()
	fake := newFakeStore()
	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return "code01", nil
	})

	code, err := service.Shorten(ctx, badURL)
	if !errors.Is(err, shortener.ErrInvalidURL) {
		t.Errorf("Shorten(%q) error = %v, want %v", badURL, err, shortener.ErrInvalidURL)
	}
	if code != "" {
		t.Errorf("Shorten(%q) = %q, want empty string", badURL, code)
	}
	if calls != 0 {
		t.Errorf("newCode called %d times, want 0 (URL must be validated before generating a code)", calls)
	}
	if len(fake.links) != 0 {
		t.Errorf("store has %d links after rejected Shorten, want 0: %v", len(fake.links), fake.links)
	}
}

func TestShorten_RetriesOnCollision(t *testing.T) {
	const (
		url       = "https://example.com"
		takenURL  = "https://taken.example"
		takenCode = "taken"
		freshCode = "fresh"
	)

	ctx := t.Context()
	fake := newFakeStore()
	fake.links[takenCode] = storage.Link{Code: takenCode, URL: takenURL, CreatedAt: time.Now()}

	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		if calls < shortener.MaxAttempts {
			return takenCode, nil
		}
		return freshCode, nil
	})

	code, err := service.Shorten(ctx, url)
	if errors.Is(err, shortener.ErrNoFreeCode) {
		t.Fatalf("Shorten(%q) gave up with %v after %d calls, want success on attempt %d",
			url, err, calls, shortener.MaxAttempts)
	}
	if err != nil {
		t.Fatalf("Shorten(%q) unexpected error: %v", url, err)
	}
	if code != freshCode {
		t.Errorf("Shorten(%q) = %q, want %q", url, code, freshCode)
	}
	if calls != shortener.MaxAttempts {
		t.Errorf("newCode called %d times, want %d", calls, shortener.MaxAttempts)
	}
	if got := fake.links[freshCode].URL; got != url {
		t.Errorf("store[%q] = %q, want %q", freshCode, got, url)
	}
	if got := fake.links[takenCode].URL; got != takenURL {
		t.Errorf("store[%q] = %q, want %q (existing link overwritten)", takenCode, got, takenURL)
	}
	if len(fake.links) != 2 {
		t.Errorf("store has %d links, want 2: %v", len(fake.links), fake.links)
	}
}

func TestShorten_NoFreeCode(t *testing.T) {
	const (
		url       = "https://example.com"
		takenURL  = "https://taken.example"
		takenCode = "taken"
	)

	ctx := t.Context()
	fake := newFakeStore()
	fake.links[takenCode] = storage.Link{Code: takenCode, URL: takenURL, CreatedAt: time.Now()}

	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return takenCode, nil
	})

	code, err := service.Shorten(ctx, url)
	if !errors.Is(err, shortener.ErrNoFreeCode) {
		t.Fatalf("Shorten(%q) error = %v, want %v", url, err, shortener.ErrNoFreeCode)
	}
	if code != "" {
		t.Errorf("Shorten(%q) = %q, want empty string", url, code)
	}
	if calls != shortener.MaxAttempts {
		t.Errorf("newCode called %d times, want %d", calls, shortener.MaxAttempts)
	}
	if got := fake.links[takenCode].URL; got != takenURL {
		t.Errorf("store[%q] = %q, want %q (existing link overwritten)", takenCode, got, takenURL)
	}
	if len(fake.links) != 1 {
		t.Errorf("store has %d links, want 1: %v", len(fake.links), fake.links)
	}
}

func TestShorten_StorageErrorNotRetried(t *testing.T) {
	const url = "https://example.com"
	errDB := errors.New("db down")

	ctx := t.Context()
	fake := newFakeStore()
	fake.saveErr = errDB

	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return "code01", nil
	})

	code, err := service.Shorten(ctx, url)
	if !errors.Is(err, errDB) {
		t.Errorf("Shorten(%q) error = %v, want %v", url, err, errDB)
	}
	if code != "" {
		t.Errorf("Shorten(%q) = %q, want empty string", url, code)
	}
	if calls != 1 {
		t.Errorf("newCode called %d times, want 1 (storage errors must not be retried)", calls)
	}
	if len(fake.links) != 0 {
		t.Errorf("store has %d links, want 0: %v", len(fake.links), fake.links)
	}
}

func TestShorten_GeneratorError(t *testing.T) {
	const url = "https://example.com"
	errRand := errors.New("rand failed")

	ctx := t.Context()
	fake := newFakeStore()
	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return "", errRand
	})

	code, err := service.Shorten(ctx, url)
	if !errors.Is(err, errRand) {
		t.Errorf("Shorten(%q) error = %v, want %v", url, err, errRand)
	}
	if code != "" {
		t.Errorf("Shorten(%q) = %q, want empty string", url, code)
	}
	if calls != 1 {
		t.Errorf("newCode called %d times, want 1 (generator errors must not be retried)", calls)
	}
	if len(fake.links) != 0 {
		t.Errorf("store has %d links, want 0: %v", len(fake.links), fake.links)
	}
}

func TestResolve_Found(t *testing.T) {
	const (
		code    = "code1"
		wantURL = "https://go.dev"
	)

	ctx := t.Context()
	fake := newFakeStore()
	fake.links[code] = storage.Link{Code: code, URL: wantURL, CreatedAt: time.Now()}
	got, err := newService(t, fake).Resolve(ctx, code)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v, want nil", code, err)
	}
	if got != wantURL {
		t.Errorf("Resolve(%q) = %q, want %q", code, got, wantURL)
	}
	if clicks := fake.links[code].Clicks; clicks != 1 {
		t.Errorf("Clicks after Resolve = %d, want 1", clicks)
	}
}

func TestResolve_NotFound(t *testing.T) {
	const code = "code1"

	ctx := t.Context()
	fake := newFakeStore()
	got, err := newService(t, fake).Resolve(ctx, code)
	if !errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Resolve(%q) error = %v, want %v", code, err, shortener.ErrNotFound)
	}
	if got != "" {
		t.Errorf("Resolve(%q) = %q, want empty string", code, got)
	}
}

func TestResolve_StorageError(t *testing.T) {
	const code = "code1"
	errDB := errors.New("db down")

	ctx := t.Context()
	fake := newFakeStore()
	fake.visitErr = errDB
	got, err := newService(t, fake).Resolve(ctx, code)

	if !errors.Is(err, errDB) {
		t.Errorf("Resolve(%q) error = %v, want %v", code, err, errDB)
	}
	if errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Resolve(%q) error = %v, must not be %v", code, err, shortener.ErrNotFound)
	}
	if got != "" {
		t.Errorf("Resolve(%q) = %q, want empty string", code, got)
	}
}

func TestStats_Found(t *testing.T) {
	const (
		code   = "code1"
		url    = "https://go.dev"
		clicks = 7
	)
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	initial := storage.Link{Code: code, URL: url, Clicks: clicks, CreatedAt: created}
	want := shortener.Link{Code: code, URL: url, Clicks: clicks, CreatedAt: created}

	ctx := t.Context()
	fake := newFakeStore()
	fake.links[code] = initial
	got, err := newService(t, fake).Stats(ctx, code)
	if err != nil {
		t.Fatalf("Stats(%q) error = %v, want nil", code, err)
	}
	if got != want {
		t.Errorf("Stats(%q) = %+v, want %+v", code, got, want)
	}
	if got := fake.links[code].Clicks; got != clicks {
		t.Errorf("Clicks after Stats = %d, want %d (Stats must not count a click)", got, clicks)
	}
}

func TestStats_NotFound(t *testing.T) {
	const code = "code1"

	ctx := t.Context()
	fake := newFakeStore()
	got, err := newService(t, fake).Stats(ctx, code)
	if !errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Stats(%q) error = %v, want %v", code, err, shortener.ErrNotFound)
	}
	if (got != shortener.Link{}) {
		t.Errorf("Stats(%q) = %+v, want shortener.Link{}", code, got)
	}
}

func TestStats_StorageError(t *testing.T) {
	const code = "code1"
	errDB := errors.New("db down")

	ctx := t.Context()
	fake := newFakeStore()
	fake.getErr = errDB
	got, err := newService(t, fake).Stats(ctx, code)

	if !errors.Is(err, errDB) {
		t.Errorf("Stats(%q) error = %v, want %v", code, err, errDB)
	}
	if errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Stats(%q) error = %v, must not be %v", code, err, shortener.ErrNotFound)
	}
	if (got != shortener.Link{}) {
		t.Errorf("Stats(%q) = %+v, want shortener.Link{}", code, got)
	}
}
