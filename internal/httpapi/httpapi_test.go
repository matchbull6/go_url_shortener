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
	"time"

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

	statsLink    shortener.Link
	statsErr     error
	gotStatsCode string
}

func (f *fakeService) Shorten(_ context.Context, longURL string) (string, error) {
	f.gotURL = longURL
	return f.shortenCode, f.shortenErr
}

func (f *fakeService) Resolve(_ context.Context, code string) (string, error) {
	f.gotCode = code
	return f.resolveURL, f.resolveErr
}

func (f *fakeService) Stats(_ context.Context, code string) (shortener.Link, error) {
	f.gotStatsCode = code
	return f.statsLink, f.statsErr
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error body: %v; body = %q", err, rec.Body.String())
	}
	return resp
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
	if got := decodeError(t, rec).Error.Code; got != "not_found" {
		t.Errorf("error code = %q, want %q", got, "not_found")
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
	if got := decodeError(t, rec).Error.Code; got != "internal" {
		t.Errorf("error code = %q, want %q", got, "internal")
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
	if got := decodeError(t, rec).Error.Code; got != "body_too_large" {
		t.Errorf("error code = %q, want %q", got, "body_too_large")
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
	if got := decodeError(t, rec).Error.Code; got != "invalid_json" {
		t.Errorf("error code = %q, want %q", got, "invalid_json")
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
	if got := decodeError(t, rec).Error.Code; got != "invalid_url" {
		t.Errorf("error code = %q, want %q", got, "invalid_url")
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
	if got := decodeError(t, rec).Error.Code; got != "no_free_code" {
		t.Errorf("error code = %q, want %q", got, "no_free_code")
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
	if got := decodeError(t, rec).Error.Code; got != "internal" {
		t.Errorf("error code = %q, want %q", got, "internal")
	}
	if body := rec.Body.String(); strings.Contains(body, secret) {
		t.Errorf("response body %q leaks internal error", body)
	}
}

func TestStats_Found(t *testing.T) {
	const code, url = "abc123", "https://go.dev"
	// Non-UTC on purpose: the handler must normalize to UTC.
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("UTC+3", 3*60*60))
	fake := &fakeService{statsLink: shortener.Link{Code: code, URL: url, Clicks: 7, CreatedAt: created}}

	req := httptest.NewRequest(http.MethodGet, "/api/links/"+code+"/stats", nil)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Result().Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var got struct {
		Code      string `json:"code"`
		URL       string `json:"url"`
		ShortURL  string `json:"short_url"`
		Clicks    int64  `json:"clicks"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v; body = %q", err, rec.Body.String())
	}
	if got.Code != code {
		t.Errorf("code = %q, want %q", got.Code, code)
	}
	if got.URL != url {
		t.Errorf("url = %q, want %q", got.URL, url)
	}
	if want := "http://short.test/" + code; got.ShortURL != want {
		t.Errorf("short_url = %q, want %q", got.ShortURL, want)
	}
	if got.Clicks != 7 {
		t.Errorf("clicks = %d, want 7", got.Clicks)
	}
	if want := "2026-01-02T00:04:05Z"; got.CreatedAt != want {
		t.Errorf("created_at = %q, want %q", got.CreatedAt, want)
	}

	if fake.gotStatsCode != code {
		t.Errorf("Stats called with %q, want %q", fake.gotStatsCode, code)
	}
	if fake.gotCode != "" {
		t.Errorf("Resolve called with %q, want no call (it would count a click)", fake.gotCode)
	}
}

func TestStats_NotFound(t *testing.T) {
	fake := &fakeService{statsErr: shortener.ErrNotFound}

	req := httptest.NewRequest(http.MethodGet, "/api/links/missing/stats", nil)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := decodeError(t, rec).Error.Code; got != "not_found" {
		t.Errorf("error code = %q, want %q", got, "not_found")
	}
}

func TestStats_ServiceError(t *testing.T) {
	const secret = "password=hunter2"
	fake := &fakeService{statsErr: errors.New("db down: " + secret)}

	req := httptest.NewRequest(http.MethodGet, "/api/links/abc123/stats", nil)
	rec := httptest.NewRecorder()
	httpapi.New(fake, "http://short.test").ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := decodeError(t, rec).Error.Code; got != "internal" {
		t.Errorf("error code = %q, want %q", got, "internal")
	}
	if body := rec.Body.String(); strings.Contains(body, secret) {
		t.Errorf("response body %q leaks internal error", body)
	}
}
