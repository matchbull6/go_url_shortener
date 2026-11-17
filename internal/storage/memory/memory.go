// Package memory is an in-memory storage.Storage; data is lost on restart.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/matchbull6/go_url_shortener/internal/storage"
)

type Store struct {
	mu    sync.RWMutex
	links map[string]storage.Link
}

func New() *Store {
	return &Store{
		links: make(map[string]storage.Link),
	}
}

func (s *Store) Get(_ context.Context, code string) (storage.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	link, ok := s.links[code]
	if !ok {
		return storage.Link{}, storage.ErrNotFound
	}
	return link, nil
}

func (s *Store) Visit(_ context.Context, code string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.links[code]
	if !ok {
		return "", storage.ErrNotFound
	}

	link.Clicks++
	s.links[code] = link
	return link.URL, nil
}

func (s *Store) Save(_ context.Context, code, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.links[code]; ok {
		return storage.ErrExists
	}

	s.links[code] = storage.Link{Code: code, URL: url, CreatedAt: time.Now()}
	return nil
}
