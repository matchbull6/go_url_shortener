// Package memory is an in-memory storage.Storage; data is lost on restart.
package memory

import (
	"sync"

	"github.com/matchbull6/go_url_shortener/internal/storage"
)

type Store struct {
	mu    sync.RWMutex
	links map[string]string
}

func New() *Store {
	return &Store{
		links: make(map[string]string),
	}
}

func (s *Store) Get(code string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	url, ok := s.links[code]
	if !ok {
		return "", storage.ErrNotFound
	}
	return url, nil
}

func (s *Store) Save(code, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.links[code]; ok {
		return storage.ErrExists
	}

	s.links[code] = url
	return nil
}
