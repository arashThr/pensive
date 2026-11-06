package rand

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// Bytes generates n cryptographically random bytes.
func Bytes(n int) ([]byte, error) {
	b := make([]byte, n)
	nRead, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("rand bytes: %w", err)
	}
	if nRead < n {
		return nil, fmt.Errorf("rand bytes: read %d < %d bytes", nRead, n)
	}
	return b, nil
}

// String returns a URL-safe base64-encoded random string derived from n bytes.
func String(n int) (string, error) {
	b, err := Bytes(n)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
