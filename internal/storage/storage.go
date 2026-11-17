// Package storage defines the Storage contract. Backends live in subpackages
// and must pass the tests in storagetest.
package storage

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("storage: code not found")
	ErrExists   = errors.New("storage: code already exists")
)

type Link struct {
	Code      string
	URL       string
	Clicks    int64
	CreatedAt time.Time
}

type Storage interface {
	// Get returns the link without counting a click.
	Get(ctx context.Context, code string) (Link, error)

	// Visit counts a click and returns the URL.
	Visit(ctx context.Context, code string) (string, error)

	// If code is already taken it returns ErrExists and
	// leaves the existing URL untouched.
	Save(ctx context.Context, code, url string) error
}
