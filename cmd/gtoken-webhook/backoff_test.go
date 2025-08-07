package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRetryWithBackoff_Success(t *testing.T) {
	config := DefaultBackoffConfig()
	config.MaxAttempts = 3

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	ctx := context.Background()
	err := RetryWithBackoff(ctx, config, operation)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoff_MaxAttemptsExceeded(t *testing.T) {
	config := DefaultBackoffConfig()
	config.MaxAttempts = 3

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("persistent error")
	}

	ctx := context.Background()
	err := RetryWithBackoff(ctx, config, operation)

	if err == nil {
		t.Error("Expected error, got success")
	}

	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("Expected error about failed attempts, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
	config := DefaultBackoffConfig()
	config.MaxAttempts = 5

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("temporary error")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RetryWithBackoff(ctx, config, operation)

	if err == nil {
		t.Error("Expected error due to context cancellation, got success")
	}

	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected error about cancellation, got: %v", err)
	}

	// Should have made at least one attempt before cancellation
	if attempts < 1 {
		t.Errorf("Expected at least 1 attempt, got %d", attempts)
	}
}

func TestRetryWithBackoff_ImmediateSuccess(t *testing.T) {
	config := DefaultBackoffConfig()

	attempts := 0
	operation := func() error {
		attempts++
		return nil
	}

	ctx := context.Background()
	err := RetryWithBackoff(ctx, config, operation)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected bool
	}{
		{
			name:     "connection refused",
			error:    errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "timeout error",
			error:    errors.New("timeout"),
			expected: true,
		},
		{
			name:     "rate limit",
			error:    errors.New("rate limit exceeded"),
			expected: true,
		},
		{
			name:     "server error",
			error:    errors.New("server error 500"),
			expected: true,
		},
		{
			name:     "tls handshake",
			error:    errors.New("tls handshake failed"),
			expected: true,
		},
		{
			name:     "permission denied",
			error:    errors.New("permission denied"),
			expected: false,
		},
		{
			name:     "not found",
			error:    errors.New("not found"),
			expected: false,
		},
		{
			name:     "nil error",
			error:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.error)
			if result != tt.expected {
				t.Errorf("IsRetryableError(%v) = %v, expected %v", tt.error, result, tt.expected)
			}
		})
	}
}

func TestDefaultBackoffConfig(t *testing.T) {
	config := DefaultBackoffConfig()

	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay 100ms, got %v", config.InitialDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay 30s, got %v", config.MaxDelay)
	}

	if config.MaxAttempts != 5 {
		t.Errorf("Expected MaxAttempts 5, got %d", config.MaxAttempts)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier 2.0, got %f", config.Multiplier)
	}

	if !config.Jitter {
		t.Error("Expected Jitter true, got false")
	}
}

func TestRetryWithBackoff_DelayCalculation(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		MaxAttempts:  4,
		Multiplier:   2.0,
		Jitter:       false, // Disable jitter for predictable testing
	}

	attempts := 0
	startTime := time.Now()

	operation := func() error {
		attempts++
		if attempts < 4 {
			return errors.New("temporary error")
		}
		return nil
	}

	ctx := context.Background()
	err := RetryWithBackoff(ctx, config, operation)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	elapsed := time.Since(startTime)

	// Expected delays: 100ms, 200ms, 400ms (total: 700ms)
	// Allow some tolerance for test execution time
	if elapsed < 600*time.Millisecond {
		t.Errorf("Expected at least 600ms elapsed time, got %v", elapsed)
	}
}
