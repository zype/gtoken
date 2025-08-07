package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	cmp "github.com/google/go-cmp/cmp"
	whmodel "github.com/slok/kubewebhook/v2/pkg/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fake "k8s.io/client-go/kubernetes/fake"
)

// mockECRManager is a mock implementation of ECRManager for testing
type mockECRManager struct {
	copySecretErr error
}

func (m *mockECRManager) GetECRToken(ctx context.Context) (string, time.Time, error) {
	return "mock-token", time.Now().Add(time.Hour), nil
}

func (m *mockECRManager) CreateMasterSecret(ctx context.Context, token string) error {
	return nil
}

func (m *mockECRManager) AddECRRegistryToMaster(ctx context.Context, registryURL, token string) error {
	return nil
}

func (m *mockECRManager) CopySecretToNamespace(ctx context.Context, namespace string) error {
	return m.copySecretErr
}

func (m *mockECRManager) ValidateSecret(ctx context.Context, namespace string) (bool, error) {
	return true, nil
}

func (m *mockECRManager) StartTokenRefresh(ctx context.Context) error {
	return nil
}

func (m *mockECRManager) Stop() {
}

func (m *mockECRManager) RefreshAllNamespaces(ctx context.Context, token string) error {
	return nil
}

func (m *mockECRManager) UpdateTokenState(ctx context.Context, token string, expiresAt time.Time) error {
	return nil
}

func (m *mockECRManager) GetTokenState(ctx context.Context) (string, time.Time, error) {
	return "mock-hash", time.Now().Add(time.Hour), nil
}

func (m *mockECRManager) GetSecretTokenVersion(ctx context.Context, namespace string) (string, error) {
	return "mock-token-version", nil
}

// createFakeClientWithServiceAccount creates a fake client with a default service account
func createFakeClientWithServiceAccount() kubernetes.Interface {
	client := fake.NewSimpleClientset()
	_, err := client.CoreV1().ServiceAccounts("test-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "test-ns",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	return client
}

func TestMain(m *testing.M) {
	testMode = true
	os.Exit(m.Run())
}

//nolint:funlen
func Test_mutatingWebhook_mutateContainers(t *testing.T) {
	type fields struct {
		k8sClient  kubernetes.Interface
		image      string
		pullPolicy string
		volumeName string
		volumePath string
		tokenFile  string
	}
	type args struct {
		containers []corev1.Container
		roleArn    string
		ns         string
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		mutated          bool
		wantedContainers []corev1.Container
	}{
		{
			name: "mutate single container",
			fields: fields{
				k8sClient:  fake.NewSimpleClientset(),
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				containers: []corev1.Container{
					{
						Name:  "TestContainer",
						Image: "test-image",
					},
				},
				roleArn: "arn:aws:iam::123456789012:role/testrole",
				ns:      "test-namespace",
			},
			wantedContainers: []corev1.Container{
				{
					Name:         "TestContainer",
					Image:        "test-image",
					VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
					Env: []corev1.EnvVar{
						{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
						{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
						{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
					},
				},
			},
			mutated: true,
		},
		{
			name: "mutate multiple container",
			fields: fields{
				k8sClient:  fake.NewSimpleClientset(),
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				containers: []corev1.Container{
					{
						Name:  "TestContainer1",
						Image: "test-image-1",
					},
					{
						Name:  "TestContainer2",
						Image: "test-image-2",
					},
				},
				roleArn: "arn:aws:iam::123456789012:role/testrole",
				ns:      "test-namespace",
			},
			wantedContainers: []corev1.Container{
				{
					Name:         "TestContainer1",
					Image:        "test-image-1",
					VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
					Env: []corev1.EnvVar{
						{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
						{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
						{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
					},
				},
				{
					Name:         "TestContainer2",
					Image:        "test-image-2",
					VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
					Env: []corev1.EnvVar{
						{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
						{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
						{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
					},
				},
			},
			mutated: true,
		},
		{
			name: "no containers to mutate",
			fields: fields{
				k8sClient:  fake.NewSimpleClientset(),
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
			},
			args: args{
				roleArn: "arn:aws:iam::123456789012:role/testrole",
				ns:      "test-namespace",
			},
			mutated: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &mutatingWebhook{
				k8sClient:  tt.fields.k8sClient,
				image:      tt.fields.image,
				pullPolicy: tt.fields.pullPolicy,
				volumeName: tt.fields.volumeName,
				volumePath: tt.fields.volumePath,
				tokenFile:  tt.fields.tokenFile,
			}
			got := mw.mutateContainers(tt.args.containers, tt.args.roleArn)
			if got != tt.mutated {
				t.Errorf("mutatingWebhook.mutateContainers() = %v, want %v", got, tt.mutated)
			}
			if !cmp.Equal(tt.args.containers, tt.wantedContainers) {
				t.Errorf("mutatingWebhook.mutateContainers() = diff %v", cmp.Diff(tt.args.containers, tt.wantedContainers))
			}
		})
	}
}

//nolint:funlen
func Test_mutatingWebhook_mutatePod(t *testing.T) {
	containerRestartPolicy := corev1.ContainerRestartPolicyAlways

	type fields struct {
		image      string
		pullPolicy string
		volumeName string
		volumePath string
		tokenFile  string
	}
	type args struct {
		pod                *corev1.Pod
		ns                 string
		serviceAccountName string
		annotations        map[string]string
		dryRun             bool
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantErr   bool
		wantedPod *corev1.Pod
	}{
		{
			name: "mutate pod",
			fields: fields{
				image:      "doitintl/gtoken:test",
				pullPolicy: "Always",
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "TestContainer",
								Image: "test-image",
							},
						},
						ServiceAccountName: "test-sa",
					},
				},
				ns:                 "test-namespace",
				serviceAccountName: "test-sa",
				annotations:        map[string]string{awsRoleArnKey: "arn:aws:iam::123456789012:role/testrole"},
			},
			wantedPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:    "generate-gcp-id-token",
							Image:   "doitintl/gtoken:test",
							Command: []string{"/gtoken", "--file=/test-volume-path/test-token", "--refresh=false"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(requestsCPU),
									corev1.ResourceMemory: resource.MustParse(requestsMemory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(limitsCPU),
									corev1.ResourceMemory: resource.MustParse(limitsMemory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "test-volume-name",
									MountPath: "/test-volume-path",
								},
							},
							ImagePullPolicy: "Always",
						},
						{
							Name:          "update-gcp-id-token",
							Image:         "doitintl/gtoken:test",
							RestartPolicy: &containerRestartPolicy,
							Command:       []string{"/gtoken", "--file=/test-volume-path/test-token", "--refresh=true"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(requestsCPU),
									corev1.ResourceMemory: resource.MustParse(requestsMemory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(limitsCPU),
									corev1.ResourceMemory: resource.MustParse(limitsMemory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "test-volume-name",
									MountPath: "/test-volume-path",
								},
							},
							ImagePullPolicy: "Always",
						},
					},
					Containers: []corev1.Container{
						{
							Name:         "TestContainer",
							Image:        "test-image",
							VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
							Env: []corev1.EnvVar{
								{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
								{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
								{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "test-volume-name",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									Medium: corev1.StorageMediumMemory,
								},
							},
						},
					},
					ServiceAccountName: "test-sa",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        tt.args.serviceAccountName,
					Namespace:   tt.args.ns,
					Annotations: tt.args.annotations,
				},
			}
			mw := &mutatingWebhook{
				k8sClient:  fake.NewSimpleClientset(sa),
				image:      tt.fields.image,
				pullPolicy: tt.fields.pullPolicy,
				volumeName: tt.fields.volumeName,
				volumePath: tt.fields.volumePath,
				tokenFile:  tt.fields.tokenFile,
			}
			if err := mw.mutatePod(context.TODO(), tt.args.pod, tt.args.ns, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("mutatingWebhook.mutatePod() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !cmp.Equal(tt.args.pod, tt.wantedPod) {
				t.Errorf("mutatingWebhook.mutateContainers() = diff %v", cmp.Diff(tt.args.pod, tt.wantedPod))
			}
		})
	}
}

func Test_mutatingWebhook_mutatePod_ECR(t *testing.T) {
	containerRestartPolicy := corev1.ContainerRestartPolicyAlways

	type fields struct {
		image      string
		pullPolicy string
		volumeName string
		volumePath string
		tokenFile  string
	}
	type args struct {
		pod                *corev1.Pod
		ns                 string
		serviceAccountName string
		annotations        map[string]string
		dryRun             bool
		ecrSecret          *corev1.Secret
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantErr   bool
		wantedPod *corev1.Pod
	}{
		{
			name: "pod with ECR images and valid ECR secret",
			fields: fields{
				image:      "doitintl/gtoken:test",
				pullPolicy: "Always",
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "TestContainer",
								Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
							},
						},
						ServiceAccountName: "test-sa",
					},
				},
				ns:                 "test-namespace",
				serviceAccountName: "test-sa",
				annotations:        map[string]string{awsRoleArnKey: "arn:aws:iam::123456789012:role/testrole"},
				ecrSecret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ecr-secret",
						Namespace: "test-namespace",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						".dockerconfigjson": []byte(`{"auths":{"123456789012.dkr.ecr.us-west-2.amazonaws.com":{"auth":"dXNlcjpwYXNz"}}}`),
					},
				},
			},
			wantedPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:    "generate-gcp-id-token",
							Image:   "doitintl/gtoken:test",
							Command: []string{"/gtoken", "--file=/test-volume-path/test-token", "--refresh=false"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(requestsCPU),
									corev1.ResourceMemory: resource.MustParse(requestsMemory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(limitsCPU),
									corev1.ResourceMemory: resource.MustParse(limitsMemory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "test-volume-name",
									MountPath: "/test-volume-path",
								},
							},
							ImagePullPolicy: "Always",
						},
						{
							Name:          "update-gcp-id-token",
							Image:         "doitintl/gtoken:test",
							RestartPolicy: &containerRestartPolicy,
							Command:       []string{"/gtoken", "--file=/test-volume-path/test-token", "--refresh=true"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(requestsCPU),
									corev1.ResourceMemory: resource.MustParse(requestsMemory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(limitsCPU),
									corev1.ResourceMemory: resource.MustParse(limitsMemory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "test-volume-name",
									MountPath: "/test-volume-path",
								},
							},
							ImagePullPolicy: "Always",
						},
					},
					Containers: []corev1.Container{
						{
							Name:         "TestContainer",
							Image:        "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
							VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
							Env: []corev1.EnvVar{
								{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
								{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
								{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
							},
						},
					},
					ServiceAccountName: "test-sa",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "ecr-secret"},
					},
					Volumes: []corev1.Volume{
						{
							Name: "test-volume-name",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									Medium: corev1.StorageMediumMemory,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pod with ECR images but no AWS role annotation",
			fields: fields{
				image:      "doitintl/gtoken:test",
				pullPolicy: "Always",
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "TestContainer",
								Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
							},
						},
						ServiceAccountName: "test-sa",
					},
				},
				ns:                 "test-namespace",
				serviceAccountName: "test-sa",
				annotations:        map[string]string{}, // No AWS role annotation
				ecrSecret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ecr-secret",
						Namespace: "test-namespace",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						".dockerconfigjson": []byte(`{"auths":{"123456789012.dkr.ecr.us-west-2.amazonaws.com":{"auth":"dXNlcjpwYXNz"}}}`),
					},
				},
			},
			wantedPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "TestContainer",
							Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
						},
					},
					ServiceAccountName: "test-sa",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "ecr-secret"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pod with ECR images but missing ECR secret",
			fields: fields{
				image:      "doitintl/gtoken:test",
				pullPolicy: "Always",
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "TestContainer",
								Image: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:latest",
							},
						},
						ServiceAccountName: "test-sa",
					},
				},
				ns:                 "test-namespace",
				serviceAccountName: "test-sa",
				annotations:        map[string]string{}, // No AWS role annotation
				ecrSecret:          nil,                 // No ECR secret
			},
			wantErr: true, // Should fail because ECR images need valid secret
		},
		{
			name: "pod without ECR images (should not be affected)",
			fields: fields{
				image:      "doitintl/gtoken:test",
				pullPolicy: "Always",
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "TestContainer",
								Image: "gcr.io/my-project/my-app:latest",
							},
						},
						ServiceAccountName: "test-sa",
					},
				},
				ns:                 "test-namespace",
				serviceAccountName: "test-sa",
				annotations:        map[string]string{}, // No AWS role annotation
				ecrSecret:          nil,                 // No ECR secret
			},
			wantedPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "TestContainer",
							Image: "gcr.io/my-project/my-app:latest",
						},
					},
					ServiceAccountName: "test-sa",
				},
			},
			wantErr: false, // Should not fail because no ECR images
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with service account and optional ECR secret
			client := fake.NewSimpleClientset()

			// Create service account with annotations
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        tt.args.serviceAccountName,
					Namespace:   tt.args.ns,
					Annotations: tt.args.annotations,
				},
			}
			_, err := client.CoreV1().ServiceAccounts(tt.args.ns).Create(context.Background(), sa, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("failed to create service account: %v", err)
			}

			// Create ECR secret if provided
			if tt.args.ecrSecret != nil {
				_, err := client.CoreV1().Secrets(tt.args.ns).Create(context.Background(), tt.args.ecrSecret, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create ECR secret: %v", err)
				}
			}

			mw := &mutatingWebhook{
				k8sClient:  client,
				image:      tt.fields.image,
				pullPolicy: tt.fields.pullPolicy,
				volumeName: tt.fields.volumeName,
				volumePath: tt.fields.volumePath,
				tokenFile:  tt.fields.tokenFile,
			}

			err = mw.mutatePod(context.Background(), tt.args.pod, tt.args.ns, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("mutatePod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.wantedPod != nil {
				if diff := cmp.Diff(tt.wantedPod, tt.args.pod); diff != "" {
					t.Errorf("mutatePod() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func Test_mutatingWebhook_podMutator_ECR_AdmissionDenial(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		namespace      string
		ecrManager     ECRManager
		k8sClient      kubernetes.Interface
		expectedDenial bool
		expectedError  string
	}{
		{
			name: "ECR validation failure should deny admission",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
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
			ecrManager:     &mockECRManager{},
			k8sClient:      fake.NewSimpleClientset(), // No ECR secret exists
			expectedDenial: true,
			expectedError:  "ECR validation failed",
		},
		{
			name: "ECR secret copy failure should deny admission",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
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
			ecrManager: &mockECRManager{
				copySecretErr: fmt.Errorf("failed to copy secret"),
			},
			k8sClient:      fake.NewSimpleClientset(),
			expectedDenial: true,
			expectedError:  "ECR validation failed",
		},
		{
			name: "Non-ECR pod should not be denied",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "default",
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "gcr.io/my-project/my-app:latest",
						},
					},
				},
			},
			namespace:      "test-ns",
			ecrManager:     &mockECRManager{},
			k8sClient:      createFakeClientWithServiceAccount(),
			expectedDenial: false,
			expectedError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := &mutatingWebhook{
				k8sClient:  tt.k8sClient,
				ecrManager: tt.ecrManager,
				image:      "test-image",
				pullPolicy: "IfNotPresent",
				volumeName: "test-volume",
				volumePath: "/test/path",
				tokenFile:  "test-token",
			}

			ar := &whmodel.AdmissionReview{
				Namespace: tt.namespace,
			}

			result, err := webhook.podMutator(context.Background(), ar, tt.pod)

			if tt.expectedDenial {
				// Should return error (admission denied)
				if err == nil {
					t.Errorf("Expected admission denial, but got no error")
					return
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, err.Error())
				}
				if result != nil {
					t.Errorf("Expected nil result when admission is denied, got %v", result)
				}
			} else {
				// Should not return error (admission allowed)
				if err != nil {
					t.Errorf("Expected no admission denial, but got error: %v", err)
					return
				}
				if result == nil {
					t.Errorf("Expected non-nil result when admission is allowed")
				}
			}
		})
	}
}
