package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestECRTokenFix(t *testing.T) {
	// This test demonstrates the fix for the token format issue in both methods

	// Create a fake client
	client := fake.NewSimpleClientset()

	// Create ECR manager config
	config := ECRManagerConfig{
		K8sClient:     client,
		GTokenNS:      "gtoken",
		ECRSecretName: "ecr-secret",
		RefreshBuffer: 5 * time.Minute,
	}

	// Create ECR manager
	manager := NewECRManager(&config)

	// Create a mock AWS token (base64 encoded "AWS:mockpassword")
	mockAwsToken := base64.StdEncoding.EncodeToString([]byte("AWS:mockpassword"))

	// Test CreateMasterSecret
	ctx := context.Background()
	err := manager.CreateMasterSecret(ctx, mockAwsToken)
	if err != nil {
		t.Fatalf("CreateMasterSecret failed: %v", err)
	}

	// Get the secret
	secret, err := client.CoreV1().Secrets("gtoken").Get(ctx, "ecr-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	// Check the secret data
	if _, ok := secret.Data[".dockerconfigjson"]; !ok {
		t.Fatal("Secret does not contain .dockerconfigjson")
	}

	// Parse the Docker config JSON
	var dockerConfig map[string]interface{}
	if err := json.Unmarshal(secret.Data[".dockerconfigjson"], &dockerConfig); err != nil {
		t.Fatalf("Failed to unmarshal Docker config: %v", err)
	}

	// Verify the Docker config has auths
	auths, ok := dockerConfig["auths"].(map[string]interface{})
	if !ok {
		t.Fatal("Docker config does not contain auths")
	}

	// Test AddECRRegistryToMaster
	registryURL := "123456789012.dkr.ecr.us-west-2.amazonaws.com"
	err = manager.AddECRRegistryToMaster(ctx, registryURL, mockAwsToken)
	if err != nil {
		t.Fatalf("AddECRRegistryToMaster failed: %v", err)
	}

	// Get the secret again
	secret, err = client.CoreV1().Secrets("gtoken").Get(ctx, "ecr-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get secret after AddECRRegistryToMaster: %v", err)
	}

	// Parse the Docker config JSON again
	if err := json.Unmarshal(secret.Data[".dockerconfigjson"], &dockerConfig); err != nil {
		t.Fatalf("Failed to unmarshal Docker config after AddECRRegistryToMaster: %v", err)
	}

	// Verify the Docker config has auths
	auths, ok = dockerConfig["auths"].(map[string]interface{})
	if !ok {
		t.Fatal("Docker config does not contain auths after AddECRRegistryToMaster")
	}

	// Verify the registry was added
	registry, ok := auths[registryURL].(map[string]interface{})
	if !ok {
		t.Fatalf("Registry %s not found in auths", registryURL)
	}

	// Verify the auth token
	auth, ok := registry["auth"].(string)
	if !ok {
		t.Fatal("Registry does not contain auth")
	}

	// Decode the auth token
	decodedAuth, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		t.Fatalf("Failed to decode auth token: %v", err)
	}

	// The decoded auth should be "AWS:mockpassword", not "AWS:<base64-encoded>"
	if string(decodedAuth) != "AWS:mockpassword" {
		t.Errorf("Expected auth to be 'AWS:mockpassword', got: %s", decodedAuth)
	}
}
