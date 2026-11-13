package shortener_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/matchbull6/go_url_shortener/internal/shortener"
)

func TestGenerateCode_Length(t *testing.T) {
	for _, length := range []int{4, 5, 6, 7} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			code, err := shortener.GenerateCode(length)
			if err != nil {
				t.Fatalf("generateCode(%d) error = %v, want nil", length, err)
			}
			if len(code) != length {
				t.Errorf("generateCode(%d) = %q, len %d, want %d", length, code, len(code), length)
			}
		})
	}
}

func TestGenerateCode_OnlyAlphabetChars(t *testing.T) {
	for range 100 {
		code, err := shortener.GenerateCode(shortener.DefaultCodeLength)
		if err != nil {
			t.Fatalf("generateCode(%d) error = %v, want nil", shortener.DefaultCodeLength, err)
		}
		for i, c := range code {
			if !strings.ContainsRune(shortener.Chars, c) {
				t.Fatalf("generateCode(%d) = %q: char %q at index %d is not in alphabet", shortener.DefaultCodeLength, code, c, i)
			}
		}
	}
}

func TestGenerateCode_Unique(t *testing.T) {
	const n = 1000
	seen := make(map[string]int, n)
	for i := range n {
		code, err := shortener.GenerateCode(shortener.DefaultCodeLength)
		if err != nil {
			t.Fatalf("generateCode(%d) error = %v, want nil", shortener.DefaultCodeLength, err)
		}
		if first, dup := seen[code]; dup {
			t.Fatalf("generateCode(%d) returned %q on calls #%d and #%d", shortener.DefaultCodeLength, code, first, i)
		}
		seen[code] = i
	}
}

func TestGenerateCode_UsesWholeAlphabet(t *testing.T) {
	seen := make(map[rune]bool, len(shortener.Chars))
	for range 1000 {
		code, err := shortener.GenerateCode(shortener.DefaultCodeLength)
		if err != nil {
			t.Fatalf("generateCode(%d) error = %v, want nil", shortener.DefaultCodeLength, err)
		}
		for _, c := range code {
			seen[c] = true
		}
	}

	var missing []rune
	for _, c := range shortener.Chars {
		if !seen[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		t.Errorf("after 1000 codes, chars never generated: %q", string(missing))
	}
}
