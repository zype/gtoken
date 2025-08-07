package main

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewECRMetrics(t *testing.T) {
	metrics := NewECRMetrics()
	if metrics == nil {
		t.Error("Expected non-nil ECRMetrics instance")
	}
}

func TestECRMetrics_RecordTokenRefresh(t *testing.T) {
	// Reset metrics before test
	ecrTokenRefreshTotal.Reset()
	ecrTokenRefreshDuration.Reset()

	metrics := NewECRMetrics()
	duration := 2 * time.Second

	// Test success case
	metrics.RecordTokenRefresh("success", duration)

	// Check counter
	successCount := testutil.ToFloat64(ecrTokenRefreshTotal.WithLabelValues("success"))
	if successCount != 1 {
		t.Errorf("Expected success count to be 1, got %f", successCount)
	}

	// Test failure case
	metrics.RecordTokenRefresh("failure", duration)

	// Check failure counter
	failureCount := testutil.ToFloat64(ecrTokenRefreshTotal.WithLabelValues("failure"))
	if failureCount != 1 {
		t.Errorf("Expected failure count to be 1, got %f", failureCount)
	}
}

func TestECRMetrics_RecordSecretCopy(t *testing.T) {
	// Reset metrics before test
	ecrSecretCopyTotal.Reset()
	ecrSecretCopyDuration.Reset()

	metrics := NewECRMetrics()
	duration := 1 * time.Second
	namespace := "test-namespace"

	// Test success case
	metrics.RecordSecretCopy(namespace, "success", duration)

	// Check counter
	successCount := testutil.ToFloat64(ecrSecretCopyTotal.WithLabelValues(namespace, "success"))
	if successCount != 1 {
		t.Errorf("Expected success count to be 1, got %f", successCount)
	}

	// Test failure case
	metrics.RecordSecretCopy(namespace, "failure", duration)

	// Check failure counter
	failureCount := testutil.ToFloat64(ecrSecretCopyTotal.WithLabelValues(namespace, "failure"))
	if failureCount != 1 {
		t.Errorf("Expected failure count to be 1, got %f", failureCount)
	}
}

func TestECRMetrics_RecordError(t *testing.T) {
	// Reset metrics before test
	ecrErrorsTotal.Reset()

	metrics := NewECRMetrics()
	operation := "test_operation"
	errorType := "test_error"

	metrics.RecordError(operation, errorType)

	// Check counter
	errorCount := testutil.ToFloat64(ecrErrorsTotal.WithLabelValues(operation, errorType))
	if errorCount != 1 {
		t.Errorf("Expected error count to be 1, got %f", errorCount)
	}
}

func TestECRMetrics_RecordConcurrentOperation(t *testing.T) {
	// Reset metrics before test
	ecrConcurrentOperations.Reset()

	metrics := NewECRMetrics()
	operation := "test_operation"
	count := 5

	metrics.RecordConcurrentOperation(operation, count)

	// Check gauge
	concurrentCount := testutil.ToFloat64(ecrConcurrentOperations.WithLabelValues(operation))
	if concurrentCount != float64(count) {
		t.Errorf("Expected concurrent count to be %d, got %f", count, concurrentCount)
	}
}

func TestECRMetrics_RecordTokenExpiry(t *testing.T) {
	// Reset metrics before test
	ecrTokenExpiryTime.Reset()

	metrics := NewECRMetrics()
	expiryTime := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	metrics.RecordTokenExpiry(expiryTime)

	// Check gauge
	recordedExpiry := testutil.ToFloat64(ecrTokenExpiryTime.WithLabelValues())
	expectedExpiry := float64(expiryTime.Unix())
	if recordedExpiry != expectedExpiry {
		t.Errorf("Expected expiry timestamp to be %f, got %f", expectedExpiry, recordedExpiry)
	}
}

func TestECRMetrics_RecordTokenVersion(t *testing.T) {
	// Reset metrics before test
	ecrTokenVersion.Reset()

	metrics := NewECRMetrics()
	version := "1234567890"

	metrics.RecordTokenVersion(version)

	// Check gauge
	recordedVersion := testutil.ToFloat64(ecrTokenVersion.WithLabelValues())
	expectedVersion := float64(1234567890)
	if recordedVersion != expectedVersion {
		t.Errorf("Expected token version to be %f, got %f", expectedVersion, recordedVersion)
	}

	// Test empty version
	ecrTokenVersion.Reset()
	metrics.RecordTokenVersion("")

	// Should not record anything for empty version
	recordedVersion = testutil.ToFloat64(ecrTokenVersion.WithLabelValues())
	if recordedVersion != 0 {
		t.Errorf("Expected token version to be 0 for empty string, got %f", recordedVersion)
	}
}

func TestECRMetrics_RecordManagedNamespaces(t *testing.T) {
	// Reset metrics before test
	ecrManagedNamespaces.Reset()

	metrics := NewECRMetrics()
	count := 10

	metrics.RecordManagedNamespaces(count)

	// Check gauge
	recordedCount := testutil.ToFloat64(ecrManagedNamespaces.WithLabelValues())
	if recordedCount != float64(count) {
		t.Errorf("Expected managed namespaces count to be %d, got %f", count, recordedCount)
	}
}

func TestECRMetrics_RecordAdmission(t *testing.T) {
	// Reset metrics before test
	ecrAdmissionTotal.Reset()

	metrics := NewECRMetrics()
	decision := "allow"
	reason := "ecr_validation_success"

	metrics.RecordAdmission(decision, reason)

	// Check counter
	admissionCount := testutil.ToFloat64(ecrAdmissionTotal.WithLabelValues(decision, reason))
	if admissionCount != 1 {
		t.Errorf("Expected admission count to be 1, got %f", admissionCount)
	}
}

func TestECRMetrics_HelperFunctions(t *testing.T) {
	// Reset metrics before test
	ecrErrorsTotal.Reset()

	metrics := NewECRMetrics()

	// Test helper functions
	metrics.RecordTokenFetchError()
	metrics.RecordSecretValidationError()
	metrics.RecordSecretCopyError()
	metrics.RecordConfigMapError()
	metrics.RecordNamespaceLabelError()

	// Check that all helper functions recorded errors
	tokenFetchErrors := testutil.ToFloat64(ecrErrorsTotal.WithLabelValues("token_fetch", "fetch_failed"))
	if tokenFetchErrors != 1 {
		t.Errorf("Expected token fetch errors to be 1, got %f", tokenFetchErrors)
	}

	secretValidationErrors := testutil.ToFloat64(ecrErrorsTotal.WithLabelValues("secret_validation", "validation_failed"))
	if secretValidationErrors != 1 {
		t.Errorf("Expected secret validation errors to be 1, got %f", secretValidationErrors)
	}

	secretCopyErrors := testutil.ToFloat64(ecrErrorsTotal.WithLabelValues("secret_copy", "copy_failed"))
	if secretCopyErrors != 1 {
		t.Errorf("Expected secret copy errors to be 1, got %f", secretCopyErrors)
	}

	configMapErrors := testutil.ToFloat64(ecrErrorsTotal.WithLabelValues("configmap", "operation_failed"))
	if configMapErrors != 1 {
		t.Errorf("Expected configmap errors to be 1, got %f", configMapErrors)
	}

	namespaceLabelErrors := testutil.ToFloat64(ecrErrorsTotal.WithLabelValues("namespace_label", "label_failed"))
	if namespaceLabelErrors != 1 {
		t.Errorf("Expected namespace label errors to be 1, got %f", namespaceLabelErrors)
	}
}
