package controllers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
	"github.com/Bidon15/banhbaoring/operator/internal/conditions"
	"github.com/Bidon15/banhbaoring/operator/internal/constants"
	"github.com/Bidon15/banhbaoring/operator/internal/resources"
	"github.com/Bidon15/banhbaoring/operator/internal/resources/openbao"
	"github.com/Bidon15/banhbaoring/operator/internal/unseal"
)

// reconcileOpenBao handles all OpenBao resources.
func (r *ClusterReconciler) reconcileOpenBao(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
	log := log.FromContext(ctx)
	log.Info("Reconciling OpenBao")

	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)

	// 0. Create ServiceAccount
	sa := openbao.ServiceAccount(cluster)
	if err := r.createOrUpdate(ctx, cluster, sa); err != nil {
		return fmt.Errorf("failed to reconcile serviceaccount: %w", err)
	}

	// 0.5 Create self-signed TLS secret (if not exists)
	tlsSecret, err := r.ensureTLSSecret(ctx, cluster, name)
	if err != nil {
		return fmt.Errorf("failed to ensure TLS secret: %w", err)
	}
	if tlsSecret != nil {
		if err := r.createOrUpdate(ctx, cluster, tlsSecret); err != nil {
			return fmt.Errorf("failed to reconcile TLS secret: %w", err)
		}
	}

	// 1. Create/update ConfigMap
	configMap, err := openbao.ConfigMap(cluster)
	if err != nil {
		return fmt.Errorf("failed to build configmap: %w", err)
	}
	if err := r.createOrUpdate(ctx, cluster, configMap); err != nil {
		return fmt.Errorf("failed to reconcile configmap: %w", err)
	}

	// 2. Create/update headless Service
	headlessSvc := openbao.HeadlessService(cluster)
	if err := r.createOrUpdate(ctx, cluster, headlessSvc); err != nil {
		return fmt.Errorf("failed to reconcile headless service: %w", err)
	}

	// 3. Create/update active Service
	activeSvc := openbao.ActiveService(cluster)
	if err := r.createOrUpdate(ctx, cluster, activeSvc); err != nil {
		return fmt.Errorf("failed to reconcile active service: %w", err)
	}

	// 4. Create/update internal Service
	internalSvc := openbao.InternalService(cluster)
	if err := r.createOrUpdate(ctx, cluster, internalSvc); err != nil {
		return fmt.Errorf("failed to reconcile internal service: %w", err)
	}

	// 5. Create/update StatefulSet
	sts := openbao.StatefulSet(cluster)

	// TODO: Add init container for plugin download once release artifacts are published
	// For now, we skip the plugin download init container since the GitHub release doesn't exist yet
	// The plugin can be registered manually after deployment
	// sts.Spec.Template.Spec.InitContainers = append(
	// 	sts.Spec.Template.Spec.InitContainers,
	// 	openbao.InitContainer(cluster),
	// )

	// Add unseal provider configuration if enabled
	if cluster.Spec.OpenBao.AutoUnseal.Enabled {
		if err := r.configureAutoUnseal(ctx, cluster, sts); err != nil {
			return fmt.Errorf("failed to configure auto-unseal: %w", err)
		}
	}

	if err := r.createOrUpdate(ctx, cluster, sts); err != nil {
		return fmt.Errorf("failed to reconcile statefulset: %w", err)
	}

	return nil
}

// configureAutoUnseal adds auto-unseal configuration to the StatefulSet.
func (r *ClusterReconciler) configureAutoUnseal(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster, sts *appsv1.StatefulSet) error {
	provider, err := unseal.GetProviderForCluster(cluster)
	if err != nil {
		return err
	}
	if provider == nil {
		return nil
	}

	// Validate the provider configuration
	if err := provider.Validate(&cluster.Spec.OpenBao.AutoUnseal); err != nil {
		return fmt.Errorf("invalid auto-unseal configuration: %w", err)
	}

	// Get additional environment variables
	envVars, err := provider.GetEnvVars(ctx, &cluster.Spec.OpenBao.AutoUnseal, cluster.Namespace)
	if err != nil {
		return fmt.Errorf("failed to get provider env vars: %w", err)
	}

	// Add env vars to the openbao container
	for i := range sts.Spec.Template.Spec.Containers {
		if sts.Spec.Template.Spec.Containers[i].Name == "openbao" {
			sts.Spec.Template.Spec.Containers[i].Env = append(
				sts.Spec.Template.Spec.Containers[i].Env,
				envVars...,
			)

			// Add volume mounts
			volumeMounts := provider.GetVolumeMounts(&cluster.Spec.OpenBao.AutoUnseal)
			sts.Spec.Template.Spec.Containers[i].VolumeMounts = append(
				sts.Spec.Template.Spec.Containers[i].VolumeMounts,
				volumeMounts...,
			)
			break
		}
	}

	// Add volumes
	volumes := provider.GetVolumes(&cluster.Spec.OpenBao.AutoUnseal)
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, volumes...)

	return nil
}

// isOpenBaoReady checks if OpenBao pods are ready.
func (r *ClusterReconciler) isOpenBaoReady(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) bool {
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)

	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err != nil {
		return false
	}

	expectedReplicas := cluster.Spec.OpenBao.Replicas
	if expectedReplicas == 0 {
		expectedReplicas = int32(constants.DefaultOpenBaoReplicas)
	}

	return sts.Status.ReadyReplicas >= expectedReplicas
}

// updateOpenBaoStatus updates the cluster status with OpenBao info.
func (r *ClusterReconciler) updateOpenBaoStatus(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)

	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err != nil {
		if errors.IsNotFound(err) {
			cluster.Status.OpenBao = banhbaoringv1.ComponentStatus{
				Ready:   false,
				Message: "StatefulSet not found",
			}
			return nil
		}
		return err
	}

	expectedReplicas := *sts.Spec.Replicas
	ready := sts.Status.ReadyReplicas >= expectedReplicas

	cluster.Status.OpenBao = banhbaoringv1.ComponentStatus{
		Ready:    ready,
		Version:  cluster.Spec.OpenBao.Version,
		Replicas: fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, expectedReplicas),
	}

	condStatus := metav1.ConditionFalse
	reason := conditions.ReasonNotReady
	message := fmt.Sprintf("Waiting for OpenBao pods: %d/%d ready", sts.Status.ReadyReplicas, expectedReplicas)

	if ready {
		condStatus = metav1.ConditionTrue
		reason = conditions.ReasonReady
		message = "OpenBao cluster is ready"
	}

	conditions.SetCondition(&cluster.Status.Conditions, conditions.TypeOpenBaoReady, condStatus, reason, message)

	return nil
}

// initializeOpenBao performs first-time initialization.
func (r *ClusterReconciler) initializeOpenBao(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
	log := log.FromContext(ctx)
	log.Info("Initializing OpenBao cluster")

	// This would typically be done via a Job that:
	// 1. Calls `vault operator init` on the first pod
	// 2. Stores the root token and unseal keys in a Secret
	// 3. Unseals all pods (if not using auto-unseal)
	// 4. Registers the secp256k1 plugin

	// For auto-unseal, the process is simpler as pods self-unseal

	// TODO: Implement actual initialization logic via Job
	// This requires creating a Job that runs the init script

	return nil
}

// registerPlugin registers the secp256k1 plugin.
func (r *ClusterReconciler) registerPlugin(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
	log := log.FromContext(ctx)
	log.Info("Registering secp256k1 plugin")

	// TODO: Create a Job that runs the plugin registration script
	// The Job needs access to the root token

	return nil
}

// createOrUpdate creates or updates a Kubernetes resource.
func (r *ClusterReconciler) createOrUpdate(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster, obj client.Object) error {
	// Set owner reference
	if err := ctrl.SetControllerReference(cluster, obj, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Try to create the resource
	if err := r.Create(ctx, obj); err != nil {
		if errors.IsAlreadyExists(err) {
			// Resource exists, update it
			existing := obj.DeepCopyObject().(client.Object)
			key := types.NamespacedName{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			}
			if err := r.Get(ctx, key, existing); err != nil {
				return fmt.Errorf("failed to get existing resource: %w", err)
			}

			// Copy resource version for update
			obj.SetResourceVersion(existing.GetResourceVersion())

			if err := r.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
	} else {
		return fmt.Errorf("failed to create resource: %w", err)
	}
}

	return nil
}

// ensureTLSSecret creates a self-signed TLS certificate for OpenBao if it doesn't exist.
func (r *ClusterReconciler) ensureTLSSecret(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster, name string) (*corev1.Secret, error) {
	secretName := name + "-tls"
	
	// Check if secret already exists
	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: cluster.Namespace}, existing)
	if err == nil {
		return nil, nil // Secret exists, no need to create
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	// Generate self-signed certificate
	certPEM, keyPEM, caPEM, err := generateSelfSignedCert(name, cluster.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate: %w", err)
	}

	labels := resources.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)
	
	return &corev1.Secret{
		ObjectMeta: resources.ObjectMeta(secretName, cluster.Namespace, labels),
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
			"ca.crt":  caPEM,
		},
	}, nil
}

// generateSelfSignedCert generates a self-signed CA and certificate for OpenBao.
func generateSelfSignedCert(name, namespace string) (certPEM, keyPEM, caPEM []byte, err error) {
	// Generate CA key
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create CA certificate
	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"BanhBaoRing"},
			CommonName:   "OpenBao CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, nil, err
	}

	// Generate server key
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create server certificate
	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"BanhBaoRing"},
			CommonName:   name,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(1, 0, 0), // 1 year
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		DNSNames: []string{
			name,
			name + "." + namespace,
			name + "." + namespace + ".svc",
			name + "." + namespace + ".svc.cluster.local",
			"*." + name,
			"*." + name + "." + namespace,
			"*." + name + "." + namespace + ".svc",
			"*." + name + "." + namespace + ".svc.cluster.local",
			"localhost",
		},
		IPAddresses: nil,
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// Encode to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})
	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	return certPEM, keyPEM, caPEM, nil
}
