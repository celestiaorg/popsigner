package openbao

import (
	"bytes"
	"fmt"
	"text/template"

	corev1 "k8s.io/api/core/v1"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
	"github.com/Bidon15/banhbaoring/operator/internal/constants"
	"github.com/Bidon15/banhbaoring/operator/internal/resources"
)

const configTemplate = `ui = true
plugin_directory = "{{ .PluginDir }}"

listener "tcp" {
  address         = "0.0.0.0:8200"
  cluster_address = "0.0.0.0:8201"
  tls_cert_file   = "/vault/tls/tls.crt"
  tls_key_file    = "/vault/tls/tls.key"
}

storage "raft" {
  path = "/vault/data"
  node_id = "${HOSTNAME}"
  {{ range $i := .PeerIndices }}
  retry_join {
    leader_api_addr = "https://{{ $.StatefulSetName }}-{{ $i }}.{{ $.StatefulSetName }}:8200"
    leader_ca_cert_file = "/vault/tls/ca.crt"
  }
  {{ end }}
}

api_addr     = "https://${HOSTNAME}.{{ .StatefulSetName }}:8200"
cluster_addr = "https://${HOSTNAME}.{{ .StatefulSetName }}:8201"

{{ if .AutoUnseal }}
{{ .SealConfig }}
{{ end }}

telemetry {
  prometheus_retention_time = "30s"
  disable_hostname = true
}
`

// ConfigData holds data for the configuration template.
type ConfigData struct {
	PluginDir       string
	StatefulSetName string
	PeerIndices     []int
	AutoUnseal      bool
	SealConfig      string
}

// ConfigMap builds the OpenBao configuration ConfigMap.
func ConfigMap(cluster *banhbaoringv1.BanhBaoRingCluster) (*corev1.ConfigMap, error) {
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)
	labels := resources.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)

	replicas := int(cluster.Spec.OpenBao.Replicas)
	if replicas == 0 {
		replicas = constants.DefaultOpenBaoReplicas
	}

	peerIndices := make([]int, replicas)
	for i := 0; i < replicas; i++ {
		peerIndices[i] = i
	}

	data := ConfigData{
		PluginDir:       PluginDir,
		StatefulSetName: name,
		PeerIndices:     peerIndices,
		AutoUnseal:      cluster.Spec.OpenBao.AutoUnseal.Enabled,
		SealConfig:      buildSealConfig(cluster),
	}

	tmpl, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute config template: %w", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: resources.ObjectMeta(name+"-config", cluster.Namespace, labels),
		Data: map[string]string{
			"config.hcl": buf.String(),
		},
	}, nil
}

func buildSealConfig(cluster *banhbaoringv1.BanhBaoRingCluster) string {
	unseal := cluster.Spec.OpenBao.AutoUnseal
	if !unseal.Enabled {
		return ""
	}

	switch unseal.Provider {
	case "awskms":
		if unseal.AWSKMS != nil {
			region := unseal.AWSKMS.Region
			if region == "" {
				region = "us-east-1"
			}
			return fmt.Sprintf(`seal "awskms" {
  region     = "%s"
  kms_key_id = "%s"
}`, region, unseal.AWSKMS.KeyID)
		}

	case "gcpkms":
		if unseal.GCPKMS != nil {
			return fmt.Sprintf(`seal "gcpckms" {
  project     = "%s"
  region      = "%s"
  key_ring    = "%s"
  crypto_key  = "%s"
}`, unseal.GCPKMS.Project, unseal.GCPKMS.Location, unseal.GCPKMS.KeyRing, unseal.GCPKMS.CryptoKey)
		}

	case "azurekv":
		if unseal.AzureKV != nil {
			return fmt.Sprintf(`seal "azurekeyvault" {
  tenant_id  = "%s"
  vault_name = "%s"
  key_name   = "%s"
}`, unseal.AzureKV.TenantID, unseal.AzureKV.VaultName, unseal.AzureKV.KeyName)
		}

	case "transit":
		if unseal.Transit != nil {
			mountPath := unseal.Transit.MountPath
			if mountPath == "" {
				mountPath = "transit"
			}
			return fmt.Sprintf(`seal "transit" {
  address    = "%s"
  mount_path = "%s"
  key_name   = "%s"
}`, unseal.Transit.Address, mountPath, unseal.Transit.KeyName)
		}
	}

	return ""
}
