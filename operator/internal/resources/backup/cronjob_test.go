package backup

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

func TestCronJob(t *testing.T) {
	tests := []struct {
		name             string
		cluster          *banhbaoringv1.BanhBaoRingCluster
		expectedName     string
		expectedSchedule string
		wantS3Env        bool
		wantGCSEnv       bool
	}{
		{
			name: "default schedule with S3",
			cluster: &banhbaoringv1.BanhBaoRingCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingClusterSpec{
					Domain: "keys.example.com",
					Backup: banhbaoringv1.BackupSpec{
						Enabled: true,
						Destination: banhbaoringv1.BackupDestination{
							S3: &banhbaoringv1.S3Destination{
								Bucket: "my-bucket",
								Region: "us-west-2",
								Prefix: "backups/",
								Credentials: banhbaoringv1.SecretKeyRef{
									Name: "aws-creds",
									Key:  "credentials",
								},
							},
						},
					},
				},
			},
			expectedName:     "test-cluster-backup",
			expectedSchedule: DefaultSchedule,
			wantS3Env:        true,
			wantGCSEnv:       false,
		},
		{
			name: "custom schedule with GCS",
			cluster: &banhbaoringv1.BanhBaoRingCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prod-cluster",
					Namespace: "production",
				},
				Spec: banhbaoringv1.BanhBaoRingClusterSpec{
					Domain: "keys.prod.example.com",
					Backup: banhbaoringv1.BackupSpec{
						Enabled:   true,
						Schedule:  "0 4 * * 0",
						Retention: 90,
						Destination: banhbaoringv1.BackupDestination{
							GCS: &banhbaoringv1.GCSDestination{
								Bucket: "gcs-bucket",
								Prefix: "prod-backups/",
								Credentials: banhbaoringv1.SecretKeyRef{
									Name: "gcp-creds",
									Key:  "service-account.json",
								},
							},
						},
					},
				},
			},
			expectedName:     "prod-cluster-backup",
			expectedSchedule: "0 4 * * 0",
			wantS3Env:        false,
			wantGCSEnv:       true,
		},
		{
			name: "empty schedule uses default",
			cluster: &banhbaoringv1.BanhBaoRingCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dev-cluster",
					Namespace: "development",
				},
				Spec: banhbaoringv1.BanhBaoRingClusterSpec{
					Domain: "keys.dev.example.com",
					Backup: banhbaoringv1.BackupSpec{
						Enabled: true,
					},
				},
			},
			expectedName:     "dev-cluster-backup",
			expectedSchedule: DefaultSchedule,
			wantS3Env:        false,
			wantGCSEnv:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cronjob := CronJob(tt.cluster)

			// Check name
			if cronjob.Name != tt.expectedName {
				t.Errorf("expected name %s, got %s", tt.expectedName, cronjob.Name)
			}

			// Check namespace
			if cronjob.Namespace != tt.cluster.Namespace {
				t.Errorf("expected namespace %s, got %s", tt.cluster.Namespace, cronjob.Namespace)
			}

			// Check schedule
			if cronjob.Spec.Schedule != tt.expectedSchedule {
				t.Errorf("expected schedule %s, got %s", tt.expectedSchedule, cronjob.Spec.Schedule)
			}

			// Check concurrency policy
			if cronjob.Spec.ConcurrencyPolicy != "Forbid" {
				t.Errorf("expected concurrency policy Forbid, got %s", cronjob.Spec.ConcurrencyPolicy)
			}

			// Check container
			containers := cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers
			if len(containers) != 1 {
				t.Fatalf("expected 1 container, got %d", len(containers))
			}

			if containers[0].Name != "backup" {
				t.Errorf("expected container name 'backup', got %s", containers[0].Name)
			}

			if containers[0].Image != BackupImage {
				t.Errorf("expected image %s, got %s", BackupImage, containers[0].Image)
			}

			// Check environment variables
			env := containers[0].Env
			hasS3Bucket := false
			hasGCSBucket := false
			hasClusterName := false

			for _, e := range env {
				switch e.Name {
				case "S3_BUCKET":
					hasS3Bucket = true
				case "GCS_BUCKET":
					hasGCSBucket = true
				case "CLUSTER_NAME":
					hasClusterName = true
					if e.Value != tt.cluster.Name {
						t.Errorf("expected CLUSTER_NAME %s, got %s", tt.cluster.Name, e.Value)
					}
				}
			}

			if hasS3Bucket != tt.wantS3Env {
				t.Errorf("expected hasS3Bucket=%v, got %v", tt.wantS3Env, hasS3Bucket)
			}

			if hasGCSBucket != tt.wantGCSEnv {
				t.Errorf("expected hasGCSBucket=%v, got %v", tt.wantGCSEnv, hasGCSBucket)
			}

			if !hasClusterName {
				t.Error("expected CLUSTER_NAME env var to be set")
			}
		})
	}
}

func TestCronJobName(t *testing.T) {
	tests := []struct {
		clusterName string
		expected    string
	}{
		{"my-cluster", "my-cluster-backup"},
		{"prod", "prod-backup"},
		{"test-env-cluster", "test-env-cluster-backup"},
	}

	for _, tt := range tests {
		t.Run(tt.clusterName, func(t *testing.T) {
			got := CronJobName(tt.clusterName)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestBuildEnv(t *testing.T) {
	cluster := &banhbaoringv1.BanhBaoRingCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: banhbaoringv1.BanhBaoRingClusterSpec{
			Backup: banhbaoringv1.BackupSpec{
				Enabled:   true,
				Retention: 30,
				Destination: banhbaoringv1.BackupDestination{
					S3: &banhbaoringv1.S3Destination{
						Bucket: "test-bucket",
						Region: "eu-west-1",
						Prefix: "daily/",
						Credentials: banhbaoringv1.SecretKeyRef{
							Name: "aws-secret",
							Key:  "key",
						},
					},
				},
			},
		},
	}

	env := buildEnv(cluster)

	// Check expected env vars exist
	envMap := make(map[string]string)
	for _, e := range env {
		if e.Value != "" {
			envMap[e.Name] = e.Value
		}
	}

	expectedVars := map[string]string{
		"CLUSTER_NAME":   "test-cluster",
		"NAMESPACE":      "default",
		"COMPONENTS":     "openbao,database,secrets",
		"BACKUP_TYPE":    "full",
		"S3_BUCKET":      "test-bucket",
		"S3_PREFIX":      "daily/",
		"AWS_REGION":     "eu-west-1",
		"VAULT_ADDR":     "http://test-cluster-openbao:8200",
		"RETENTION_DAYS": "30",
	}

	for name, expected := range expectedVars {
		if got, ok := envMap[name]; !ok {
			t.Errorf("expected env var %s to be set", name)
		} else if got != expected {
			t.Errorf("expected %s=%s, got %s", name, expected, got)
		}
	}
}
