package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// BackupReconciler reconciles a BanhBaoRingBackup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=banhbaoring.io,resources=banhbaoringbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling BanhBaoRingBackup", "name", req.Name)

	// Fetch the backup
	backup := &banhbaoringv1.BanhBaoRingBackup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip if already completed or failed
	if backup.Status.Phase == "Completed" || backup.Status.Phase == "Failed" {
		return ctrl.Result{}, nil
	}

	// Get parent cluster
	cluster := &banhbaoringv1.BanhBaoRingCluster{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      backup.Spec.ClusterRef.Name,
		Namespace: backup.Namespace,
	}, cluster); err != nil {
		log.Error(err, "Failed to get parent cluster", "cluster", backup.Spec.ClusterRef.Name)
		return r.updateBackupStatus(ctx, backup, "Failed", fmt.Sprintf("Parent cluster not found: %v", err))
	}

	// Create backup job if not started
	if backup.Status.Phase == "" || backup.Status.Phase == "Pending" {
		jobName := fmt.Sprintf("%s-backup", backup.Name)

		// Check if job already exists
		existingJob := &batchv1.Job{}
		err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: backup.Namespace}, existingJob)
		if err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		if errors.IsNotFound(err) {
			// Create the backup job
			job := r.buildBackupJob(backup, cluster)
			if err := ctrl.SetControllerReference(backup, job, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Failed to create backup job")
				return r.updateBackupStatus(ctx, backup, "Failed", fmt.Sprintf("Failed to create job: %v", err))
			}
			log.Info("Created backup job", "job", jobName)
		}

		now := metav1.Now()
		backup.Status.StartedAt = &now
		return r.updateBackupStatus(ctx, backup, "Running", "")
	}

	// Check job status
	job := &batchv1.Job{}
	jobName := fmt.Sprintf("%s-backup", backup.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: backup.Namespace}, job); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Waiting for backup job to appear", "job", jobName)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// Job succeeded
	if job.Status.Succeeded > 0 {
		now := metav1.Now()
		backup.Status.CompletedAt = &now
		backup.Status.Components = r.buildComponentStatus(backup, "Completed")
		log.Info("Backup completed successfully", "backup", backup.Name)
		return r.updateBackupStatus(ctx, backup, "Completed", "")
	}

	// Job failed
	if job.Status.Failed > 0 {
		backup.Status.Components = r.buildComponentStatus(backup, "Failed")
		log.Info("Backup job failed", "backup", backup.Name)
		return r.updateBackupStatus(ctx, backup, "Failed", "Backup job failed")
	}

	// Still running
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *BackupReconciler) buildBackupJob(backup *banhbaoringv1.BanhBaoRingBackup, cluster *banhbaoringv1.BanhBaoRingCluster) *batchv1.Job {
	name := fmt.Sprintf("%s-backup", backup.Name)
	backoffLimit := int32(2)

	// Build backup script
	script := r.buildBackupScript(backup, cluster)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: backup.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "banhbaoring-backup",
				"app.kubernetes.io/instance":   backup.Name,
				"app.kubernetes.io/managed-by": "banhbaoring-operator",
				"banhbaoring.io/cluster":       cluster.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "banhbaoring-backup",
						"app.kubernetes.io/instance": backup.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:            "backup",
						Image:           "banhbaoring/backup:latest",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/sh", "-c", script},
						Env:             r.buildBackupEnv(backup, cluster),
					}},
				},
			},
		},
	}
}

func (r *BackupReconciler) buildBackupScript(backup *banhbaoringv1.BanhBaoRingBackup, cluster *banhbaoringv1.BanhBaoRingCluster) string {
	return `#!/bin/sh
set -e

TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Backup OpenBao (Raft snapshot)
if echo "$COMPONENTS" | grep -q "openbao"; then
    echo "Backing up OpenBao..."
    vault operator raft snapshot save /tmp/openbao-${TIMESTAMP}.snap
    aws s3 cp /tmp/openbao-${TIMESTAMP}.snap s3://${S3_BUCKET}/${S3_PREFIX}openbao-${TIMESTAMP}.snap
fi

# Backup PostgreSQL
if echo "$COMPONENTS" | grep -q "database"; then
    echo "Backing up PostgreSQL..."
    pg_dump $DATABASE_URL | gzip > /tmp/postgres-${TIMESTAMP}.sql.gz
    aws s3 cp /tmp/postgres-${TIMESTAMP}.sql.gz s3://${S3_BUCKET}/${S3_PREFIX}postgres-${TIMESTAMP}.sql.gz
fi

# Backup secrets metadata
if echo "$COMPONENTS" | grep -q "secrets"; then
    echo "Backing up secrets metadata..."
    kubectl get secrets -n ${NAMESPACE} -l banhbaoring.io/cluster=${CLUSTER_NAME} -o yaml > /tmp/secrets-${TIMESTAMP}.yaml
    aws s3 cp /tmp/secrets-${TIMESTAMP}.yaml s3://${S3_BUCKET}/${S3_PREFIX}secrets-${TIMESTAMP}.yaml
fi

echo "Backup completed successfully"
`
}

func (r *BackupReconciler) buildBackupEnv(backup *banhbaoringv1.BanhBaoRingBackup, cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
	// Determine components to backup
	components := backup.Spec.Components
	if len(components) == 0 {
		components = []string{"openbao", "database", "secrets"}
	}

	// Determine backup destination (use backup-specific or fall back to cluster)
	dest := backup.Spec.Destination
	if dest == nil {
		dest = &cluster.Spec.Backup.Destination
	}

	env := []corev1.EnvVar{
		{Name: "COMPONENTS", Value: strings.Join(components, ",")},
		{Name: "CLUSTER_NAME", Value: cluster.Name},
		{Name: "NAMESPACE", Value: backup.Namespace},
		{Name: "BACKUP_TYPE", Value: backup.Spec.Type},
	}

	// Add S3 configuration
	if dest != nil && dest.S3 != nil {
		env = append(env,
			corev1.EnvVar{Name: "S3_BUCKET", Value: dest.S3.Bucket},
			corev1.EnvVar{Name: "S3_PREFIX", Value: dest.S3.Prefix},
			corev1.EnvVar{Name: "AWS_REGION", Value: dest.S3.Region},
		)
		// Add credentials from secret
		env = append(env,
			corev1.EnvVar{
				Name: "AWS_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dest.S3.Credentials.Name,
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
							Name: dest.S3.Credentials.Name,
						},
						Key: "secret-access-key",
					},
				},
			},
		)
	}

	// Add GCS configuration
	if dest != nil && dest.GCS != nil {
		env = append(env,
			corev1.EnvVar{Name: "GCS_BUCKET", Value: dest.GCS.Bucket},
			corev1.EnvVar{Name: "GCS_PREFIX", Value: dest.GCS.Prefix},
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

func (r *BackupReconciler) buildComponentStatus(backup *banhbaoringv1.BanhBaoRingBackup, status string) []banhbaoringv1.BackupComponentStatus {
	components := backup.Spec.Components
	if len(components) == 0 {
		components = []string{"openbao", "database", "secrets"}
	}

	result := make([]banhbaoringv1.BackupComponentStatus, len(components))
	for i, comp := range components {
		result[i] = banhbaoringv1.BackupComponentStatus{
			Name:   comp,
			Status: status,
		}
	}
	return result
}

func (r *BackupReconciler) updateBackupStatus(ctx context.Context, backup *banhbaoringv1.BanhBaoRingBackup, phase, message string) (ctrl.Result, error) {
	backup.Status.Phase = phase

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
		condition.Message = "Backup completed successfully"
	}

	// Update or add condition
	found := false
	for i, c := range backup.Status.Conditions {
		if c.Type == "Ready" {
			backup.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		backup.Status.Conditions = append(backup.Status.Conditions, condition)
	}

	if err := r.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, err
	}

	if phase == "Running" {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func boolPtr(b bool) *bool {
	return &b
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&banhbaoringv1.BanhBaoRingBackup{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
