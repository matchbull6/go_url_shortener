// Package storage defines the Storage contract. Backends live in subpackages
// and must pass the tests in storagetest.
package storage

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("storage: code not found")
	ErrExists   = errors.New("storage: code already exists")
)

type Storage interface {
	Get(ctx context.Context, code string) (string, error)

	// If code is already taken it returns ErrExists and
	// leaves the existing URL untouched.
	Save(ctx context.Context, code, url string) error
}
