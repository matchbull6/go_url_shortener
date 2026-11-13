package shortener_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/matchbull6/go_url_shortener/internal/shortener"
)

func TestCheckURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		reason  string
	}{
		// --- valid ---
		{name: "https", in: "https://go.dev"},
		{name: "http with path query fragment", in: "http://example.com/path?q=1#frag"},
		{name: "localhost with port", in: "https://localhost:8080"},
		{name: "ipv4 with port", in: "http://127.0.0.1:8080/api"},
		{name: "ipv6 with port", in: "https://[::1]:8080/"},
		{name: "uppercase scheme", in: "HTTPS://GO.DEV"},
		{name: "unicode host", in: "https://пример.рф/путь"},
		{name: "exactly max length", in: "https://" + strings.Repeat("a", shortener.MaxURLLength-len("https://"))},

		// --- empty and length ---
		{name: "empty", in: "", wantErr: true, reason: "empty"},
		{name: "too long", in: "https://" + strings.Repeat("a", shortener.MaxURLLength), wantErr: true, reason: "too long"},

		// --- url.Parse returns an error ---
		{name: "leading space", in: " https://go.dev", wantErr: true},
		{name: "trailing newline", in: "https://go.dev\n", wantErr: true},
		{name: "space in host", in: "http://exa mple.com", wantErr: true},
		{name: "non-numeric port", in: "http://example.com:port", wantErr: true},
		{name: "unclosed ipv6 bracket", in: "http://[::1", wantErr: true},
		{name: "bad percent escape", in: "http://%zz.com", wantErr: true},
		{name: "null byte", in: "https://go.dev\x00", wantErr: true},
		{name: "tab", in: "https://go.dev\t/path", wantErr: true},

		// --- missing scheme ---
		{name: "only spaces", in: "   ", wantErr: true, reason: "scheme"},
		{name: "bare word", in: "abc", wantErr: true, reason: "scheme"},
		{name: "domain without scheme", in: "go.dev", wantErr: true, reason: "scheme"},
		{name: "www without scheme", in: "www.go.dev/path", wantErr: true, reason: "scheme"},
		{name: "relative path", in: "/relative/path", wantErr: true, reason: "scheme"},
		{name: "protocol-relative", in: "//go.dev", wantErr: true, reason: "scheme"},
		{name: "only query", in: "?q=1", wantErr: true, reason: "scheme"},
		{name: "only fragment", in: "#frag", wantErr: true, reason: "scheme"},

		// --- disallowed schemes ---
		{name: "javascript", in: "javascript:alert(1)", wantErr: true, reason: "scheme"},
		{name: "javascript mixed case", in: "JaVaScRiPt:alert(1)", wantErr: true, reason: "scheme"},
		{name: "data", in: "data:text/html,<script>alert(1)</script>", wantErr: true, reason: "scheme"},
		{name: "file", in: "file:///etc/passwd", wantErr: true, reason: "scheme"},
		{name: "ftp", in: "ftp://example.com", wantErr: true, reason: "scheme"},
		{name: "mailto", in: "mailto:user@example.com", wantErr: true, reason: "scheme"},
		{name: "tel", in: "tel:+1234567", wantErr: true, reason: "scheme"},
		{name: "websocket", in: "ws://example.com", wantErr: true, reason: "scheme"},
		{name: "browser internal", in: "chrome://settings", wantErr: true, reason: "scheme"},

		// --- missing host ---
		{name: "http only", in: "http://", wantErr: true, reason: "host"},
		{name: "https only", in: "https://", wantErr: true, reason: "host"},
		{name: "triple slash", in: "http:///path", wantErr: true, reason: "host"},
		{name: "single slash", in: "https:/go.dev", wantErr: true, reason: "host"},
		{name: "opaque", in: "http:go.dev", wantErr: true, reason: "host"},
		{name: "port without hostname", in: "http://:8080", wantErr: true, reason: "host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := shortener.CheckURL(tt.in)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("checkURL(%q) error = %v, want nil", tt.in, err)
				}
				return
			}

			if !errors.Is(err, shortener.ErrInvalidURL) {
				t.Fatalf("checkURL(%q) error = %v, want %v", tt.in, err, shortener.ErrInvalidURL)
			}
			if tt.reason != "" && !strings.Contains(err.Error(), tt.reason) {
				t.Errorf("checkURL(%q) error = %q, want it to mention %q", tt.in, err, tt.reason)
			}
		})
	}
}
