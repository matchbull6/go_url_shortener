// Package httpapi maps HTTP requests to the shortener service.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/matchbull6/go_url_shortener/internal/shortener"
)

type Service interface {
	Shorten(longURL string) (string, error)
	Resolve(code string) (string, error)
}

type handler struct {
	service Service
	baseURL string
}

func New(service Service, baseURL string) http.Handler {
	h := &handler{service: service, baseURL: baseURL}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", h.redirect)
	mux.HandleFunc("POST /api/shorten", h.shorten)
	return mux
}

func (h *handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	url, err := h.service.Resolve(code)
	if errors.Is(err, shortener.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("resolve failed", "code", code, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}

const maxBodyBytes = 4 << 10 // 4 KiB

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func (h *handler) shorten(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	code, err := h.service.Shorten(req.URL)
	if errors.Is(err, shortener.ErrInvalidURL) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errors.Is(err, shortener.ErrNoFreeCode) {
		slog.Warn("no free short code", "url", req.URL, "err", err)
		w.Header().Set("Retry-After", "1")
		http.Error(w, "no free short code, try again", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		slog.Error("shorten failed", "url", req.URL, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := shortenResponse{Code: code, ShortURL: h.baseURL + "/" + code}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("write response failed", "err", err)
	}
}
