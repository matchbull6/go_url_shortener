package shortener_test

import (
	"context"
	"testing"
	"time"

	"github.com/matchbull6/go_url_shortener/internal/shortener"
	"github.com/matchbull6/go_url_shortener/internal/storage"
)

var _ storage.Storage = (*fakeStore)(nil)

type fakeStore struct {
	links    map[string]storage.Link
	getErr   error
	visitErr error
	saveErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		links: make(map[string]storage.Link),
	}
}

func (s *fakeStore) Get(_ context.Context, code string) (storage.Link, error) {
	if s.getErr != nil {
		return storage.Link{}, s.getErr
	}

	link, ok := s.links[code]
	if !ok {
		return storage.Link{}, storage.ErrNotFound
	}
	return link, nil
}

func (s *fakeStore) Visit(_ context.Context, code string) (string, error) {
	if s.visitErr != nil {
		return "", s.visitErr
	}

	link, ok := s.links[code]
	if !ok {
		return "", storage.ErrNotFound
	}
	link.Clicks++
	s.links[code] = link
	return link.URL, nil
}

func (s *fakeStore) Save(_ context.Context, code, url string) error {
	if s.saveErr != nil {
		return s.saveErr
	}

	if _, ok := s.links[code]; ok {
		return storage.ErrExists
	}

	s.links[code] = storage.Link{Code: code, URL: url, CreatedAt: time.Now()}
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
