// Package dashboard provides Dashboard resource builders for the BanhBaoRing operator.
package dashboard

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
	DashboardImage = "banhbaoring/dashboard"
	DashboardPort  = 3000
)

// Deployment builds the Dashboard Deployment.
func Deployment(cluster *banhbaoringv1.BanhBaoRingCluster) *appsv1.Deployment {
	spec := cluster.Spec.Dashboard
	name := fmt.Sprintf("%s-dashboard", cluster.Name)

	version := spec.Version
	if version == "" {
		version = constants.DefaultDashboardVersion
	}

	// Use custom image if specified, otherwise use default
	image := spec.Image
	if image == "" {
		image = DashboardImage
	}

	labels := constants.Labels(cluster.Name, constants.ComponentDashboard, version)

	replicas := spec.Replicas
	if replicas == 0 {
		replicas = int32(constants.DefaultDashboardReplicas)
	}

	apiURL := fmt.Sprintf("http://%s-api:%d", cluster.Name, constants.PortAPI)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: constants.SelectorLabels(cluster.Name, constants.ComponentDashboard),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "dashboard",
							Image: fmt.Sprintf("%s:%s", image, version),
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: DashboardPort, Protocol: corev1.ProtocolTCP},
							},
							Env: []corev1.EnvVar{
								{Name: "API_URL", Value: apiURL},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(DashboardPort),
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
										Port:   intstr.FromInt(DashboardPort),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
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

// mergeResources returns resource requirements with defaults.
func mergeResources(override corev1.ResourceRequirements) corev1.ResourceRequirements {
	defaults := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}

	if override.Requests != nil || override.Limits != nil {
		return override
	}
	return defaults
}
