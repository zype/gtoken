package main

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_hasECRImages(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod with ECR image",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with non-ECR image",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "gcr.io/my-project/my-app:latest",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod with mixed images",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test1",
							Image: "gcr.io/my-project/my-app:latest",
						},
						{
							Name:  "test2",
							Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with init containers",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "init",
							Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/init:latest",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "gcr.io/my-project/my-app:latest",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "empty pod",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasECRImages(tt.pod)
			if got != tt.expected {
				t.Errorf("hasECRImages() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func Test_validateECRSecret(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		secret        *corev1.Secret
		client        kubernetes.Interface
		expectedValid bool
		expectedErr   bool
	}{
		{
			name:      "valid ECR secret",
			namespace: "test-ns",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ecr-secret",
					Namespace: "test-ns",
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(`{"auths":{"123456789012.dkr.ecr.us-west-2.amazonaws.com":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			},
			client:        fake.NewSimpleClientset(),
			expectedValid: true,
			expectedErr:   false,
		},
		{
			name:          "missing secret",
			namespace:     "test-ns",
			client:        fake.NewSimpleClientset(),
			expectedValid: false,
			expectedErr:   true,
		},
		{
			name:      "invalid secret type",
			namespace: "test-ns",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ecr-secret",
					Namespace: "test-ns",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
			client:        fake.NewSimpleClientset(),
			expectedValid: false,
			expectedErr:   true,
		},
		{
			name:      "invalid docker config",
			namespace: "test-ns",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ecr-secret",
					Namespace: "test-ns",
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(`invalid json`),
				},
			},
			client:        fake.NewSimpleClientset(),
			expectedValid: false,
			expectedErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the secret in the fake client if provided
			if tt.secret != nil {
				_, err := tt.client.CoreV1().Secrets(tt.namespace).Create(context.Background(), tt.secret, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create test secret: %v", err)
				}
			}

			valid, err := validateECRSecret(context.Background(), tt.client, tt.namespace)
			if (err != nil) != tt.expectedErr {
				t.Errorf("validateECRSecret() error = %v, wantErr %v", err, tt.expectedErr)
				return
			}
			if valid != tt.expectedValid {
				t.Errorf("validateECRSecret() valid = %v, want %v", valid, tt.expectedValid)
			}
		})
	}
}

func Test_ensureECRSecret(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		namespace      string
		secret         *corev1.Secret
		client         kubernetes.Interface
		expectedErr    bool
		expectedSecret bool
	}{
		{
			name: "pod with ECR image and valid secret",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
						},
					},
				},
			},
			namespace: "test-ns",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ecr-secret",
					Namespace: "test-ns",
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(`{"auths":{"123456789012.dkr.ecr.us-west-2.amazonaws.com":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			},
			client:         fake.NewSimpleClientset(),
			expectedErr:    false,
			expectedSecret: true,
		},
		{
			name: "pod without ECR image",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "gcr.io/my-project/my-app:latest",
						},
					},
				},
			},
			namespace:      "test-ns",
			client:         fake.NewSimpleClientset(),
			expectedErr:    false,
			expectedSecret: false,
		},
		{
			name: "pod with ECR image but invalid secret",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
						},
					},
				},
			},
			namespace:      "test-ns",
			client:         fake.NewSimpleClientset(),
			expectedErr:    true,
			expectedSecret: false,
		},
		{
			name: "pod with ECR image and existing imagePullSecret",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "ecr-secret"},
					},
				},
			},
			namespace: "test-ns",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ecr-secret",
					Namespace: "test-ns",
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(`{"auths":{"123456789012.dkr.ecr.us-west-2.amazonaws.com":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			},
			client:         fake.NewSimpleClientset(),
			expectedErr:    false,
			expectedSecret: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the secret in the fake client if provided
			if tt.secret != nil {
				_, err := tt.client.CoreV1().Secrets(tt.namespace).Create(context.Background(), tt.secret, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create test secret: %v", err)
				}
			}

			err := ensureECRSecret(context.Background(), tt.client, tt.pod, tt.namespace)
			if (err != nil) != tt.expectedErr {
				t.Errorf("ensureECRSecret() error = %v, wantErr %v", err, tt.expectedErr)
				return
			}

			// Check if the secret was added to imagePullSecrets
			found := false
			for _, secret := range tt.pod.Spec.ImagePullSecrets {
				if secret.Name == "ecr-secret" {
					found = true
					break
				}
			}
			if found != tt.expectedSecret {
				t.Errorf("ensureECRSecret() secret added = %v, want %v", found, tt.expectedSecret)
			}
		})
	}
}
