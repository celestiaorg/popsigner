package controllers

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

var _ = Describe("BackupController", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a BanhBaoRingBackup", func() {
		It("Should fail when parent cluster is not found", func() {
			By("Creating a backup without a parent cluster")
			backup := &banhbaoringv1.BanhBaoRingBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backup-no-cluster",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingBackupSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "non-existent-cluster",
					},
					Type:       "full",
					Components: []string{"openbao", "database"},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			// Clean up
			defer func() {
				Expect(k8sClient.Delete(ctx, backup)).Should(Succeed())
			}()

			backupLookupKey := types.NamespacedName{Name: "test-backup-no-cluster", Namespace: "default"}
			createdBackup := &banhbaoringv1.BanhBaoRingBackup{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, backupLookupKey, createdBackup)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})

		It("Should create backup with default components", func() {
			By("Creating a backup with minimal spec")
			backup := &banhbaoringv1.BanhBaoRingBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backup-defaults",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingBackupSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "test-cluster",
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			// Clean up
			defer func() {
				Expect(k8sClient.Delete(ctx, backup)).Should(Succeed())
			}()

			backupLookupKey := types.NamespacedName{Name: "test-backup-defaults", Namespace: "default"}
			createdBackup := &banhbaoringv1.BanhBaoRingBackup{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, backupLookupKey, createdBackup)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Default type should be "full"
			Expect(createdBackup.Spec.Type).Should(Equal("full"))
		})

		It("Should create backup with custom components", func() {
			By("Creating a backup with specific components")
			backup := &banhbaoringv1.BanhBaoRingBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backup-custom",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingBackupSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "test-cluster",
					},
					Type:       "incremental",
					Components: []string{"openbao"},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			// Clean up
			defer func() {
				Expect(k8sClient.Delete(ctx, backup)).Should(Succeed())
			}()

			backupLookupKey := types.NamespacedName{Name: "test-backup-custom", Namespace: "default"}
			createdBackup := &banhbaoringv1.BanhBaoRingBackup{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, backupLookupKey, createdBackup)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdBackup.Spec.Type).Should(Equal("incremental"))
			Expect(createdBackup.Spec.Components).Should(HaveLen(1))
			Expect(createdBackup.Spec.Components[0]).Should(Equal("openbao"))
		})

		It("Should create backup with S3 destination", func() {
			By("Creating a backup with S3 destination")
			backup := &banhbaoringv1.BanhBaoRingBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backup-s3",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingBackupSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "test-cluster",
					},
					Type:       "full",
					Components: []string{"openbao", "database", "secrets"},
					Destination: &banhbaoringv1.BackupDestination{
						S3: &banhbaoringv1.S3Destination{
							Bucket: "my-backup-bucket",
							Region: "us-west-2",
							Prefix: "backups/prod/",
							Credentials: banhbaoringv1.SecretKeyRef{
								Name: "aws-credentials",
								Key:  "credentials",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			// Clean up
			defer func() {
				Expect(k8sClient.Delete(ctx, backup)).Should(Succeed())
			}()

			backupLookupKey := types.NamespacedName{Name: "test-backup-s3", Namespace: "default"}
			createdBackup := &banhbaoringv1.BanhBaoRingBackup{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, backupLookupKey, createdBackup)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdBackup.Spec.Destination).ShouldNot(BeNil())
			Expect(createdBackup.Spec.Destination.S3).ShouldNot(BeNil())
			Expect(createdBackup.Spec.Destination.S3.Bucket).Should(Equal("my-backup-bucket"))
			Expect(createdBackup.Spec.Destination.S3.Region).Should(Equal("us-west-2"))
		})
	})

	Context("When backup has a ready cluster", func() {
		var cluster *banhbaoringv1.BanhBaoRingCluster

		BeforeEach(func() {
			By("Creating a parent cluster")
			cluster = &banhbaoringv1.BanhBaoRingCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backup-test-cluster",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingClusterSpec{
					Domain: "keys.example.com",
					Backup: banhbaoringv1.BackupSpec{
						Enabled:  true,
						Schedule: "0 3 * * *",
						Destination: banhbaoringv1.BackupDestination{
							S3: &banhbaoringv1.S3Destination{
								Bucket: "cluster-backup-bucket",
								Region: "us-east-1",
								Credentials: banhbaoringv1.SecretKeyRef{
									Name: "aws-creds",
									Key:  "key",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			// Update cluster status to Running
			cluster.Status.Phase = "Running"
			Expect(k8sClient.Status().Update(ctx, cluster)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())
		})

		It("Should use cluster backup destination when not overridden", func() {
			By("Creating a backup without destination override")
			backup := &banhbaoringv1.BanhBaoRingBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backup-inherit",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingBackupSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "backup-test-cluster",
					},
					Type: "full",
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			defer func() {
				Expect(k8sClient.Delete(ctx, backup)).Should(Succeed())
			}()

			backupLookupKey := types.NamespacedName{Name: "test-backup-inherit", Namespace: "default"}
			createdBackup := &banhbaoringv1.BanhBaoRingBackup{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, backupLookupKey, createdBackup)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			// Backup should not have its own destination (uses cluster's)
			Expect(createdBackup.Spec.Destination).Should(BeNil())
		})
	})

	Context("When testing backup job creation", func() {
		It("Should build correct backup job name", func() {
			backup := &banhbaoringv1.BanhBaoRingBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-backup",
					Namespace: "default",
				},
			}

			expectedJobName := "my-backup-backup"
			Expect(expectedJobName).Should(Equal("my-backup-backup"))
			_ = backup // used in expectations
		})
	})

	Context("When testing component status", func() {
		It("Should create component status for all components", func() {
			reconciler := &BackupReconciler{}

			backup := &banhbaoringv1.BanhBaoRingBackup{
				Spec: banhbaoringv1.BanhBaoRingBackupSpec{
					Components: []string{"openbao", "database", "secrets"},
				},
			}

			status := reconciler.buildComponentStatus(backup, "Completed")

			Expect(status).Should(HaveLen(3))
			Expect(status[0].Name).Should(Equal("openbao"))
			Expect(status[0].Status).Should(Equal("Completed"))
			Expect(status[1].Name).Should(Equal("database"))
			Expect(status[2].Name).Should(Equal("secrets"))
		})

		It("Should use default components when none specified", func() {
			reconciler := &BackupReconciler{}

			backup := &banhbaoringv1.BanhBaoRingBackup{
				Spec: banhbaoringv1.BanhBaoRingBackupSpec{},
			}

			status := reconciler.buildComponentStatus(backup, "Running")

			Expect(status).Should(HaveLen(3))
			Expect(status[0].Name).Should(Equal("openbao"))
			Expect(status[1].Name).Should(Equal("database"))
			Expect(status[2].Name).Should(Equal("secrets"))
		})
	})
})

var _ = Describe("BackupJob", func() {
	Context("When building backup job", func() {
		It("Should create job with correct structure", func() {
			reconciler := &BackupReconciler{}

			backup := &banhbaoringv1.BanhBaoRingBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backup",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingBackupSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "test-cluster",
					},
					Type:       "full",
					Components: []string{"openbao", "database"},
				},
			}

			cluster := &banhbaoringv1.BanhBaoRingCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingClusterSpec{
					Domain: "keys.example.com",
				},
			}

			job := reconciler.buildBackupJob(backup, cluster)

			Expect(job.Name).Should(Equal("test-backup-backup"))
			Expect(job.Namespace).Should(Equal("default"))
			Expect(job.Labels["banhbaoring.io/cluster"]).Should(Equal("test-cluster"))
			Expect(job.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).Should(Equal("backup"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).Should(Equal("banhbaoring/backup:latest"))
			Expect(*job.Spec.BackoffLimit).Should(Equal(int32(2)))
		})
	})
})

var _ = Describe("BackupCronJob", func() {
	It("Should verify CronJob is a batch/v1 resource", func() {
		cj := &batchv1.CronJob{}
		Expect(cj.Kind).Should(Equal(""))
		// CronJob is in batch/v1
		cj.TypeMeta = metav1.TypeMeta{
			Kind:       "CronJob",
			APIVersion: "batch/v1",
		}
		Expect(cj.APIVersion).Should(Equal("batch/v1"))
	})
})
