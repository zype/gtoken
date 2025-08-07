package main

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ECR token refresh metrics
	ecrTokenRefreshTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecr_token_refresh_total",
			Help: "Total number of ECR token refresh cycles",
		},
		[]string{"status"}, // success, failure
	)

	ecrTokenRefreshDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ecr_token_refresh_duration_seconds",
			Help:    "Duration of ECR token refresh operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"}, // success, failure
	)

	// Secret copy operation metrics
	ecrSecretCopyTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecr_secret_copy_total",
			Help: "Total number of ECR secret copy operations",
		},
		[]string{"namespace", "status"}, // namespace, success, failure
	)

	ecrSecretCopyDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ecr_secret_copy_duration_seconds",
			Help:    "Duration of ECR secret copy operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "status"}, // namespace, success, failure
	)

	// Error rate metrics
	ecrErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecr_errors_total",
			Help: "Total number of ECR-related errors",
		},
		[]string{"operation", "error_type"}, // operation, error_type
	)

	// Concurrent operations metrics
	ecrConcurrentOperations = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ecr_concurrent_operations",
			Help: "Number of concurrent ECR operations",
		},
		[]string{"operation"}, // operation type
	)

	// Token state metrics
	ecrTokenExpiryTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ecr_token_expiry_timestamp",
			Help: "Unix timestamp when the current ECR token expires",
		},
		[]string{}, // no labels
	)

	ecrTokenVersion = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ecr_token_version",
			Help: "Current ECR token version (Unix timestamp)",
		},
		[]string{}, // no labels
	)

	// Namespace management metrics
	ecrManagedNamespaces = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ecr_managed_namespaces_total",
			Help: "Total number of namespaces managed by ECR",
		},
		[]string{}, // no labels
	)

	// Admission metrics
	ecrAdmissionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ecr_admission_total",
			Help: "Total number of ECR-related admission decisions",
		},
		[]string{"decision", "reason"}, // decision (allow, deny), reason
	)
)

// ECRMetrics provides methods to record ECR-related metrics
type ECRMetrics struct{}

// NewECRMetrics creates a new ECR metrics instance
func NewECRMetrics() *ECRMetrics {
	return &ECRMetrics{}
}

// RecordTokenRefresh records a token refresh operation
func (m *ECRMetrics) RecordTokenRefresh(status string, duration time.Duration) {
	ecrTokenRefreshTotal.WithLabelValues(status).Inc()
	ecrTokenRefreshDuration.WithLabelValues(status).Observe(duration.Seconds())
}

// RecordSecretCopy records a secret copy operation
func (m *ECRMetrics) RecordSecretCopy(namespace, status string, duration time.Duration) {
	ecrSecretCopyTotal.WithLabelValues(namespace, status).Inc()
	ecrSecretCopyDuration.WithLabelValues(namespace, status).Observe(duration.Seconds())
}

// RecordError records an ECR-related error
func (m *ECRMetrics) RecordError(operation, errorType string) {
	ecrErrorsTotal.WithLabelValues(operation, errorType).Inc()
}

// RecordConcurrentOperation tracks concurrent operations
func (m *ECRMetrics) RecordConcurrentOperation(operation string, count int) {
	ecrConcurrentOperations.WithLabelValues(operation).Set(float64(count))
}

// RecordTokenExpiry records the token expiry timestamp
func (m *ECRMetrics) RecordTokenExpiry(expiryTime time.Time) {
	ecrTokenExpiryTime.WithLabelValues().Set(float64(expiryTime.Unix()))
}

// RecordTokenVersion records the current token version
func (m *ECRMetrics) RecordTokenVersion(version string) {
	if version != "" {
		// Try to parse version as Unix timestamp
		if timestamp, err := strconv.ParseInt(version, 10, 64); err == nil {
			ecrTokenVersion.WithLabelValues().Set(float64(timestamp))
		}
	}
}

// RecordManagedNamespaces records the number of managed namespaces
func (m *ECRMetrics) RecordManagedNamespaces(count int) {
	ecrManagedNamespaces.WithLabelValues().Set(float64(count))
}

// RecordAdmission records an admission decision
func (m *ECRMetrics) RecordAdmission(decision, reason string) {
	ecrAdmissionTotal.WithLabelValues(decision, reason).Inc()
}

// Helper functions for common error types
func (m *ECRMetrics) RecordTokenFetchError() {
	m.RecordError("token_fetch", "fetch_failed")
}

func (m *ECRMetrics) RecordSecretValidationError() {
	m.RecordError("secret_validation", "validation_failed")
}

func (m *ECRMetrics) RecordSecretCopyError() {
	m.RecordError("secret_copy", "copy_failed")
}

func (m *ECRMetrics) RecordConfigMapError() {
	m.RecordError("configmap", "operation_failed")
}

func (m *ECRMetrics) RecordNamespaceLabelError() {
	m.RecordError("namespace_label", "label_failed")
}
