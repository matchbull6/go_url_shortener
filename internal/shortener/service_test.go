package shortener_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/matchbull6/go_url_shortener/internal/shortener"
)

// Every other test replaces newCode, so this is the only one that checks
// New wires in the real generator.
func TestNew_UsesRealGenerator(t *testing.T) {
	const url = "https://go.dev"
	const length = 5

	fake := newFakeStore()
	service, err := shortener.New(fake, length)
	if err != nil {
		t.Fatalf("New(store, %d) error = %v, want nil", shortener.DefaultCodeLength, err)
	}

	code, err := service.Shorten(url)
	if err != nil {
		t.Fatalf("Shorten(%q) error = %v, want nil", url, err)
	}
	if len(code) != length {
		t.Errorf("Shorten(%q) = %q, len %d, want len %d", url, code, len(code), length)
	}
	if fake.links[code] != url {
		t.Errorf("store[%q] = %q after Shorten, want %q", code, fake.links[code], url)
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

	fake := newFakeStore()
	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return fixedCode, nil
	})

	code, err := service.Shorten(goodURL)
	if err != nil {
		t.Fatalf("Shorten(%q) error = %v, want nil", goodURL, err)
	}
	if code != fixedCode {
		t.Errorf("Shorten(%q) = %q, want %q", goodURL, code, fixedCode)
	}
	if calls != 1 {
		t.Errorf("newCode called %d times, want 1", calls)
	}
	if fake.links[fixedCode] != goodURL {
		t.Errorf("store[%q] = %q after Shorten, want %q", fixedCode, fake.links[fixedCode], goodURL)
	}
	if len(fake.links) != 1 {
		t.Errorf("store has %d links, want 1: %v", len(fake.links), fake.links)
	}
}

func TestShorten_InvalidURL(t *testing.T) {
	const badURL = "javascript:alert(1)"

	fake := newFakeStore()
	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return "code01", nil
	})

	code, err := service.Shorten(badURL)
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

	fake := newFakeStore()
	fake.links[takenCode] = takenURL

	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		if calls < shortener.MaxAttempts {
			return takenCode, nil
		}
		return freshCode, nil
	})

	code, err := service.Shorten(url)
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
	if got := fake.links[freshCode]; got != url {
		t.Errorf("store[%q] = %q, want %q", freshCode, got, url)
	}
	if got := fake.links[takenCode]; got != takenURL {
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

	fake := newFakeStore()
	fake.links[takenCode] = takenURL

	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return takenCode, nil
	})

	code, err := service.Shorten(url)
	if !errors.Is(err, shortener.ErrNoFreeCode) {
		t.Fatalf("Shorten(%q) error = %v, want %v", url, err, shortener.ErrNoFreeCode)
	}
	if code != "" {
		t.Errorf("Shorten(%q) = %q, want empty string", url, code)
	}
	if calls != shortener.MaxAttempts {
		t.Errorf("newCode called %d times, want %d", calls, shortener.MaxAttempts)
	}
	if got := fake.links[takenCode]; got != takenURL {
		t.Errorf("store[%q] = %q, want %q (existing link overwritten)", takenCode, got, takenURL)
	}
	if len(fake.links) != 1 {
		t.Errorf("store has %d links, want 1: %v", len(fake.links), fake.links)
	}
}

func TestShorten_StorageErrorNotRetried(t *testing.T) {
	const url = "https://example.com"
	errDB := errors.New("db down")

	fake := newFakeStore()
	fake.saveErr = errDB

	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return "code01", nil
	})

	code, err := service.Shorten(url)
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

	fake := newFakeStore()

	service := newService(t, fake)
	calls := 0
	shortener.SetCodeGenerator(service, func() (string, error) {
		calls++
		return "", errRand
	})

	code, err := service.Shorten(url)
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
		code = "code1"
		want = "https://go.dev"
	)

	fake := newFakeStore()
	fake.links[code] = want
	got, err := newService(t, fake).Resolve(code)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v, want nil", code, err)
	}
	if got != want {
		t.Errorf("Resolve(%q) = %q, want %q", code, got, want)
	}
}

func TestResolve_NotFound(t *testing.T) {
	const code = "code1"

	fake := newFakeStore()
	got, err := newService(t, fake).Resolve(code)
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

	fake := newFakeStore()
	fake.getErr = errDB
	got, err := newService(t, fake).Resolve(code)

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
