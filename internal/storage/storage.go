// Package storage defines the Storage contract. Backends live in subpackages
// and must pass the tests in storagetest.
package storage

import "errors"

var (
	ErrNotFound = errors.New("storage: code not found")
	ErrExists   = errors.New("storage: code already exists")
)

type Storage interface {
	Get(code string) (string, error)

	// If code is already taken it returns ErrExists and
	// leaves the existing URL untouched.
	Save(code, url string) error
}
