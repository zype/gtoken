package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestECRTokenFormat(t *testing.T) {
	// This test verifies the token format issue and demonstrates the fix

	// Simulate a token returned by AWS ECR GetAuthorizationToken API
	// AWS returns a base64-encoded token that contains "AWS:password"
	mockAwsToken := base64.StdEncoding.EncodeToString([]byte("AWS:mockpassword"))

	// Current implementation: prepends "AWS:" and encodes again
	currentFormat := base64.StdEncoding.EncodeToString([]byte("AWS:" + mockAwsToken))

	// When Docker decodes this, it gets "AWS:<still-encoded-token>"
	dockerSees, _ := base64.StdEncoding.DecodeString(currentFormat)
	t.Logf("Docker sees: %s", dockerSees)

	// Verify that what Docker sees starts with "AWS:" followed by a base64 encoded string
	if !strings.HasPrefix(string(dockerSees), "AWS:") {
		t.Errorf("Expected Docker to see string starting with 'AWS:', got: %s", dockerSees)
	}

	// Attempt to decode what follows after "AWS:" - it should be another base64 encoded string
	decodable := strings.TrimPrefix(string(dockerSees), "AWS:")
	if _, err := base64.StdEncoding.DecodeString(decodable); err != nil {
		t.Errorf("Expected Docker to see 'AWS:<base64-encoded>', but what follows 'AWS:' is not base64: %v", err)
	}

	// Fix 1: Decode the token first, then format correctly
	decodedToken, _ := base64.StdEncoding.DecodeString(mockAwsToken)
	fixedFormat1 := base64.StdEncoding.EncodeToString([]byte("AWS:" + string(decodedToken)))

	// When Docker decodes the fixed format, it should get "AWS:AWS:mockpassword"
	// This is still wrong because the AWS token already has "AWS:" prefix
	dockerSeesFixed1, _ := base64.StdEncoding.DecodeString(fixedFormat1)
	t.Logf("Docker sees with fix 1: %s", dockerSeesFixed1)

	// Fix 2: Use the token directly (since it already contains "AWS:")
	fixedFormat2 := mockAwsToken

	// When Docker decodes the fixed format, it should get "AWS:mockpassword"
	dockerSeesFixed2, _ := base64.StdEncoding.DecodeString(fixedFormat2)
	t.Logf("Docker sees with fix 2: %s", dockerSeesFixed2)

	// Verify that what Docker sees with fix 2 is correct
	if string(dockerSeesFixed2) != "AWS:mockpassword" {
		t.Errorf("Expected Docker to see 'AWS:mockpassword', got: %s", dockerSeesFixed2)
	}
}
