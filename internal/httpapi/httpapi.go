// Package httpapi maps HTTP requests to the shortener service.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/matchbull6/go_url_shortener/internal/shortener"
)

type Service interface {
	Shorten(ctx context.Context, longURL string) (string, error)
	Resolve(ctx context.Context, code string) (string, error)
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

	url, err := h.service.Resolve(r.Context(), code)
	if errors.Is(err, shortener.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	if err != nil {
		slog.Error("resolve failed", "code", code, "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
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
		if maxErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
			msg := fmt.Sprintf("body too large, limit is %d bytes", maxErr.Limit)
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", msg)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}

	code, err := h.service.Shorten(r.Context(), req.URL)
	if errors.Is(err, shortener.ErrInvalidURL) {
		writeError(w, http.StatusBadRequest, "invalid_url", err.Error())
		return
	}
	if errors.Is(err, shortener.ErrNoFreeCode) {
		slog.Warn("no free short code", "url", req.URL, "err", err)
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, "no_free_code", "no free short code, try again")
		return
	}
	if err != nil {
		slog.Error("shorten failed", "url", req.URL, "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	resp := shortenResponse{Code: code, ShortURL: h.baseURL + "/" + code}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("write response failed", "err", err)
	}
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	resp := errorResponse{Error: errorBody{Code: code, Message: msg}}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("write error response failed", "err", err)
	}
}
