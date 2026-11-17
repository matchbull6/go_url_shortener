// Package shortener is the business logic: URL validation, short codes, lookup.
package shortener

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/matchbull6/go_url_shortener/internal/storage"
)

var (
	// Replaces storage.ErrNotFound, so httpapi does not import storage.
	ErrNotFound   = errors.New("shortener: code not found")
	ErrNoFreeCode = errors.New("shortener: no free code available")
)

const (
	minCodeLength = 4 // 62^4 ≈ 14.7M codes
	maxCodeLength = 32
)

const maxAttempts = 5

type Link struct {
	Code      string
	URL       string
	Clicks    int64
	CreatedAt time.Time
}

type Service struct {
	store   storage.Storage
	newCode func() (string, error)
}

func New(store storage.Storage, codeLength int) (*Service, error) {
	if codeLength < minCodeLength || codeLength > maxCodeLength {
		return nil, fmt.Errorf("shortener: code length %d out of range [%d, %d]",
			codeLength, minCodeLength, maxCodeLength)
	}
	return &Service{
		store:   store,
		newCode: func() (string, error) { return generateCode(codeLength) },
	}, nil
}

func (s *Service) Shorten(ctx context.Context, longURL string) (string, error) {
	if err := checkURL(longURL); err != nil {
		return "", err
	}

	for range maxAttempts {
		code, err := s.newCode()
		if err != nil {
			return "", err
		}

		err = s.store.Save(ctx, code, longURL)
		if errors.Is(err, storage.ErrExists) {
			continue
		}
		if err != nil {
			return "", err
		}
		return code, nil
	}
	return "", ErrNoFreeCode
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	url, err := s.store.Visit(ctx, code)
	if errors.Is(err, storage.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return url, nil
}

func (s *Service) Stats(ctx context.Context, code string) (Link, error) {
	link, err := s.store.Get(ctx, code)
	if errors.Is(err, storage.ErrNotFound) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, err
	}
	return Link{
		Code:      link.Code,
		URL:       link.URL,
		Clicks:    link.Clicks,
		CreatedAt: link.CreatedAt,
	}, nil
}
