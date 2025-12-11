package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

// RestoreReconciler reconciles a BanhBaoRingRestore object
type RestoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringrestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling BanhBaoRingRestore", "name", req.Name)

	// Fetch the restore
	restore := &banhbaoringv1.BanhBaoRingRestore{}
	if err := r.Get(ctx, req.NamespacedName, restore); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip if already completed or failed
	if restore.Status.Phase == "Completed" || restore.Status.Phase == "Failed" {
		return ctrl.Result{}, nil
	}

	// Get parent cluster
	cluster := &banhbaoringv1.BanhBaoRingCluster{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      restore.Spec.ClusterRef.Name,
		Namespace: restore.Namespace,
	}, cluster); err != nil {
		log.Error(err, "Failed to get parent cluster", "cluster", restore.Spec.ClusterRef.Name)
		return r.updateRestoreStatus(ctx, restore, "Failed", fmt.Sprintf("Parent cluster not found: %v", err))
	}

	// Verify backup exists and is complete (if referencing a backup)
	if restore.Spec.BackupRef != nil {
		backup := &banhbaoringv1.BanhBaoRingBackup{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      restore.Spec.BackupRef.Name,
			Namespace: restore.Namespace,
		}, backup); err != nil {
			log.Error(err, "Failed to get backup", "backup", restore.Spec.BackupRef.Name)
			return r.updateRestoreStatus(ctx, restore, "Failed", fmt.Sprintf("Backup not found: %v", err))
		}
		if backup.Status.Phase != "Completed" {
			return r.updateRestoreStatus(ctx, restore, "Failed", "Backup is not completed")
		}
	}

	// Execute phased restore workflow
	switch restore.Status.Phase {
	case "", "Pending":
		// Start the restore process
		now := metav1.Now()
		restore.Status.StartedAt = &now
		restore.Status.Steps = r.initializeSteps()

		if restore.Spec.Options.StopApplications {
			log.Info("Scaling down applications before restore")
			if err := r.scaleDownApps(ctx, cluster); err != nil {
				log.Error(err, "Failed to scale down applications")
				return r.updateRestoreStatus(ctx, restore, "Failed", fmt.Sprintf("Failed to stop applications: %v", err))
			}
			r.updateStep(restore, "stop-applications", "Running")
			return r.updateRestoreStatus(ctx, restore, "Stopping", "Scaling down applications")
		}
		return r.updateRestoreStatus(ctx, restore, "Restoring", "Starting restore")

	case "Stopping":
		// Wait for apps to scale down
		if r.appsScaledDown(ctx, cluster) {
			log.Info("Applications scaled down, starting restore")
			r.updateStep(restore, "stop-applications", "Completed")
			r.updateStep(restore, "restore-data", "Running")
			return r.updateRestoreStatus(ctx, restore, "Restoring", "Applications stopped, restoring data")
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil

	case "Restoring":
		// Run restore job
		jobName := fmt.Sprintf("%s-restore", restore.Name)
		job := &batchv1.Job{}
		err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: restore.Namespace}, job)

		if errors.IsNotFound(err) {
			// Create restore job
			if err := r.runRestore(ctx, restore, cluster); err != nil {
				log.Error(err, "Failed to create restore job")
				return r.updateRestoreStatus(ctx, restore, "Failed", fmt.Sprintf("Failed to create restore job: %v", err))
			}
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		if err != nil {
			return ctrl.Result{}, err
		}

		// Check job status
		if job.Status.Succeeded > 0 {
			log.Info("Restore job completed successfully")
			r.updateStep(restore, "restore-data", "Completed")
			r.updateStep(restore, "start-applications", "Running")
			return r.updateRestoreStatus(ctx, restore, "Starting", "Restore complete, starting applications")
		}
		if job.Status.Failed > 0 {
			r.updateStep(restore, "restore-data", "Failed")
			return r.updateRestoreStatus(ctx, restore, "Failed", "Restore job failed")
		}

		// Still running
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

	case "Starting":
		// Scale apps back up
		if err := r.scaleUpApps(ctx, cluster); err != nil {
			log.Error(err, "Failed to scale up applications")
			return r.updateRestoreStatus(ctx, restore, "Failed", fmt.Sprintf("Failed to start applications: %v", err))
		}

		// Wait for apps to be ready
		if r.appsReady(ctx, cluster) {
			log.Info("Restore completed successfully")
			r.updateStep(restore, "start-applications", "Completed")
			now := metav1.Now()
			restore.Status.CompletedAt = &now
			return r.updateRestoreStatus(ctx, restore, "Completed", "Restore completed successfully")
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *RestoreReconciler) initializeSteps() []banhbaoringv1.RestoreStep {
	return []banhbaoringv1.RestoreStep{
		{Name: "stop-applications", Status: "Pending"},
		{Name: "restore-data", Status: "Pending"},
		{Name: "start-applications", Status: "Pending"},
	}
}

func (r *RestoreReconciler) updateStep(restore *banhbaoringv1.BanhBaoRingRestore, stepName, status string) {
	for i, step := range restore.Status.Steps {
		if step.Name == stepName {
			restore.Status.Steps[i].Status = status
			return
		}
	}
}

func (r *RestoreReconciler) scaleDownApps(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
	zero := int32(0)

	// Scale API deployment to 0
	apiDeployment := &appsv1.Deployment{}
	apiName := fmt.Sprintf("%s-api", cluster.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: apiName, Namespace: cluster.Namespace}, apiDeployment); err == nil {
		apiDeployment.Spec.Replicas = &zero
		if err := r.Update(ctx, apiDeployment); err != nil {
			return fmt.Errorf("failed to scale down API: %w", err)
		}
	}

	// Scale Dashboard deployment to 0
	dashboardDeployment := &appsv1.Deployment{}
	dashboardName := fmt.Sprintf("%s-dashboard", cluster.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: dashboardName, Namespace: cluster.Namespace}, dashboardDeployment); err == nil {
		dashboardDeployment.Spec.Replicas = &zero
		if err := r.Update(ctx, dashboardDeployment); err != nil {
			return fmt.Errorf("failed to scale down dashboard: %w", err)
		}
	}

	return nil
}

func (r *RestoreReconciler) scaleUpApps(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) error {
	// Scale API deployment back up
	apiDeployment := &appsv1.Deployment{}
	apiName := fmt.Sprintf("%s-api", cluster.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: apiName, Namespace: cluster.Namespace}, apiDeployment); err == nil {
		replicas := cluster.Spec.API.Replicas
		if replicas == 0 {
			replicas = 2
		}
		apiDeployment.Spec.Replicas = &replicas
		if err := r.Update(ctx, apiDeployment); err != nil {
			return fmt.Errorf("failed to scale up API: %w", err)
		}
	}

	// Scale Dashboard deployment back up
	dashboardDeployment := &appsv1.Deployment{}
	dashboardName := fmt.Sprintf("%s-dashboard", cluster.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: dashboardName, Namespace: cluster.Namespace}, dashboardDeployment); err == nil {
		replicas := cluster.Spec.Dashboard.Replicas
		if replicas == 0 {
			replicas = 2
		}
		dashboardDeployment.Spec.Replicas = &replicas
		if err := r.Update(ctx, dashboardDeployment); err != nil {
			return fmt.Errorf("failed to scale up dashboard: %w", err)
		}
	}

	return nil
}

func (r *RestoreReconciler) appsScaledDown(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) bool {
	// Check API deployment
	apiDeployment := &appsv1.Deployment{}
	apiName := fmt.Sprintf("%s-api", cluster.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: apiName, Namespace: cluster.Namespace}, apiDeployment); err == nil {
		if apiDeployment.Status.Replicas > 0 {
			return false
		}
	}

	// Check Dashboard deployment
	dashboardDeployment := &appsv1.Deployment{}
	dashboardName := fmt.Sprintf("%s-dashboard", cluster.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: dashboardName, Namespace: cluster.Namespace}, dashboardDeployment); err == nil {
		if dashboardDeployment.Status.Replicas > 0 {
			return false
		}
	}

	return true
}

func (r *RestoreReconciler) appsReady(ctx context.Context, cluster *banhbaoringv1.BanhBaoRingCluster) bool {
	// Check API deployment
	apiDeployment := &appsv1.Deployment{}
	apiName := fmt.Sprintf("%s-api", cluster.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: apiName, Namespace: cluster.Namespace}, apiDeployment); err == nil {
		if apiDeployment.Status.ReadyReplicas < *apiDeployment.Spec.Replicas {
			return false
		}
	}

	// Check Dashboard deployment
	dashboardDeployment := &appsv1.Deployment{}
	dashboardName := fmt.Sprintf("%s-dashboard", cluster.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: dashboardName, Namespace: cluster.Namespace}, dashboardDeployment); err == nil {
		if dashboardDeployment.Status.ReadyReplicas < *dashboardDeployment.Spec.Replicas {
			return false
		}
	}

	return true
}

func (r *RestoreReconciler) runRestore(ctx context.Context, restore *banhbaoringv1.BanhBaoRingRestore, cluster *banhbaoringv1.BanhBaoRingCluster) error {
	name := fmt.Sprintf("%s-restore", restore.Name)
	backoffLimit := int32(2)

	script := r.buildRestoreScript(restore)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: restore.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "banhbaoring-restore",
				"app.kubernetes.io/instance":   restore.Name,
				"app.kubernetes.io/managed-by": "banhbaoring-operator",
				"banhbaoring.io/cluster":       cluster.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "banhbaoring-restore",
						"app.kubernetes.io/instance": restore.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:            "restore",
						Image:           "banhbaoring/backup:latest",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/sh", "-c", script},
						Env:             r.buildRestoreEnv(restore, cluster),
					}},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(restore, job, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, job)
}

func (r *RestoreReconciler) buildRestoreScript(restore *banhbaoringv1.BanhBaoRingRestore) string {
	return `#!/bin/sh
set -e

echo "Starting restore process..."

# Restore OpenBao (Raft snapshot)
if echo "$COMPONENTS" | grep -q "openbao"; then
    echo "Restoring OpenBao..."
    aws s3 cp s3://${S3_BUCKET}/${BACKUP_PATH}/openbao-*.snap /tmp/openbao.snap
    vault operator raft snapshot restore -force /tmp/openbao.snap
fi

# Restore PostgreSQL
if echo "$COMPONENTS" | grep -q "database"; then
    echo "Restoring PostgreSQL..."
    aws s3 cp s3://${S3_BUCKET}/${BACKUP_PATH}/postgres-*.sql.gz /tmp/postgres.sql.gz
    gunzip -c /tmp/postgres.sql.gz | psql $DATABASE_URL
fi

echo "Restore completed successfully"
`
}

func (r *RestoreReconciler) buildRestoreEnv(restore *banhbaoringv1.BanhBaoRingRestore, cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
	// Determine components to restore
	components := restore.Spec.Components
	if len(components) == 0 {
		components = []string{"openbao", "database"}
	}

	// Determine source
	source := restore.Spec.Source
	if source == nil {
		source = &cluster.Spec.Backup.Destination
	}

	env := []corev1.EnvVar{
		{Name: "COMPONENTS", Value: fmt.Sprintf("%v", components)},
		{Name: "CLUSTER_NAME", Value: cluster.Name},
		{Name: "NAMESPACE", Value: restore.Namespace},
	}

	// Add S3 configuration
	if source != nil && source.S3 != nil {
		env = append(env,
			corev1.EnvVar{Name: "S3_BUCKET", Value: source.S3.Bucket},
			corev1.EnvVar{Name: "BACKUP_PATH", Value: source.S3.Prefix},
			corev1.EnvVar{Name: "AWS_REGION", Value: source.S3.Region},
		)
		// Add credentials from secret
		env = append(env,
			corev1.EnvVar{
				Name: "AWS_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: source.S3.Credentials.Name,
						},
						Key: "access-key-id",
					},
				},
			},
			corev1.EnvVar{
				Name: "AWS_SECRET_ACCESS_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: source.S3.Credentials.Name,
						},
						Key: "secret-access-key",
					},
				},
			},
		)
	}

	// Add database URL
	dbSecretName := fmt.Sprintf("%s-database-credentials", cluster.Name)
	env = append(env,
		corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: dbSecretName,
					},
					Key:      "url",
					Optional: boolPtr(true),
				},
			},
		},
	)

	return env
}

func (r *RestoreReconciler) updateRestoreStatus(ctx context.Context, restore *banhbaoringv1.BanhBaoRingRestore, phase, message string) (ctrl.Result, error) {
	restore.Status.Phase = phase

	// Update conditions
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             phase,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	if phase == "Completed" {
		condition.Status = metav1.ConditionTrue
		condition.Message = "Restore completed successfully"
	}

	// Update or add condition
	found := false
	for i, c := range restore.Status.Conditions {
		if c.Type == "Ready" {
			restore.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		restore.Status.Conditions = append(restore.Status.Conditions, condition)
	}

	if err := r.Status().Update(ctx, restore); err != nil {
		return ctrl.Result{}, err
	}

	if phase == "Stopping" || phase == "Restoring" || phase == "Starting" {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&banhbaoringv1.BanhBaoRingRestore{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
