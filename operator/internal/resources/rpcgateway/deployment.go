// Package rpcgateway provides resource builders for the JSON-RPC gateway.
package rpcgateway

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	popsignerv1 "github.com/Bidon15/popsigner/operator/api/v1"
	"github.com/Bidon15/popsigner/operator/internal/constants"
)

const (
	// RPCGatewayImage is the default RPC gateway image (Scaleway registry)
	RPCGatewayImage = "rg.nl-ams.scw.cloud/banhbao/rpc-gateway"
)

// Deployment builds the RPC Gateway Deployment.
func Deployment(cluster *popsignerv1.POPSignerCluster) *appsv1.Deployment {
	spec := cluster.Spec.RPCGateway
	name := fmt.Sprintf("%s-%s", cluster.Name, constants.ComponentRPCGateway)

	version := spec.Version
	if version == "" {
		version = constants.DefaultRPCGatewayVersion
	}

	image := spec.Image
	if image == "" {
		image = RPCGatewayImage
	}

	labels := constants.Labels(cluster.Name, constants.ComponentRPCGateway, version)

	replicas := spec.Replicas
	if replicas == 0 {
		replicas = int32(constants.DefaultRPCGatewayReplicas)
	}

	// Build volumes and volume mounts
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// Add mTLS CA volume if enabled
	if cluster.Spec.RPCGateway.MTLS.Enabled {
		volumes = append(volumes, GetCASecretVolume(cluster))
		volumeMounts = append(volumeMounts, GetCASecretVolumeMount())
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: constants.SelectorLabels(cluster.Name, constants.ComponentRPCGateway),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   fmt.Sprintf("%d", constants.PortRPCGateway),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Volumes: volumes,
					Containers: []corev1.Container{
						{
							Name:  "rpc-gateway",
							Image: fmt.Sprintf("%s:%s", image, version),
							Ports: []corev1.ContainerPort{
								{
									Name:          "jsonrpc",
									ContainerPort: int32(constants.PortRPCGateway),
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env:          buildEnv(cluster),
							VolumeMounts: volumeMounts,
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(constants.PortRPCGateway),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(constants.PortRPCGateway),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
								TimeoutSeconds:      5,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							Resources: mergeResources(spec.Resources),
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             boolPtr(true),
								RunAsUser:                int64Ptr(1000),
								ReadOnlyRootFilesystem:   boolPtr(true),
								AllowPrivilegeEscalation: boolPtr(false),
							},
						},
					},
					ServiceAccountName: fmt.Sprintf("%s-api", cluster.Name),
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: int64Ptr(1000),
					},
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: intstrPtr(0),
					MaxSurge:       intstrPtr(1),
				},
			},
		},
	}
}

// buildEnv creates environment variables for the RPC gateway.
func buildEnv(cluster *popsignerv1.POPSignerCluster) []corev1.EnvVar {
	dbSecret := fmt.Sprintf("%s-postgres-credentials", cluster.Name)
	openbaoSvc := fmt.Sprintf("%s-openbao-active", cluster.Name)
	postgresSvc := fmt.Sprintf("%s-postgres", cluster.Name)
	redisSvc := fmt.Sprintf("%s-redis", cluster.Name)

	envVars := []corev1.EnvVar{
		// Database config
		{Name: "POPSIGNER_DATABASE_HOST", Value: postgresSvc},
		{Name: "POPSIGNER_DATABASE_PORT", Value: "5432"},
		{Name: "POPSIGNER_DATABASE_USER", ValueFrom: secretRef(dbSecret, "username")},
		{Name: "POPSIGNER_DATABASE_PASSWORD", ValueFrom: secretRef(dbSecret, "password")},
		{Name: "POPSIGNER_DATABASE_DATABASE", ValueFrom: secretRef(dbSecret, "database")},
		{Name: "POPSIGNER_DATABASE_SSL_MODE", Value: "disable"},
		// Redis config
		{Name: "POPSIGNER_REDIS_HOST", Value: redisSvc},
		{Name: "POPSIGNER_REDIS_PORT", Value: "6379"},
		// OpenBao config
		{Name: "POPSIGNER_OPENBAO_ADDRESS", Value: fmt.Sprintf("https://%s:8200", openbaoSvc)},
		{Name: "POPSIGNER_OPENBAO_TOKEN", ValueFrom: secretRef(cluster.Name+"-openbao-root", "token")},
		// RPC Gateway specific
		{Name: "POPSIGNER_RPC_GATEWAY_PORT", Value: fmt.Sprintf("%d", constants.PortRPCGateway)},
	}

	// Rate limiting config
	if cluster.Spec.RPCGateway.RateLimit.RequestsPerSecond > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "POPSIGNER_RPC_RATE_LIMIT_RPS",
			Value: fmt.Sprintf("%d", cluster.Spec.RPCGateway.RateLimit.RequestsPerSecond),
		})
	}
	if cluster.Spec.RPCGateway.RateLimit.BurstSize > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "POPSIGNER_RPC_RATE_LIMIT_BURST",
			Value: fmt.Sprintf("%d", cluster.Spec.RPCGateway.RateLimit.BurstSize),
		})
	}

	// Add mTLS environment variables
	envVars = append(envVars, GetMTLSEnvVars(cluster)...)

	return envVars
}

// secretRef creates a secret key reference.
func secretRef(name, key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			Key:                  key,
		},
	}
}

// mergeResources returns resource requirements with defaults.
func mergeResources(override corev1.ResourceRequirements) corev1.ResourceRequirements {
	defaults := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}

	if override.Requests != nil || override.Limits != nil {
		return override
	}
	return defaults
}

// Helper functions
func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }
func intstrPtr(i int) *intstr.IntOrString {
	v := intstr.FromInt(i)
	return &v
}

