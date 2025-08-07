package main

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fake "k8s.io/client-go/kubernetes/fake"
)

func TestNewECRManager(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
	}

	manager := NewECRManager(&config)
	if manager == nil {
		t.Fatal("NewECRManager returned nil")
	}
}

func TestECRManager_GetECRToken(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
		TokenFilePath: "/var/run/secrets/aws/token",
		ECRRoleARN:    "arn:aws:iam::123456789012:role/ecr-access-role",
	}

	manager := NewECRManager(&config)
	ctx := context.Background()

	// Test that it returns an error when token file doesn't exist
	token, expiry, err := manager.GetECRToken(ctx)
	if err == nil {
		t.Error("Expected error when token file doesn't exist")
	}
	if token != "" {
		t.Errorf("Expected empty token, got %s", token)
	}
	if !expiry.IsZero() {
		t.Errorf("Expected zero expiry time, got %v", expiry)
	}
}

func TestECRManager_CreateMasterSecret(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
	}

	manager := NewECRManager(&config)
	ctx := context.Background()

	// Test that it works with a valid token
	err := manager.CreateMasterSecret(ctx, "test-token")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test that it fails with empty token
	err = manager.CreateMasterSecret(ctx, "")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

func TestECRManager_CreateMasterSecret_Integration(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		namespace   string
		secretName  string
		expectError bool
	}{
		{
			name:        "create new master secret",
			token:       "test-ecr-token",
			namespace:   "gtoken",
			secretName:  "ecr-secret",
			expectError: false,
		},
		{
			name:        "update existing master secret",
			token:       "updated-ecr-token",
			namespace:   "gtoken",
			secretName:  "ecr-secret",
			expectError: false,
		},
		{
			name:        "empty token should fail",
			token:       "",
			namespace:   "gtoken",
			secretName:  "ecr-secret",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			config := ECRManagerConfig{
				K8sClient:     client,
				GTokenNS:      tt.namespace,
				ECRSecretName: tt.secretName,
				RefreshBuffer: 5 * time.Minute,
			}

			manager := NewECRManager(&config)
			ctx := context.Background()

			// TODO: Implement CreateMasterSecret and test actual functionality
			err := manager.CreateMasterSecret(ctx, tt.token)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestECRManager_CopySecretToNamespace(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
	}

	manager := NewECRManager(&config)
	ctx := context.Background()

	// Test that it returns an error (not implemented yet)
	err := manager.CopySecretToNamespace(ctx, "test-namespace")
	if err == nil {
		t.Error("Expected error for unimplemented CopySecretToNamespace")
	}
}

func TestECRManager_CopySecretToNamespace_Integration(t *testing.T) {
	tests := []struct {
		name         string
		targetNS     string
		createMaster bool
		expectError  bool
		expectSecret bool
	}{
		{
			name:         "copy secret to new namespace",
			targetNS:     "app-namespace",
			createMaster: true,
			expectError:  false,
			expectSecret: true,
		},
		{
			name:         "copy secret to existing namespace",
			targetNS:     "app-namespace",
			createMaster: true,
			expectError:  false,
			expectSecret: true,
		},
		{
			name:         "fail when master secret doesn't exist",
			targetNS:     "app-namespace",
			createMaster: false,
			expectError:  true,
			expectSecret: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			config := ECRManagerConfig{
				K8sClient:     client,
				GTokenNS:      "gtoken",
				ECRSecretName: "ecr-secret",
				RefreshBuffer: 5 * time.Minute,
			}

			manager := NewECRManager(&config)
			ctx := context.Background()
			var err error

			// Create the target namespace
			_, createErr := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: tt.targetNS},
			}, metav1.CreateOptions{})
			if createErr != nil {
				t.Fatalf("Failed to create namespace: %v", createErr)
			}

			// Create master secret if needed
			if tt.createMaster {
				err = manager.CreateMasterSecret(ctx, "test-token")
				if err != nil {
					t.Fatalf("Failed to create master secret: %v", err)
				}
			}

			// Test copying secret
			err = manager.CopySecretToNamespace(ctx, tt.targetNS)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Verify secret was copied if expected
			if tt.expectSecret {
				secret, err := client.CoreV1().Secrets(tt.targetNS).Get(ctx, "ecr-secret", metav1.GetOptions{})
				if err != nil {
					t.Errorf("Expected secret to exist but got error: %v", err)
				}
				if secret.Type != corev1.SecretTypeDockerConfigJson {
					t.Errorf("Expected secret type %s, got %s", corev1.SecretTypeDockerConfigJson, secret.Type)
				}
			}
		})
	}
}

func TestECRManager_ValidateSecret(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
	}

	manager := NewECRManager(&config)
	ctx := context.Background()

	// Test that it returns an error (not implemented yet)
	valid, err := manager.ValidateSecret(ctx, "test-namespace")
	if err == nil {
		t.Error("Expected error for unimplemented ValidateSecret")
	}
	if valid {
		t.Error("Expected false for unimplemented ValidateSecret")
	}
}

func TestECRManager_ValidateSecret_Integration(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		createSecret bool
		secretValid  bool
		expectValid  bool
		expectError  bool
	}{
		{
			name:         "valid secret exists",
			namespace:    "app-namespace",
			createSecret: true,
			secretValid:  true,
			expectValid:  true,
			expectError:  false,
		},
		{
			name:         "invalid secret exists",
			namespace:    "app-namespace",
			createSecret: true,
			secretValid:  false,
			expectValid:  false,
			expectError:  true,
		},
		{
			name:         "no secret exists",
			namespace:    "app-namespace",
			createSecret: false,
			secretValid:  false,
			expectValid:  false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			config := ECRManagerConfig{
				K8sClient:     client,
				GTokenNS:      "gtoken",
				ECRSecretName: "ecr-secret",
				RefreshBuffer: 5 * time.Minute,
			}

			manager := NewECRManager(&config)
			ctx := context.Background()

			// Create secret if needed
			if tt.createSecret {
				var secretData []byte
				if tt.secretValid {
					// Valid ECR secret
					dockerConfig := map[string]interface{}{
						"auths": map[string]interface{}{
							"123456789012.dkr.ecr.us-west-2.amazonaws.com": map[string]string{
								"auth": "dXNlcjpwYXNz",
							},
						},
					}
					secretData, _ = json.Marshal(dockerConfig)
				} else {
					// Invalid secret
					secretData = []byte("invalid")
				}

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ecr-secret",
						Namespace: tt.namespace,
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						".dockerconfigjson": secretData,
					},
				}
				_, err := client.CoreV1().Secrets(tt.namespace).Create(ctx, secret, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test secret: %v", err)
				}
			}

			// Test validation
			valid, err := manager.ValidateSecret(ctx, tt.namespace)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got %v", tt.expectValid, valid)
			}
		})
	}
}

func TestECRManager_StartTokenRefresh(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
		TokenFilePath: "/var/run/secrets/aws/token",
		ECRRoleARN:    "arn:aws:iam::123456789012:role/ecr-access-role",
	}

	manager := NewECRManager(&config)
	ctx := context.Background()

	// Test that it starts successfully
	err := manager.StartTokenRefresh(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Stop the manager
	manager.Stop()
}

func TestECRManager_Stop(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
	}

	manager := NewECRManager(&config)

	// Test that Stop doesn't panic
	manager.Stop()

	// Test that we can call Stop multiple times without issues
	manager.Stop()
}

func TestECRManager_GetECRToken_WithSharedVolume(t *testing.T) {
	// Test that the ECR manager can read AWS credentials from shared volume
	// This simulates the webhook having its own sidecar that generates AWS credentials
	config := ECRManagerConfig{
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
		TokenFilePath: "/var/run/secrets/aws/token",
		ECRRoleARN:    "arn:aws:iam::123456789012:role/ecr-access-role",
	}

	manager := NewECRManager(&config)
	ctx := context.Background()

	// Test that it returns an error when token file doesn't exist
	token, expiry, err := manager.GetECRToken(ctx)
	if err == nil {
		t.Error("Expected error when token file doesn't exist")
	}
	if token != "" {
		t.Errorf("Expected empty token, got %s", token)
	}
	if !expiry.IsZero() {
		t.Errorf("Expected zero expiry time, got %v", expiry)
	}
}

func TestECRManager_RefreshAllNamespaces(t *testing.T) {
	// Create a test ECR manager with a fake client
	ctx := context.Background()
	k8sClient := fake.NewSimpleClientset()

	// Create test namespaces with ECR management labels
	testNamespaces := []string{"namespace1", "namespace2", "namespace3"}
	for _, ns := range testNamespaces {
		_, err := k8sClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
				Labels: map[string]string{
					"gtoken.doit-intl.com/ecr-managed": "true",
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create namespace %s: %v", ns, err)
		}
	}

	config := ECRManagerConfig{
		K8sClient:     k8sClient,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
		ECRRoleARN:    "arn:aws:iam::123456789012:role/ecr-role",
		GTokenImage:   "gtoken:latest",
		TokenFilePath: "/var/run/secrets/aws/token",
	}
	em := NewECRManager(&config).(*ecrManager)

	// Test that RefreshAllNamespaces works (should handle missing secrets gracefully)
	err := em.RefreshAllNamespaces(ctx, "test-token")

	// Should not error (it should log errors but continue)
	if err != nil {
		t.Errorf("RefreshAllNamespaces should not error: %v", err)
	}
}

func TestECRManager_RefreshAllNamespaces_Integration(t *testing.T) {
	// Create a test environment
	ctx := context.Background()
	k8sClient := fake.NewSimpleClientset()

	// Create the gtoken namespace
	_, err := k8sClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "gtoken"},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create gtoken namespace: %v", err)
	}

	// Create a master ECR secret
	masterSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-secret",
			Namespace: "gtoken",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"https://123456789012.dkr.ecr.us-west-2.amazonaws.com":{"auth":"QVdTOmZha2UtdG9rZW4="}}}`),
		},
	}
	_, err = k8sClient.CoreV1().Secrets("gtoken").Create(ctx, masterSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create master ECR secret: %v", err)
	}

	// Create test namespaces
	testNamespaces := []string{"namespace1", "namespace2", "namespace3"}
	for _, ns := range testNamespaces {
		_, err := k8sClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create namespace %s: %v", ns, err)
		}
	}

	// Create ECR manager
	config := ECRManagerConfig{
		K8sClient:     k8sClient,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
		ECRRoleARN:    "arn:aws:iam::123456789012:role/ecr-role",
		GTokenImage:   "gtoken:latest",
		TokenFilePath: "/var/run/secrets/aws/token",
	}
	em := NewECRManager(&config).(*ecrManager)

	// Copy secrets to test namespaces to track them
	for _, ns := range testNamespaces {
		err := em.CopySecretToNamespace(ctx, ns)
		if err != nil {
			t.Fatalf("Failed to copy secret to namespace %s: %v", ns, err)
		}
	}

	// Verify namespaces are labeled
	for _, ns := range testNamespaces {
		namespaceObj, err := k8sClient.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Failed to get namespace %s: %v", ns, err)
			continue
		}
		if namespaceObj.Labels["gtoken.doit-intl.com/ecr-managed"] != "true" {
			t.Errorf("Namespace %s should be labeled as ECR-managed", ns)
		}
	}

	// Test RefreshAllNamespaces
	err = em.RefreshAllNamespaces(ctx, "new-token")
	if err != nil {
		t.Errorf("RefreshAllNamespaces failed: %v", err)
	}

	// Verify secrets were updated in all namespaces
	for _, ns := range testNamespaces {
		secret, err := k8sClient.CoreV1().Secrets(ns).Get(ctx, "ecr-secret", metav1.GetOptions{})
		if err != nil {
			t.Errorf("Failed to get secret in namespace %s: %v", ns, err)
			continue
		}
		if secret.Type != corev1.SecretTypeDockerConfigJson {
			t.Errorf("Secret in namespace %s has wrong type: %s", ns, secret.Type)
		}
	}
}

func TestECRManager_UpdateTokenState(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
		ECRRoleARN:    "arn:aws:iam::123456789012:role/ecr-role",
		GTokenImage:   "test-image",
		TokenFilePath: "/test/path",
	}
	em := NewECRManager(&config)

	ctx := context.Background()
	token := "test-token"
	expiresAt := time.Now().Add(time.Hour)

	// Test creating new ConfigMap
	err := em.UpdateTokenState(ctx, token, expiresAt)
	if err != nil {
		t.Fatalf("UpdateTokenState failed: %v", err)
	}

	// Verify ConfigMap was created
	configMap, err := client.CoreV1().ConfigMaps("gtoken").Get(ctx, "ecr-token-state", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap: %v", err)
	}

	// Check required fields
	if configMap.Data["token-hash"] == "" {
		t.Error("token-hash not found in ConfigMap")
	}
	if configMap.Data["expires-at"] == "" {
		t.Error("expires-at not found in ConfigMap")
	}
	if configMap.Data["last-updated"] == "" {
		t.Error("last-updated not found in ConfigMap")
	}
	if configMap.Data["token-version"] == "" {
		t.Error("token-version not found in ConfigMap")
	}

	// Test updating existing ConfigMap
	newToken := "new-test-token"
	newExpiresAt := time.Now().Add(2 * time.Hour)
	err = em.UpdateTokenState(ctx, newToken, newExpiresAt)
	if err != nil {
		t.Fatalf("UpdateTokenState failed on update: %v", err)
	}

	// Verify ConfigMap was updated
	updatedConfigMap, err := client.CoreV1().ConfigMaps("gtoken").Get(ctx, "ecr-token-state", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated ConfigMap: %v", err)
	}

	if updatedConfigMap.Data["token-hash"] == configMap.Data["token-hash"] {
		t.Error("token-hash should have changed")
	}
}

func TestECRManager_GetTokenState(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
		ECRRoleARN:    "arn:aws:iam::123456789012:role/ecr-role",
		GTokenImage:   "test-image",
		TokenFilePath: "/test/path",
	}
	em := NewECRManager(&config)

	ctx := context.Background()

	// Test getting non-existent ConfigMap
	_, _, err := em.GetTokenState(ctx)
	if err == nil {
		t.Error("Expected error when ConfigMap doesn't exist")
	}

	// Create ConfigMap
	expiresAt := time.Now().Add(time.Hour)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-token-state",
			Namespace: "gtoken",
		},
		Data: map[string]string{
			"token-hash":    "test-hash",
			"expires-at":    expiresAt.Format(time.RFC3339),
			"last-updated":  time.Now().Format(time.RFC3339),
			"token-version": "1234567890",
		},
	}
	_, err = client.CoreV1().ConfigMaps("gtoken").Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	// Test getting existing ConfigMap
	tokenHash, retrievedExpiresAt, err := em.GetTokenState(ctx)
	if err != nil {
		t.Fatalf("GetTokenState failed: %v", err)
	}

	if tokenHash != "test-hash" {
		t.Errorf("Expected token-hash 'test-hash', got '%s'", tokenHash)
	}

	// Allow small time difference due to parsing
	if math.Abs(retrievedExpiresAt.Sub(expiresAt).Seconds()) > 1 {
		t.Errorf("Expected expires-at %v, got %v", expiresAt, retrievedExpiresAt)
	}
}

func TestECRManager_GetTokenState_InvalidData(t *testing.T) {
	client := fake.NewSimpleClientset()
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
		ECRRoleARN:    "arn:aws:iam::123456789012:role/ecr-role",
		GTokenImage:   "test-image",
		TokenFilePath: "/test/path",
	}
	em := NewECRManager(&config)

	ctx := context.Background()

	// Create ConfigMap with missing token-hash
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-token-state",
			Namespace: "gtoken",
		},
		Data: map[string]string{
			"expires-at":    time.Now().Add(time.Hour).Format(time.RFC3339),
			"last-updated":  time.Now().Format(time.RFC3339),
			"token-version": "1234567890",
		},
	}
	_, err := client.CoreV1().ConfigMaps("gtoken").Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	// Test getting ConfigMap with missing token-hash
	_, _, err = em.GetTokenState(ctx)
	if err == nil {
		t.Error("Expected error when token-hash is missing")
	}
	if !strings.Contains(err.Error(), "token-hash not found") {
		t.Errorf("Expected error about missing token-hash, got: %v", err)
	}
}

func TestECRManager_GetSecretTokenVersion(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
	}

	em := NewECRManager(&config)

	// Test case: secret doesn't exist
	_, err := em.GetSecretTokenVersion(ctx, "test-ns")
	if err == nil {
		t.Error("Expected error when secret doesn't exist")
	}

	// Test case: secret exists but has no annotations
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-secret",
			Namespace: "test-ns",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}
	_, err = client.CoreV1().Secrets("test-ns").Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test secret: %v", err)
	}

	_, err = em.GetSecretTokenVersion(ctx, "test-ns")
	if err == nil {
		t.Error("Expected error when secret has no annotations")
	}

	// Test case: secret exists but missing ecr-token-version annotation
	secret.Annotations = map[string]string{
		"other-annotation": "value",
	}
	_, err = client.CoreV1().Secrets("test-ns").Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update test secret: %v", err)
	}

	_, err = em.GetSecretTokenVersion(ctx, "test-ns")
	if err == nil {
		t.Error("Expected error when secret missing ecr-token-version annotation")
	}

	// Test case: secret exists with ecr-token-version annotation
	secret.Annotations["ecr-token-version"] = "1234567890"
	_, err = client.CoreV1().Secrets("test-ns").Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update test secret: %v", err)
	}

	tokenVersion, err := em.GetSecretTokenVersion(ctx, "test-ns")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if tokenVersion != "1234567890" {
		t.Errorf("Expected token version '1234567890', got '%s'", tokenVersion)
	}
}

func TestECRManager_CreateMasterSecret_WithTokenVersion(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
	}

	em := NewECRManager(&config)

	// Create master secret
	err := em.CreateMasterSecret(ctx, "test-token")
	if err != nil {
		t.Fatalf("Failed to create master secret: %v", err)
	}

	// Verify secret was created with token version annotation
	secret, err := client.CoreV1().Secrets("gtoken").Get(ctx, "ecr-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get created secret: %v", err)
	}

	if secret.Annotations == nil {
		t.Error("Secret should have annotations")
	}

	tokenVersion, exists := secret.Annotations["ecr-token-version"]
	if !exists {
		t.Error("Secret should have ecr-token-version annotation")
	}

	if tokenVersion == "" {
		t.Error("Token version should not be empty")
	}

	// Verify token version is a valid Unix timestamp
	_, err = strconv.ParseInt(tokenVersion, 10, 64)
	if err != nil {
		t.Errorf("Token version should be a valid Unix timestamp: %v", err)
	}
}

func TestECRManager_CopySecretToNamespace_WithTokenVersion(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	// Create test namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
		},
	}
	_, err := client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test namespace: %v", err)
	}

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
	_, err = client.CoreV1().Secrets("gtoken").Create(ctx, masterSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create master secret: %v", err)
	}

	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
	}

	em := NewECRManager(&config)

	// Copy secret to namespace
	err = em.CopySecretToNamespace(ctx, "test-ns")
	if err != nil {
		t.Fatalf("Failed to copy secret: %v", err)
	}

	// Verify copied secret has token version annotation
	copiedSecret, err := client.CoreV1().Secrets("test-ns").Get(ctx, "ecr-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get copied secret: %v", err)
	}

	if copiedSecret.Annotations == nil {
		t.Error("Copied secret should have annotations")
	}

	tokenVersion, exists := copiedSecret.Annotations["ecr-token-version"]
	if !exists {
		t.Error("Copied secret should have ecr-token-version annotation")
	}

	if tokenVersion != "1234567890" {
		t.Errorf("Expected token version '1234567890', got '%s'", tokenVersion)
	}
}
