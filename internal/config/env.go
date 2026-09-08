// Package config reads service settings from environment variables.
package config

import (
	"fmt"
	"os"
	"time"
)

// String returns the variable's value or fallback.
func String(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// MustString returns the value of a required variable.
func MustString(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("environment variable %s is required", name)
	}
	return v, nil
}

// Duration parses the variable as a Go duration (e.g. 500ms, 5m, 24h).
func Duration(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s: %w", name, err)
	}
	return d, nil
}
