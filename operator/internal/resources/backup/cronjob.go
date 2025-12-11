// Package backup provides helpers for building backup-related Kubernetes resources.
package backup

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
	"github.com/Bidon15/banhbaoring/operator/internal/resources"
)

const (
	// DefaultSchedule is the default backup schedule (daily at 2 AM UTC)
	DefaultSchedule = "0 2 * * *"

	// BackupImage is the default backup container image
	BackupImage = "banhbaoring/backup:latest"
)

// CronJob creates a CronJob for scheduled backups of a BanhBaoRingCluster.
func CronJob(cluster *banhbaoringv1.BanhBaoRingCluster) *batchv1.CronJob {
	spec := cluster.Spec.Backup
	name := fmt.Sprintf("%s-backup", cluster.Name)

	schedule := spec.Schedule
	if schedule == "" {
		schedule = DefaultSchedule
	}

	labels := resources.Labels(cluster.Name, "backup", "1.0.0")
	successfulJobsHistoryLimit := int32(3)
	failedJobsHistoryLimit := int32(1)
	backoffLimit := int32(2)

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   schedule,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: batchv1.JobSpec{
					BackoffLimit: &backoffLimit,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: resources.SelectorLabels(cluster.Name, "backup"),
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{{
								Name:            "backup",
								Image:           BackupImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         []string{"/backup.sh"},
								Env:             buildEnv(cluster),
							}},
						},
					},
				},
			},
		},
	}
}

// buildEnv creates environment variables for the backup container.
func buildEnv(cluster *banhbaoringv1.BanhBaoRingCluster) []corev1.EnvVar {
	dest := cluster.Spec.Backup.Destination
	var env []corev1.EnvVar

	// Common environment variables
	env = append(env,
		corev1.EnvVar{Name: "CLUSTER_NAME", Value: cluster.Name},
		corev1.EnvVar{Name: "NAMESPACE", Value: cluster.Namespace},
		corev1.EnvVar{Name: "COMPONENTS", Value: "openbao,database,secrets"},
		corev1.EnvVar{Name: "BACKUP_TYPE", Value: "full"},
	)

	// S3 configuration
	if dest.S3 != nil {
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

	// GCS configuration
	if dest.GCS != nil {
		env = append(env,
			corev1.EnvVar{Name: "GCS_BUCKET", Value: dest.GCS.Bucket},
			corev1.EnvVar{Name: "GCS_PREFIX", Value: dest.GCS.Prefix},
		)

		// Add credentials from secret
		env = append(env,
			corev1.EnvVar{
				Name: "GOOGLE_APPLICATION_CREDENTIALS",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dest.GCS.Credentials.Name,
						},
						Key: dest.GCS.Credentials.Key,
					},
				},
			},
		)
	}

	// Database credentials
	dbSecretName := fmt.Sprintf("%s-database-credentials", cluster.Name)
	optional := true
	env = append(env,
		corev1.EnvVar{
			Name: "DATABASE_URL",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: dbSecretName,
					},
					Key:      "url",
					Optional: &optional,
				},
			},
		},
	)

	// OpenBao address
	openbaoAddr := fmt.Sprintf("http://%s-openbao:8200", cluster.Name)
	env = append(env,
		corev1.EnvVar{Name: "VAULT_ADDR", Value: openbaoAddr},
	)

	// Retention days
	if cluster.Spec.Backup.Retention > 0 {
		env = append(env,
			corev1.EnvVar{Name: "RETENTION_DAYS", Value: fmt.Sprintf("%d", cluster.Spec.Backup.Retention)},
		)
	}

	return env
}

// CronJobName returns the name of the backup CronJob for a cluster.
func CronJobName(clusterName string) string {
	return fmt.Sprintf("%s-backup", clusterName)
}
