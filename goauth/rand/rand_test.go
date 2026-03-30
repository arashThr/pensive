package rand_test

import (
	"strings"
	"testing"

	"github.com/arashthr/goauth/rand"
)

func TestBytes_Length(t *testing.T) {
	for _, n := range []int{1, 16, 32, 64} {
		b, err := rand.Bytes(n)
		if err != nil {
			t.Fatalf("Bytes(%d) error: %v", n, err)
		}
		if len(b) != n {
			t.Errorf("Bytes(%d): got %d bytes, want %d", n, len(b), n)
		}
	}
}

func TestString_NotEmpty(t *testing.T) {
	s, err := rand.String(32)
	if err != nil {
		t.Fatalf("String(32) error: %v", err)
	}
	if s == "" {
		t.Error("String(32) returned empty string")
	}
}

func TestString_URLSafe(t *testing.T) {
	for i := 0; i < 50; i++ {
		s, err := rand.String(32)
		if err != nil {
			t.Fatalf("String(32) error: %v", err)
		}
		if strings.ContainsAny(s, "+/") {
			t.Errorf("String(32) contains non-URL-safe chars: %q", s)
		}
	}
}

func TestString_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, err := rand.String(32)
		if err != nil {
			t.Fatalf("String(32) error: %v", err)
		}
		if seen[s] {
			t.Errorf("duplicate random string detected: %q", s)
		}
		seen[s] = true
	}
}
