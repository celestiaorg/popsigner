// Package openbao provides Kubernetes resource builders for OpenBao.
package openbao

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
	"github.com/Bidon15/banhbaoring/operator/internal/constants"
	"github.com/Bidon15/banhbaoring/operator/internal/resources"
)

const (
	// OpenBaoImage is the default OpenBao container image.
	OpenBaoImage = "openbao/openbao"
	// OpenBaoPort is the API port.
	OpenBaoPort = 8200
	// OpenBaoClusterPort is the cluster communication port.
	OpenBaoClusterPort = 8201
	// PluginDir is the directory where plugins are stored.
	PluginDir = "/vault/plugins"
	// DataDir is the directory where Raft data is stored.
	DataDir = "/vault/data"
	// ConfigDir is the directory where configuration is mounted.
	ConfigDir = "/vault/config"
	// TLSDir is the directory where TLS certificates are mounted.
	TLSDir = "/vault/tls"
)

// StatefulSet builds the OpenBao StatefulSet.
func StatefulSet(cluster *banhbaoringv1.BanhBaoRingCluster) *appsv1.StatefulSet {
	spec := cluster.Spec.OpenBao
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)
	labels := resources.Labels(cluster.Name, constants.ComponentOpenBao, spec.Version)
	selectorLabels := resources.SelectorLabels(cluster.Name, constants.ComponentOpenBao)

	replicas := spec.Replicas
	if replicas == 0 {
		replicas = int32(constants.DefaultOpenBaoReplicas)
	}

	version := spec.Version
	if version == "" {
		version = constants.DefaultOpenBaoVersion
	}

	return &appsv1.StatefulSet{
		ObjectMeta: resources.ObjectMeta(name, cluster.Namespace, labels),
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: buildPodSpec(cluster, name, version),
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				buildDataPVC(spec),
			},
		},
	}
}

func buildPodSpec(cluster *banhbaoringv1.BanhBaoRingCluster, name, version string) corev1.PodSpec {
	_ = cluster.Spec.OpenBao // Reserved for future use

	return corev1.PodSpec{
		ServiceAccountName: name,
		SecurityContext: &corev1.PodSecurityContext{
			FSGroup: int64Ptr(1000),
		},
		Containers: []corev1.Container{
			buildOpenBaoContainer(cluster, version),
		},
		Volumes: buildVolumes(name),
	}
}

func buildOpenBaoContainer(cluster *banhbaoringv1.BanhBaoRingCluster, version string) corev1.Container {
	spec := cluster.Spec.OpenBao

	resourceReqs := resources.MergeResourceRequirements(
		resources.DefaultResourceRequirements(),
		spec.Resources,
	)

	return corev1.Container{
		Name:    "openbao",
		Image:   fmt.Sprintf("%s:%s", OpenBaoImage, version),
		Command: []string{"bao", "server", "-config=" + ConfigDir + "/config.hcl"},
		Ports: []corev1.ContainerPort{
			resources.ContainerPort("api", OpenBaoPort),
			resources.ContainerPort("cluster", OpenBaoClusterPort),
		},
		Env: buildEnv(cluster),
		VolumeMounts: []corev1.VolumeMount{
			resources.VolumeMount("data", DataDir, false),
			resources.VolumeMount("config", ConfigDir, true),
			resources.VolumeMount("plugins", PluginDir, false),
			resources.VolumeMount("tls", TLSDir, true),
		},
		Resources: resourceReqs,
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/v1/sys/health?standbyok=true",
					Port:   intstr.FromInt(OpenBaoPort),
					Scheme: corev1.URISchemeHTTPS,
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/v1/sys/health?standbyok=true",
					Port:   intstr.FromInt(OpenBaoPort),
					Scheme: corev1.URISchemeHTTPS,
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       30,
		},
		// IPC_LOCK capability is no longer needed as OpenBao dropped mlock support
	}
}

func buildEnv(cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)

	env := []corev1.EnvVar{
		resources.EnvVar("VAULT_ADDR", "https://127.0.0.1:8200"),
		resources.EnvVar("VAULT_CLUSTER_ADDR", fmt.Sprintf("https://$(HOSTNAME).%s:8201", name)),
		resources.EnvVar("VAULT_API_ADDR", fmt.Sprintf("https://$(HOSTNAME).%s:8200", name)),
		resources.EnvVar("VAULT_SKIP_VERIFY", "true"),
		{
			Name: "HOSTNAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
	}

	// Add auto-unseal env vars
	if cluster.Spec.OpenBao.AutoUnseal.Enabled {
		env = append(env, autoUnsealEnv(cluster)...)
	}

	return env
}

func autoUnsealEnv(cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
	unseal := cluster.Spec.OpenBao.AutoUnseal
	var env []corev1.EnvVar

	switch unseal.Provider {
	case "awskms":
		if unseal.AWSKMS != nil {
			env = append(env, resources.EnvVar("AWS_REGION", unseal.AWSKMS.Region))
			if unseal.AWSKMS.Credentials != nil {
				env = append(env,
					resources.EnvVarFromSecret("AWS_ACCESS_KEY_ID", unseal.AWSKMS.Credentials.Name, "access-key-id"),
					resources.EnvVarFromSecret("AWS_SECRET_ACCESS_KEY", unseal.AWSKMS.Credentials.Name, "secret-access-key"),
				)
			}
		}
	case "gcpkms":
		if unseal.GCPKMS != nil {
			env = append(env, resources.EnvVar("GOOGLE_PROJECT", unseal.GCPKMS.Project))
			if unseal.GCPKMS.Credentials != nil {
				env = append(env, corev1.EnvVar{
					Name:  "GOOGLE_APPLICATION_CREDENTIALS",
					Value: "/vault/gcp/credentials.json",
				})
			}
		}
	case "azurekv":
		if unseal.AzureKV != nil {
			env = append(env, resources.EnvVar("AZURE_TENANT_ID", unseal.AzureKV.TenantID))
			if unseal.AzureKV.Credentials != nil {
				env = append(env,
					resources.EnvVarFromSecret("AZURE_CLIENT_ID", unseal.AzureKV.Credentials.Name, "client-id"),
					resources.EnvVarFromSecret("AZURE_CLIENT_SECRET", unseal.AzureKV.Credentials.Name, "client-secret"),
				)
			}
		}
	}

	return env
}

func buildVolumes(name string) []corev1.Volume {
	return []corev1.Volume{
		resources.ConfigMapVolume("config", name+"-config"),
		resources.EmptyDirVolume("plugins"),
		resources.SecretVolume("tls", name+"-tls"),
	}
}

func buildDataPVC(spec banhbaoringv1.OpenBaoSpec) corev1.PersistentVolumeClaim {
	size := spec.Storage.Size.String()
	if size == "" || size == "0" {
		size = constants.DefaultOpenBaoStorageSize
	}

	return resources.PersistentVolumeClaim("data", spec.Storage.StorageClass, size)
}

func int64Ptr(i int64) *int64 { return &i }
