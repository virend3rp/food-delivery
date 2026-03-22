package config

import (
	"fmt"
	"os"
)

// Required reads an env var and returns an error if it is not set.
// Use this for values that have no safe default (DB URLs, broker URLs).
func Required(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return v, nil
}

// WithDefault reads an env var, returning fallback when unset.
// Use this for values that have a safe local-dev default.
func WithDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
