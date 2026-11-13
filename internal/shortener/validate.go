package shortener

import (
	"errors"
	"fmt"
	"net/url"
)

var ErrInvalidURL = errors.New("shortener: invalid URL")

const maxURLLength = 2000

func checkURL(s string) error {
	if s == "" {
		return fmt.Errorf("%w: empty", ErrInvalidURL)
	}
	if len(s) > maxURLLength {
		return fmt.Errorf("%w: too long (%d > %d bytes)", ErrInvalidURL, len(s), maxURLLength)
	}

	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: unsupported scheme %q, want http or https", ErrInvalidURL, u.Scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	return nil
}
