package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHealthzHandler_WebhookNotInitialized(t *testing.T) {
	// Reset global webhook
	globalWebhook = nil

	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthzHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusServiceUnavailable)
	}

	if rr.Body.String() != "Webhook not initialized\n" {
		t.Errorf("handler returned unexpected body: got %v", rr.Body.String())
	}
}

func TestSelftestHandler_WebhookNotInitialized(t *testing.T) {
	// Reset global webhook
	globalWebhook = nil

	req, err := http.NewRequest("GET", "/selftest", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(selftestHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusServiceUnavailable)
	}

	if rr.Body.String() != "Webhook not initialized\n" {
		t.Errorf("handler returned unexpected body: got %v", rr.Body.String())
	}
}

func TestHealthzHandler_WithECRManager(t *testing.T) {
	// Create a mock ECR manager
	client := fake.NewSimpleClientset()

	// Create master secret with token version annotation
	masterSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-secret",
			Namespace: "gtoken",
			Annotations: map[string]string{
				"ecr-token-version": "1234567890",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}
	_, err := client.CoreV1().Secrets("gtoken").Create(context.Background(), masterSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create master secret: %v", err)
	}

	// Create ConfigMap with token state
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-token-state",
			Namespace: "gtoken",
		},
		Data: map[string]string{
			"token-hash":    "test-hash",
			"expires-at":    time.Now().Add(time.Hour).Format(time.RFC3339),
			"last-updated":  time.Now().Format(time.RFC3339),
			"token-version": "1234567890",
		},
	}
	_, err = client.CoreV1().ConfigMaps("gtoken").Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Create ECR manager
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
	}
	ecrManager := NewECRManager(&config)

	// Set up global webhook
	globalWebhook = &mutatingWebhook{
		ecrManager: ecrManager,
	}

	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthzHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Parse JSON response
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["status"] != true {
		t.Errorf("Expected status to be true, got %v", response["status"])
	}

	if response["healthy"] != true {
		t.Errorf("Expected healthy to be true, got %v", response["healthy"])
	}
}

func TestHealthzHandler_ExpiredToken(t *testing.T) {
	// Create a mock ECR manager
	client := fake.NewSimpleClientset()

	// Create master secret with token version annotation
	masterSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-secret",
			Namespace: "gtoken",
			Annotations: map[string]string{
				"ecr-token-version": "1234567890",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}
	_, err := client.CoreV1().Secrets("gtoken").Create(context.Background(), masterSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create master secret: %v", err)
	}

	// Create ConfigMap with expired token state
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-token-state",
			Namespace: "gtoken",
		},
		Data: map[string]string{
			"token-hash":    "test-hash",
			"expires-at":    time.Now().Add(-time.Hour).Format(time.RFC3339), // Expired
			"last-updated":  time.Now().Format(time.RFC3339),
			"token-version": "1234567890",
		},
	}
	_, err = client.CoreV1().ConfigMaps("gtoken").Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Create ECR manager
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
	}
	ecrManager := NewECRManager(&config)

	// Set up global webhook
	globalWebhook = &mutatingWebhook{
		ecrManager: ecrManager,
	}

	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthzHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusServiceUnavailable)
	}

	// Parse JSON response
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["status"] != false {
		t.Errorf("Expected status to be false, got %v", response["status"])
	}

	if response["healthy"] != false {
		t.Errorf("Expected healthy to be false, got %v", response["healthy"])
	}

	errors, ok := response["errors"].([]interface{})
	if !ok {
		t.Fatal("Expected errors field in response")
	}

	if len(errors) == 0 {
		t.Error("Expected errors in response")
	}

	// Check for expired token error
	found := false
	for _, err := range errors {
		if errStr, ok := err.(string); ok && errStr == "ECR token is expired" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'ECR token is expired' error in response")
	}
}

func TestSelftestHandler_WithECRManager(t *testing.T) {
	// Create a mock ECR manager
	client := fake.NewSimpleClientset()

	// Create master secret with token version annotation
	masterSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-secret",
			Namespace: "gtoken",
			Annotations: map[string]string{
				"ecr-token-version": "1234567890",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}
	_, err := client.CoreV1().Secrets("gtoken").Create(context.Background(), masterSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create master secret: %v", err)
	}

	// Create ECR manager
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
	}
	ecrManager := NewECRManager(&config)

	// Set up global webhook
	globalWebhook = &mutatingWebhook{
		ecrManager: ecrManager,
	}

	req, err := http.NewRequest("GET", "/selftest", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(selftestHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusServiceUnavailable)
	}

	// Parse JSON response
	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if response["status"] != false {
		t.Errorf("Expected status to be false, got %v", response["status"])
	}

	if response["healthy"] != false {
		t.Errorf("Expected healthy to be false, got %v", response["healthy"])
	}

	results, ok := response["results"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected results field in response")
	}

	// Check that we have the expected test results
	if _, ok := results["sts_validation"]; !ok {
		t.Error("Expected sts_validation in results")
	}

	if _, ok := results["ecr_auth"]; !ok {
		t.Error("Expected ecr_auth in results")
	}

	if _, ok := results["secret_sync"]; !ok {
		t.Error("Expected secret_sync in results")
	}
}
