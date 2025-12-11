package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

// ClusterReconciler reconciles a BanhBaoRingCluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets;deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling BanhBaoRingCluster", "name", req.Name)

	// Fetch the cluster
	cluster := &banhbaoringv1.BanhBaoRingCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Phase 1: Prerequisites (namespace, certificates)
	// TODO: Implement prerequisites reconciliation

	// Phase 2: Data layer (PostgreSQL, Redis)
	if err := r.reconcilePostgreSQL(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile PostgreSQL")
		return ctrl.Result{}, err
	}

	if err := r.reconcileRedis(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile Redis")
		return ctrl.Result{}, err
	}

	// Update database status
	if err := r.updateDatabaseStatus(ctx, cluster); err != nil {
		log.Error(err, "Failed to update database status")
		return ctrl.Result{}, err
	}

	// Check if data layer is ready before proceeding
	if !r.isDataLayerReady(ctx, cluster) {
		log.Info("Data layer not ready, requeuing")
		if err := r.Status().Update(ctx, cluster); err != nil {
			log.Error(err, "Failed to update cluster status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Phase 3: OpenBao (StatefulSet, unseal, plugin)
	if err := r.reconcileOpenBao(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile OpenBao")
		return ctrl.Result{}, err
	}

	// Update OpenBao status
	if err := r.updateOpenBaoStatus(ctx, cluster); err != nil {
		log.Error(err, "Failed to update OpenBao status")
		return ctrl.Result{}, err
	}

	// Check if OpenBao is ready before proceeding to apps
	if !r.isOpenBaoReady(ctx, cluster) {
		log.Info("OpenBao not ready, requeuing")
		if err := r.Status().Update(ctx, cluster); err != nil {
			log.Error(err, "Failed to update cluster status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Phase 4: Applications (API, Dashboard)
	if err := r.reconcileAPI(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile API")
		return ctrl.Result{}, err
	}

	if err := r.reconcileDashboard(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile Dashboard")
		return ctrl.Result{}, err
	}

	// Update application status
	if err := r.updateAppsStatus(ctx, cluster); err != nil {
		log.Error(err, "Failed to update apps status")
		return ctrl.Result{}, err
	}

	// Phase 5: Monitoring (optional)
	if err := r.reconcileMonitoring(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile monitoring")
		return ctrl.Result{}, err
	}

	// Phase 6: Ingress & networking
	// TODO: Implement networking reconciliation

	// Update status
	if err := r.Status().Update(ctx, cluster); err != nil {
		log.Error(err, "Failed to update cluster status")
		return ctrl.Result{}, err
	}

	// Requeue after 30s for periodic reconciliation
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&banhbaoringv1.BanhBaoRingCluster{}).
		Complete(r)
}
