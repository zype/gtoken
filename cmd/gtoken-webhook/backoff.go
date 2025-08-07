package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"time"
)

// BackoffConfig holds configuration for exponential backoff
type BackoffConfig struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	MaxAttempts  int
	Multiplier   float64
	Jitter       bool
}

// DefaultBackoffConfig returns a reasonable default backoff configuration
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		MaxAttempts:  5,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// RetryWithBackoff executes a function with exponential backoff
func RetryWithBackoff(ctx context.Context, config BackoffConfig, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check if context is canceled
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation canceled: %w", ctx.Err())
		default:
		}

		// Execute the operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't sleep on the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate delay with exponential backoff
		delay := config.InitialDelay * time.Duration(math.Pow(config.Multiplier, float64(attempt)))
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		// Add jitter if enabled
		if config.Jitter {
			// Use crypto/rand for secure random jitter
			jitterMax := int64(float64(delay) * 0.1) // 10% jitter
			if jitterMax > 0 {
				jitterVal, err := rand.Int(rand.Reader, big.NewInt(jitterMax))
				if err == nil {
					delay += time.Duration(jitterVal.Int64())
				}
			}
		}

		// Wait for the calculated delay or context cancellation
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("operation canceled during backoff: %w", ctx.Err())
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Common retryable error patterns
	errStr := err.Error()
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary",
		"retry",
		"rate limit",
		"throttle",
		"server error",
		"internal error",
		"unavailable",
		"network error",
		"tls handshake",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(s) == len(substr) && s == substr ||
			len(s) > len(substr) && (s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr)))
}

// containsSubstring checks if a string contains a substring (case-insensitive)
func containsSubstring(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase (simplified version)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
