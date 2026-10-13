package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ValidateTurnstile verifies a Cloudflare Turnstile token against the siteverify API.
// Returns nil on success. If secretKey is empty, validation is skipped (dev mode).
func ValidateTurnstile(token, secretKey, remoteIP string) error {
	if secretKey == "" {
		return nil // CAPTCHA disabled
	}
	if token == "" {
		return fmt.Errorf("turnstile: token is required")
	}

	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
		"secret":   {secretKey},
		"response": {token},
		"remoteip": {remoteIP},
	})
	if err != nil {
		return fmt.Errorf("turnstile siteverify request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("turnstile read response: %w", err)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("turnstile parse response: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("turnstile verification failed")
	}
	return nil
}
