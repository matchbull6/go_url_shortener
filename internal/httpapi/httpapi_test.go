package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matchbull6/go_url_shortener/internal/httpapi"
	"github.com/matchbull6/go_url_shortener/internal/shortener"
)

var _ httpapi.Service = (*shortener.Service)(nil)

type fakeService struct {
	shortenCode string
	shortenErr  error
	gotURL      string

	resolveURL string
	resolveErr error
	gotCode    string
}

func (f *fakeService) Shorten(_ context.Context, longURL string) (string, error) {
	f.gotURL = longURL
	return f.shortenCode, f.shortenErr
}

func (f *fakeService) Resolve(_ context.Context, code string) (string, error) {
	f.gotCode = code
	return f.resolveURL, f.resolveErr
}

func TestRedirect_Found(t *testing.T) {
	const code, url = "abc123", "https://go.dev"
	fake := &fakeService{resolveURL: url}

	req := httptest.NewRequest(http.MethodGet, "/"+code, nil)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("GET /%s status = %d, want %d", code, rec.Code, http.StatusFound)
	}
	if got := rec.Result().Header.Get("Location"); got != url {
		t.Errorf("GET /%s Location = %q, want %q", code, got, url)
	}
	if fake.gotCode != code {
		t.Errorf("Resolve called with %q, want %q", fake.gotCode, code)
	}
}

func TestRedirect_NotFound(t *testing.T) {
	fake := &fakeService{resolveErr: shortener.ErrNotFound}

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /missing status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Result().Header.Get("Location"); got != "" {
		t.Errorf("GET /missing Location = %q, want none", got)
	}
}

func TestRedirect_ServiceError(t *testing.T) {
	const secret = "password=hunter2"
	fake := &fakeService{resolveErr: errors.New("db down: " + secret)}

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("GET /abc123 status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if body := rec.Body.String(); strings.Contains(body, secret) {
		t.Errorf("response body %q leaks internal error", body)
	}
}

func TestShorten_Success(t *testing.T) {
	const url, code = "https://go.dev", "abc123"
	fake := &fakeService{shortenCode: code}

	body := strings.NewReader(`{"url": "https://go.dev"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/shorten status = %d, want %d (body: %q)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if got := rec.Result().Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if fake.gotURL != url {
		t.Errorf("Shorten called with %q, want %q", fake.gotURL, url)
	}

	var resp struct {
		Code     string `json:"code"`
		ShortURL string `json:"short_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	if resp.Code != code {
		t.Errorf("response code = %q, want %q", resp.Code, code)
	}
	if want := "http://short.test/" + code; resp.ShortURL != want {
		t.Errorf("response short_url = %q, want %q", resp.ShortURL, want)
	}
}

func TestShorten_BodyTooLarge(t *testing.T) {
	fake := &fakeService{}

	body := strings.NewReader(`{"url":"https://example.com/` + strings.Repeat("a", 5000) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if fake.gotURL != "" {
		t.Errorf("Shorten called with %q, want no call", fake.gotURL)
	}
}

func TestShorten_BadJSON(t *testing.T) {
	fake := &fakeService{}

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if fake.gotURL != "" {
		t.Errorf("Shorten called with %q, want no call", fake.gotURL)
	}
}

func TestShorten_InvalidURL(t *testing.T) {
	fake := &fakeService{shortenErr: fmt.Errorf("%w: missing host", shortener.ErrInvalidURL)}

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url": "http://"}`))
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if body := rec.Body.String(); !strings.Contains(body, "missing host") {
		t.Errorf("body = %q, want it to explain the reason (missing host)", body)
	}
}

func TestShorten_NoFreeCode(t *testing.T) {
	const want = "try again"

	fake := &fakeService{shortenErr: shortener.ErrNoFreeCode}

	body := strings.NewReader(`{"url": "https://go.dev"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /api/shorten status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); !strings.Contains(got, want) {
		t.Errorf("POST /api/shorten body = %q, want it to contain %q", got, want)
	}
	if got := rec.Result().Header.Get("Retry-After"); got != "1" {
		t.Errorf("POST /api/shorten Retry-After = %q, want %q", got, "1")
	}
}

func TestShorten_ServiceError(t *testing.T) {
	const secret = "password=hunter2"
	fake := &fakeService{shortenErr: errors.New("db down: " + secret)}

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url": "https://go.dev"}`))
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if body := rec.Body.String(); strings.Contains(body, secret) {
		t.Errorf("response body %q leaks internal error", body)
	}
}
