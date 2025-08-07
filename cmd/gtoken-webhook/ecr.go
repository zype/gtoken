package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// hasECRImages checks if any container or init container in the pod uses an ECR image
func hasECRImages(pod *corev1.Pod) bool {
	// Check regular containers
	for _, container := range pod.Spec.Containers {
		if isECRImage(container.Image) {
			return true
		}
	}

	// Check init containers
	for _, container := range pod.Spec.InitContainers {
		if isECRImage(container.Image) {
			return true
		}
	}

	return false
}

// isECRImage checks if the given image string is from Amazon ECR
func isECRImage(image string) bool {
	return strings.Contains(image, ".dkr.ecr.") && strings.Contains(image, ".amazonaws.com/")
}

// extractECRRegistry extracts the ECR registry URL from an ECR image string
func extractECRRegistry(image string) string {
	// ECR images are in format: account.dkr.ecr.region.amazonaws.com/repository:tag
	// We want to extract: account.dkr.ecr.region.amazonaws.com
	if !isECRImage(image) {
		return ""
	}
	
	// Find the first slash (which separates registry from repository)
	slashIndex := strings.Index(image, "/")
	if slashIndex == -1 {
		return ""
	}
	
	return image[:slashIndex]
}

// extractECRRegistryFromPod extracts the ECR registry URL from the first ECR image found in a pod
func extractECRRegistryFromPod(pod *corev1.Pod) string {
	// Check regular containers first
	for _, container := range pod.Spec.Containers {
		if isECRImage(container.Image) {
			registry := extractECRRegistry(container.Image)
			if registry != "" {
				return registry
			}
		}
	}

	// Check init containers if no ECR image found in regular containers
	for _, container := range pod.Spec.InitContainers {
		if isECRImage(container.Image) {
			registry := extractECRRegistry(container.Image)
			if registry != "" {
				return registry
			}
		}
	}

	return ""
}

// dockerConfig represents the structure of a Docker config.json file
type dockerConfig struct {
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

// validateECRSecret checks if a valid ECR pull secret exists in the namespace
func validateECRSecret(ctx context.Context, client kubernetes.Interface, namespace string) (bool, error) {
	// Get the secret from the namespace
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, "ecr-secret", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get ECR secret: %w", err)
	}

	// Check if it's a docker config secret
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return false, fmt.Errorf("secret is not a docker config json")
	}

	// Get the docker config data
	dockerConfigData, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return false, fmt.Errorf("secret does not contain .dockerconfigjson data")
	}

	// Parse the docker config
	var config dockerConfig
	if err := json.Unmarshal(dockerConfigData, &config); err != nil {
		return false, fmt.Errorf("invalid docker config json: %w", err)
	}

	// Check if it contains any ECR registry entries
	for registry := range config.Auths {
		if strings.Contains(registry, ".dkr.ecr.") && strings.Contains(registry, ".amazonaws.com") {
			return true, nil
		}
	}

	return false, fmt.Errorf("no ECR registry entries found in docker config")
}

// ensureECRSecret ensures the ECR secret exists in the namespace and adds it to pod's imagePullSecrets
func ensureECRSecret(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod, namespace string) error {
	// Check if pod has ECR images
	if !hasECRImages(pod) {
		return nil
	}

	// Validate ECR secret exists and is valid
	valid, err := validateECRSecret(ctx, client, namespace)
	if err != nil {
		return fmt.Errorf("failed to validate ECR secret: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid ECR secret in namespace %s", namespace)
	}

	// Add ECR secret to pod's imagePullSecrets if not already present
	secretName := "ecr-secret"
	found := false
	for _, secret := range pod.Spec.ImagePullSecrets {
		if secret.Name == secretName {
			found = true
			break
		}
	}
	if !found {
		pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{
			Name: secretName,
		})
	}

	return nil
}
