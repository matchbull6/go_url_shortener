package memory_test

import (
	"testing"

	"github.com/matchbull6/go_url_shortener/internal/storage"
	"github.com/matchbull6/go_url_shortener/internal/storage/memory"
	"github.com/matchbull6/go_url_shortener/internal/storage/storagetest"
)

var _ storage.Storage = (*memory.Store)(nil)

func TestStore(t *testing.T) {
	storagetest.Run(t, func(t *testing.T) storage.Storage {
		return memory.New()
	})
}
