package openbao

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
	"github.com/Bidon15/banhbaoring/operator/internal/constants"
	"github.com/Bidon15/banhbaoring/operator/internal/resources"
)

// ServiceAccount builds the ServiceAccount for OpenBao.
func ServiceAccount(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.ServiceAccount {
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)
	labels := resources.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)

	return &corev1.ServiceAccount{
		ObjectMeta: resources.ObjectMeta(name, cluster.Namespace, labels),
	}
}

// HeadlessService builds the headless service for StatefulSet DNS.
func HeadlessService(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)
	labels := resources.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)
	selectorLabels := resources.SelectorLabels(cluster.Name, constants.ComponentOpenBao)

	return &corev1.Service{
		ObjectMeta: resources.ObjectMeta(name, cluster.Namespace, labels),
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "api",
					Port:       OpenBaoPort,
					TargetPort: intstr.FromInt(OpenBaoPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "cluster",
					Port:       OpenBaoClusterPort,
					TargetPort: intstr.FromInt(OpenBaoClusterPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			PublishNotReadyAddresses: true,
		},
	}
}

// ActiveService builds the service that routes to all OpenBao pods.
func ActiveService(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)
	labels := resources.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)
	selectorLabels := resources.SelectorLabels(cluster.Name, constants.ComponentOpenBao)

	return &corev1.Service{
		ObjectMeta: resources.ObjectMeta(name+"-active", cluster.Namespace, labels),
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "api",
					Port:       OpenBaoPort,
					TargetPort: intstr.FromInt(OpenBaoPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// InternalService builds an internal service for control plane components.
func InternalService(cluster *banhbaoringv1.BanhBaoRingCluster) *corev1.Service {
	name := resources.ResourceName(cluster.Name, constants.ComponentOpenBao)
	labels := resources.Labels(cluster.Name, constants.ComponentOpenBao, cluster.Spec.OpenBao.Version)
	selectorLabels := resources.SelectorLabels(cluster.Name, constants.ComponentOpenBao)

	return &corev1.Service{
		ObjectMeta: resources.ObjectMeta(name+"-internal", cluster.Namespace, labels),
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "api",
					Port:       OpenBaoPort,
					TargetPort: intstr.FromInt(OpenBaoPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
