// Package api provides API resource builders for the BanhBaoRing operator.
package api

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
	"github.com/Bidon15/banhbaoring/operator/internal/constants"
)

const (
	APIImage = "banhbaoring/control-plane"
	APIPort  = 8080
)

// Deployment builds the API Deployment.
func Deployment(cluster *banhbaoringv1.BanhBaoRingCluster) *appsv1.Deployment {
	spec := cluster.Spec.API
	name := fmt.Sprintf("%s-api", cluster.Name)

	version := spec.Version
	if version == "" {
		version = constants.DefaultAPIVersion
	}

	// Use custom image if specified, otherwise use default
	image := spec.Image
	if image == "" {
		image = APIImage
	}

	labels := constants.Labels(cluster.Name, constants.ComponentAPI, version)

	replicas := spec.Replicas
	if replicas == 0 {
		replicas = int32(constants.DefaultAPIReplicas)
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
				MatchLabels: constants.SelectorLabels(cluster.Name, constants.ComponentAPI),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "api",
							Image: fmt.Sprintf("%s:%s", image, version),
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: APIPort, Protocol: corev1.ProtocolTCP},
							},
							Env: buildEnv(cluster),
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(APIPort),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(APIPort),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
							},
							Resources: mergeResources(spec.Resources),
						},
					},
				},
			},
		},
	}
}

// buildEnv creates environment variables for the API container.
// Uses BANHBAO_ prefix as expected by the control-plane's viper config.
func buildEnv(cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
	dbSecret := fmt.Sprintf("%s-postgres-credentials", cluster.Name)
	openbaoSvc := fmt.Sprintf("%s-openbao-active", cluster.Name)
	postgresSvc := fmt.Sprintf("%s-postgres", cluster.Name)
	redisSvc := fmt.Sprintf("%s-redis", cluster.Name)

	return []corev1.EnvVar{
		// Database config (BANHBAO_ prefix for viper)
		{Name: "BANHBAO_DATABASE_HOST", Value: postgresSvc},
		{Name: "BANHBAO_DATABASE_PORT", Value: "5432"},
		{Name: "BANHBAO_DATABASE_USER", ValueFrom: secretRef(dbSecret, "username")},
		{Name: "BANHBAO_DATABASE_PASSWORD", ValueFrom: secretRef(dbSecret, "password")},
		{Name: "BANHBAO_DATABASE_DATABASE", ValueFrom: secretRef(dbSecret, "database")},
		{Name: "BANHBAO_DATABASE_SSL_MODE", Value: "disable"},
		// Redis config
		{Name: "BANHBAO_REDIS_HOST", Value: redisSvc},
		{Name: "BANHBAO_REDIS_PORT", Value: "6379"},
		// OpenBao config
		{Name: "BANHBAO_OPENBAO_ADDRESS", Value: fmt.Sprintf("https://%s:8200", openbaoSvc)},
		{Name: "BANHBAO_OPENBAO_TOKEN", ValueFrom: secretRef(cluster.Name+"-openbao-root", "token")},
		// OAuth config (from oauth-credentials secret)
		{Name: "BANHBAO_AUTH_OAUTH_GITHUB_ID", ValueFrom: secretRefOptional("oauth-credentials", "github-client-id")},
		{Name: "BANHBAO_AUTH_OAUTH_GITHUB_SECRET", ValueFrom: secretRefOptional("oauth-credentials", "github-client-secret")},
		{Name: "BANHBAO_AUTH_OAUTH_GOOGLE_ID", ValueFrom: secretRefOptional("oauth-credentials", "google-client-id")},
		{Name: "BANHBAO_AUTH_OAUTH_GOOGLE_SECRET", ValueFrom: secretRefOptional("oauth-credentials", "google-client-secret")},
		{Name: "BANHBAO_AUTH_OAUTH_CALLBACK_URL", Value: fmt.Sprintf("https://%s", cluster.Spec.Domain)},
	}
}

// secretRef creates a secret key reference for environment variables.
func secretRef(name, key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			Key:                  key,
		},
	}
}

// secretRefOptional creates an optional secret key reference for environment variables.
func secretRefOptional(name, key string) *corev1.EnvVarSource {
	optional := true
	return &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			Key:                  key,
			Optional:             &optional,
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
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}

	if override.Requests != nil || override.Limits != nil {
		return override
	}
	return defaults
}
