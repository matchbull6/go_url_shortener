package shortener_test

import (
	"testing"

	"github.com/matchbull6/go_url_shortener/internal/shortener"
	"github.com/matchbull6/go_url_shortener/internal/storage"
)

var _ storage.Storage = (*fakeStore)(nil)

type fakeStore struct {
	links   map[string]string
	saveErr error
	getErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		links: make(map[string]string),
	}
}

func (s *fakeStore) Get(code string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}

	url, ok := s.links[code]
	if !ok {
		return "", storage.ErrNotFound
	}
	return url, nil
}

func (s *fakeStore) Save(code, url string) error {
	if s.saveErr != nil {
		return s.saveErr
	}

	if _, ok := s.links[code]; ok {
		return storage.ErrExists
	}

	s.links[code] = url
	return nil
}

func newService(t *testing.T, store storage.Storage) *shortener.Service {
	t.Helper()
	s, err := shortener.New(store, shortener.DefaultCodeLength)
	if err != nil {
		t.Fatalf("New(store, %d) error = %v, want nil", shortener.DefaultCodeLength, err)
	}
	return s
}
