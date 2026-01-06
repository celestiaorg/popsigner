package rpcgateway

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	popsignerv1 "github.com/Bidon15/popsigner/operator/api/v1"
	"github.com/Bidon15/popsigner/operator/internal/constants"
)

func TestDeployment(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled:  true,
				Version:  "1.0.0",
				Replicas: 3,
			},
		},
	}

	deployment := Deployment(cluster)

	// Verify name
	expectedName := "test-cluster-rpc-gateway"
	if deployment.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, deployment.Name)
	}

	// Verify namespace
	if deployment.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", deployment.Namespace)
	}

	// Verify labels
	if deployment.Labels[constants.LabelComponent] != constants.ComponentRPCGateway {
		t.Errorf("expected component label %q, got %q", constants.ComponentRPCGateway, deployment.Labels[constants.LabelComponent])
	}

	// Verify replicas
	if *deployment.Spec.Replicas != 3 {
		t.Errorf("expected 3 replicas, got %d", *deployment.Spec.Replicas)
	}

	// Verify container image
	expectedImage := "rg.nl-ams.scw.cloud/banhbao/rpc-gateway:1.0.0"
	if deployment.Spec.Template.Spec.Containers[0].Image != expectedImage {
		t.Errorf("expected image %q, got %q", expectedImage, deployment.Spec.Template.Spec.Containers[0].Image)
	}

	// Verify container port
	if deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != int32(constants.PortRPCGateway) {
		t.Errorf("expected port %d, got %d", constants.PortRPCGateway, deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	}

	// Verify port name
	if deployment.Spec.Template.Spec.Containers[0].Ports[0].Name != "jsonrpc" {
		t.Errorf("expected port name 'jsonrpc', got %q", deployment.Spec.Template.Spec.Containers[0].Ports[0].Name)
	}

	// Verify readiness probe
	probe := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe
	if probe == nil {
		t.Fatal("expected readiness probe to be set")
	}
	if probe.HTTPGet.Path != "/health" {
		t.Errorf("expected readiness probe path '/health', got %q", probe.HTTPGet.Path)
	}

	// Verify liveness probe
	livenessProbe := deployment.Spec.Template.Spec.Containers[0].LivenessProbe
	if livenessProbe == nil {
		t.Fatal("expected liveness probe to be set")
	}
	if livenessProbe.HTTPGet.Path != "/health" {
		t.Errorf("expected liveness probe path '/health', got %q", livenessProbe.HTTPGet.Path)
	}

	// Verify prometheus annotations
	if deployment.Spec.Template.Annotations["prometheus.io/scrape"] != "true" {
		t.Error("expected prometheus scrape annotation to be 'true'")
	}

	// Verify security context
	secCtx := deployment.Spec.Template.Spec.Containers[0].SecurityContext
	if secCtx == nil {
		t.Fatal("expected security context to be set")
	}
	if *secCtx.RunAsNonRoot != true {
		t.Error("expected RunAsNonRoot to be true")
	}
	if *secCtx.ReadOnlyRootFilesystem != true {
		t.Error("expected ReadOnlyRootFilesystem to be true")
	}
}

func TestDeploymentDefaultVersion(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
				// Version not set
			},
		},
	}

	deployment := Deployment(cluster)

	// Verify default version is used
	expectedImage := "rg.nl-ams.scw.cloud/banhbao/rpc-gateway:" + constants.DefaultRPCGatewayVersion
	if deployment.Spec.Template.Spec.Containers[0].Image != expectedImage {
		t.Errorf("expected image %q, got %q", expectedImage, deployment.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestDeploymentDefaultReplicas(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
				Version: "1.0.0",
				// Replicas not set (0)
			},
		},
	}

	deployment := Deployment(cluster)

	// Verify default replicas is used
	if *deployment.Spec.Replicas != int32(constants.DefaultRPCGatewayReplicas) {
		t.Errorf("expected %d replicas, got %d", constants.DefaultRPCGatewayReplicas, *deployment.Spec.Replicas)
	}
}

func TestDeploymentCustomImage(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
				Image:   "ghcr.io/myorg/custom-rpc-gateway",
				Version: "v2.0.0",
			},
		},
	}

	deployment := Deployment(cluster)

	expectedImage := "ghcr.io/myorg/custom-rpc-gateway:v2.0.0"
	if deployment.Spec.Template.Spec.Containers[0].Image != expectedImage {
		t.Errorf("expected image %q, got %q", expectedImage, deployment.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestDeploymentWithRateLimit(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
				RateLimit: popsignerv1.RateLimitConfig{
					RequestsPerSecond: 50,
					BurstSize:         100,
				},
			},
		},
	}

	deployment := Deployment(cluster)

	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	hasRPS := false
	hasBurst := false
	for _, env := range envVars {
		if env.Name == "POPSIGNER_RPC_RATE_LIMIT_RPS" && env.Value == "50" {
			hasRPS = true
		}
		if env.Name == "POPSIGNER_RPC_RATE_LIMIT_BURST" && env.Value == "100" {
			hasBurst = true
		}
	}
	if !hasRPS {
		t.Error("expected POPSIGNER_RPC_RATE_LIMIT_RPS env var to be present with value '50'")
	}
	if !hasBurst {
		t.Error("expected POPSIGNER_RPC_RATE_LIMIT_BURST env var to be present with value '100'")
	}
}

func TestDeploymentCustomResources(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2000m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
		},
	}

	deployment := Deployment(cluster)

	resources := deployment.Spec.Template.Spec.Containers[0].Resources

	cpuRequest := resources.Requests[corev1.ResourceCPU]
	if cpuRequest.String() != "500m" {
		t.Errorf("expected CPU request '500m', got %q", cpuRequest.String())
	}

	memLimit := resources.Limits[corev1.ResourceMemory]
	if memLimit.String() != "1Gi" {
		t.Errorf("expected memory limit '1Gi', got %q", memLimit.String())
	}
}

func TestBuildEnv(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
			},
		},
	}

	env := buildEnv(cluster)

	// Verify we have all expected env vars
	envMap := make(map[string]bool)
	for _, e := range env {
		envMap[e.Name] = true
	}

	expectedEnvs := []string{
		"POPSIGNER_DATABASE_HOST",
		"POPSIGNER_DATABASE_PORT",
		"POPSIGNER_DATABASE_USER",
		"POPSIGNER_DATABASE_PASSWORD",
		"POPSIGNER_DATABASE_DATABASE",
		"POPSIGNER_DATABASE_SSL_MODE",
		"POPSIGNER_REDIS_HOST",
		"POPSIGNER_REDIS_PORT",
		"POPSIGNER_OPENBAO_ADDRESS",
		"POPSIGNER_OPENBAO_TOKEN",
		"POPSIGNER_RPC_GATEWAY_PORT",
	}
	for _, expected := range expectedEnvs {
		if !envMap[expected] {
			t.Errorf("expected env var %q to be present", expected)
		}
	}

	// Verify OPENBAO_ADDRESS is a direct value (not secret ref)
	for _, e := range env {
		if e.Name == "POPSIGNER_OPENBAO_ADDRESS" {
			expectedAddr := "https://test-cluster-openbao-active:8200"
			if e.Value != expectedAddr {
				t.Errorf("expected POPSIGNER_OPENBAO_ADDRESS %q, got %q", expectedAddr, e.Value)
			}
		}
	}
}

func TestSecretRef(t *testing.T) {
	ref := secretRef("my-secret", "my-key")

	if ref.SecretKeyRef.Name != "my-secret" {
		t.Errorf("expected secret name 'my-secret', got %q", ref.SecretKeyRef.Name)
	}
	if ref.SecretKeyRef.Key != "my-key" {
		t.Errorf("expected key 'my-key', got %q", ref.SecretKeyRef.Key)
	}
}

func TestDeploymentWithMTLS(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
				MTLS: popsignerv1.MTLSConfig{
					Enabled: true,
				},
			},
		},
	}

	deployment := Deployment(cluster)

	// Verify volumes
	if len(deployment.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(deployment.Spec.Template.Spec.Volumes))
	}
	if deployment.Spec.Template.Spec.Volumes[0].Name != CAVolumeName {
		t.Errorf("expected volume name %q, got %q", CAVolumeName, deployment.Spec.Template.Spec.Volumes[0].Name)
	}

	// Verify volume mounts
	container := deployment.Spec.Template.Spec.Containers[0]
	if len(container.VolumeMounts) != 1 {
		t.Errorf("expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != CAVolumeName {
		t.Errorf("expected volume mount name %q, got %q", CAVolumeName, container.VolumeMounts[0].Name)
	}
	if container.VolumeMounts[0].MountPath != CAMountPath {
		t.Errorf("expected mount path %q, got %q", CAMountPath, container.VolumeMounts[0].MountPath)
	}
	if !container.VolumeMounts[0].ReadOnly {
		t.Error("expected volume mount to be read-only")
	}

	// Verify mTLS env vars
	envVars := container.Env
	hasMTLSEnabled := false
	hasMTLSCACertPath := false
	hasMTLSClientAuthType := false
	for _, env := range envVars {
		switch env.Name {
		case "MTLS_ENABLED":
			hasMTLSEnabled = true
			if env.Value != "true" {
				t.Errorf("expected MTLS_ENABLED to be 'true', got %q", env.Value)
			}
		case "MTLS_CA_CERT_PATH":
			hasMTLSCACertPath = true
			if env.Value != "/etc/popsigner/ca/ca.crt" {
				t.Errorf("expected MTLS_CA_CERT_PATH to be '/etc/popsigner/ca/ca.crt', got %q", env.Value)
			}
		case "MTLS_CLIENT_AUTH_TYPE":
			hasMTLSClientAuthType = true
			if env.Value != DefaultClientAuthType {
				t.Errorf("expected MTLS_CLIENT_AUTH_TYPE to be %q, got %q", DefaultClientAuthType, env.Value)
			}
		}
	}
	if !hasMTLSEnabled {
		t.Error("expected MTLS_ENABLED env var to be present")
	}
	if !hasMTLSCACertPath {
		t.Error("expected MTLS_CA_CERT_PATH env var to be present")
	}
	if !hasMTLSClientAuthType {
		t.Error("expected MTLS_CLIENT_AUTH_TYPE env var to be present")
	}
}

func TestDeploymentWithMTLSDisabled(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
				MTLS: popsignerv1.MTLSConfig{
					Enabled: false,
				},
			},
		},
	}

	deployment := Deployment(cluster)

	// Verify no CA volume when mTLS disabled
	if len(deployment.Spec.Template.Spec.Volumes) != 0 {
		t.Errorf("expected 0 volumes when mTLS disabled, got %d", len(deployment.Spec.Template.Spec.Volumes))
	}

	// Verify no volume mounts
	container := deployment.Spec.Template.Spec.Containers[0]
	if len(container.VolumeMounts) != 0 {
		t.Errorf("expected 0 volume mounts when mTLS disabled, got %d", len(container.VolumeMounts))
	}

	// Verify no mTLS env vars
	for _, env := range container.Env {
		if env.Name == "MTLS_ENABLED" || env.Name == "MTLS_CA_CERT_PATH" || env.Name == "MTLS_CLIENT_AUTH_TYPE" {
			t.Errorf("did not expect mTLS env var %q when mTLS disabled", env.Name)
		}
	}
}

func TestDeploymentWithCustomMTLSConfig(t *testing.T) {
	cluster := &popsignerv1.POPSignerCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: popsignerv1.POPSignerClusterSpec{
			Domain: "keys.example.com",
			RPCGateway: popsignerv1.RPCGatewaySpec{
				Enabled: true,
				MTLS: popsignerv1.MTLSConfig{
					Enabled:        true,
					CASecretName:   "custom-ca-secret",
					CASecretKey:    "custom.crt",
					ClientAuthType: "RequireAndVerifyClientCert",
				},
			},
		},
	}

	deployment := Deployment(cluster)

	// Verify custom secret name in volume
	volume := deployment.Spec.Template.Spec.Volumes[0]
	if volume.Secret.SecretName != "custom-ca-secret" {
		t.Errorf("expected secret name 'custom-ca-secret', got %q", volume.Secret.SecretName)
	}
	if volume.Secret.Items[0].Key != "custom.crt" {
		t.Errorf("expected secret key 'custom.crt', got %q", volume.Secret.Items[0].Key)
	}

	// Verify custom client auth type in env vars
	container := deployment.Spec.Template.Spec.Containers[0]
	for _, env := range container.Env {
		if env.Name == "MTLS_CLIENT_AUTH_TYPE" {
			if env.Value != "RequireAndVerifyClientCert" {
				t.Errorf("expected MTLS_CLIENT_AUTH_TYPE to be 'RequireAndVerifyClientCert', got %q", env.Value)
			}
		}
	}
}

