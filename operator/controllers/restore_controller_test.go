package controllers

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	banhbaoringv1 "github.com/Bidon15/banhbaoring/operator/api/v1"
)

var _ = Describe("RestoreController", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a BanhBaoRingRestore", func() {
		It("Should fail when parent cluster is not found", func() {
			By("Creating a restore without a parent cluster")
			restore := &banhbaoringv1.BanhBaoRingRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore-no-cluster",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingRestoreSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "non-existent-cluster",
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).Should(Succeed())

			// Clean up
			defer func() {
				Expect(k8sClient.Delete(ctx, restore)).Should(Succeed())
			}()

			restoreLookupKey := types.NamespacedName{Name: "test-restore-no-cluster", Namespace: "default"}
			createdRestore := &banhbaoringv1.BanhBaoRingRestore{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, restoreLookupKey, createdRestore)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})

		It("Should create restore with backup reference", func() {
			By("Creating a restore with backup reference")
			restore := &banhbaoringv1.BanhBaoRingRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore-with-backup",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingRestoreSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "test-cluster",
					},
					BackupRef: &banhbaoringv1.BackupReference{
						Name: "my-backup",
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).Should(Succeed())

			// Clean up
			defer func() {
				Expect(k8sClient.Delete(ctx, restore)).Should(Succeed())
			}()

			restoreLookupKey := types.NamespacedName{Name: "test-restore-with-backup", Namespace: "default"}
			createdRestore := &banhbaoringv1.BanhBaoRingRestore{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, restoreLookupKey, createdRestore)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdRestore.Spec.BackupRef).ShouldNot(BeNil())
			Expect(createdRestore.Spec.BackupRef.Name).Should(Equal("my-backup"))
		})

		It("Should create restore with direct source", func() {
			By("Creating a restore with direct S3 source")
			restore := &banhbaoringv1.BanhBaoRingRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore-direct",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingRestoreSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "test-cluster",
					},
					Source: &banhbaoringv1.BackupDestination{
						S3: &banhbaoringv1.S3Destination{
							Bucket: "restore-bucket",
							Region: "us-west-2",
							Prefix: "backups/20241210-020000/",
							Credentials: banhbaoringv1.SecretKeyRef{
								Name: "aws-credentials",
								Key:  "credentials",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).Should(Succeed())

			// Clean up
			defer func() {
				Expect(k8sClient.Delete(ctx, restore)).Should(Succeed())
			}()

			restoreLookupKey := types.NamespacedName{Name: "test-restore-direct", Namespace: "default"}
			createdRestore := &banhbaoringv1.BanhBaoRingRestore{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, restoreLookupKey, createdRestore)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdRestore.Spec.Source).ShouldNot(BeNil())
			Expect(createdRestore.Spec.Source.S3).ShouldNot(BeNil())
			Expect(createdRestore.Spec.Source.S3.Bucket).Should(Equal("restore-bucket"))
		})

		It("Should create restore with custom options", func() {
			By("Creating a restore with custom options")
			restore := &banhbaoringv1.BanhBaoRingRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore-options",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingRestoreSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "test-cluster",
					},
					BackupRef: &banhbaoringv1.BackupReference{
						Name: "my-backup",
					},
					Components: []string{"openbao"},
					Options: banhbaoringv1.RestoreOptions{
						StopApplications: true,
						VerifyBackup:     true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).Should(Succeed())

			// Clean up
			defer func() {
				Expect(k8sClient.Delete(ctx, restore)).Should(Succeed())
			}()

			restoreLookupKey := types.NamespacedName{Name: "test-restore-options", Namespace: "default"}
			createdRestore := &banhbaoringv1.BanhBaoRingRestore{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, restoreLookupKey, createdRestore)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdRestore.Spec.Options.StopApplications).Should(BeTrue())
			Expect(createdRestore.Spec.Options.VerifyBackup).Should(BeTrue())
			Expect(createdRestore.Spec.Components).Should(HaveLen(1))
			Expect(createdRestore.Spec.Components[0]).Should(Equal("openbao"))
		})
	})

	Context("When testing restore steps", func() {
		It("Should initialize correct restore steps", func() {
			reconciler := &RestoreReconciler{}

			steps := reconciler.initializeSteps()

			Expect(steps).Should(HaveLen(3))
			Expect(steps[0].Name).Should(Equal("stop-applications"))
			Expect(steps[0].Status).Should(Equal("Pending"))
			Expect(steps[1].Name).Should(Equal("restore-data"))
			Expect(steps[1].Status).Should(Equal("Pending"))
			Expect(steps[2].Name).Should(Equal("start-applications"))
			Expect(steps[2].Status).Should(Equal("Pending"))
		})

		It("Should update step status correctly", func() {
			reconciler := &RestoreReconciler{}

			restore := &banhbaoringv1.BanhBaoRingRestore{
				Status: banhbaoringv1.BanhBaoRingRestoreStatus{
					Steps: []banhbaoringv1.RestoreStep{
						{Name: "stop-applications", Status: "Pending"},
						{Name: "restore-data", Status: "Pending"},
						{Name: "start-applications", Status: "Pending"},
					},
				},
			}

			reconciler.updateStep(restore, "stop-applications", "Running")
			Expect(restore.Status.Steps[0].Status).Should(Equal("Running"))

			reconciler.updateStep(restore, "stop-applications", "Completed")
			Expect(restore.Status.Steps[0].Status).Should(Equal("Completed"))

			reconciler.updateStep(restore, "restore-data", "Running")
			Expect(restore.Status.Steps[1].Status).Should(Equal("Running"))
		})
	})

	Context("When restore has a ready cluster", func() {
		var cluster *banhbaoringv1.BanhBaoRingCluster

		BeforeEach(func() {
			By("Creating a parent cluster for restore tests")
			cluster = &banhbaoringv1.BanhBaoRingCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "restore-test-cluster",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingClusterSpec{
					Domain: "keys.example.com",
					API: banhbaoringv1.APISpec{
						Replicas: 2,
					},
					Dashboard: banhbaoringv1.DashboardSpec{
						Replicas: 2,
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

		It("Should create restore for the cluster", func() {
			By("Creating a restore for the ready cluster")
			restore := &banhbaoringv1.BanhBaoRingRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore-ready",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingRestoreSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "restore-test-cluster",
					},
					BackupRef: &banhbaoringv1.BackupReference{
						Name: "latest-backup",
					},
					Options: banhbaoringv1.RestoreOptions{
						StopApplications: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).Should(Succeed())

			defer func() {
				Expect(k8sClient.Delete(ctx, restore)).Should(Succeed())
			}()

			restoreLookupKey := types.NamespacedName{Name: "test-restore-ready", Namespace: "default"}
			createdRestore := &banhbaoringv1.BanhBaoRingRestore{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, restoreLookupKey, createdRestore)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdRestore.Spec.ClusterRef.Name).Should(Equal("restore-test-cluster"))
		})
	})
})

var _ = Describe("RestoreJob", func() {
	Context("When building restore job", func() {
		It("Should create job with correct structure", func() {
			reconciler := &RestoreReconciler{}

			restore := &banhbaoringv1.BanhBaoRingRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore",
					Namespace: "default",
				},
				Spec: banhbaoringv1.BanhBaoRingRestoreSpec{
					ClusterRef: banhbaoringv1.ClusterReference{
						Name: "test-cluster",
					},
					Components: []string{"openbao", "database"},
				},
			}

			script := reconciler.buildRestoreScript(restore)

			Expect(script).Should(ContainSubstring("Restoring OpenBao"))
			Expect(script).Should(ContainSubstring("Restoring PostgreSQL"))
			Expect(script).Should(ContainSubstring("vault operator raft snapshot restore"))
		})
	})

	Context("When testing restore phases", func() {
		It("Should recognize valid restore phases", func() {
			validPhases := []string{"Pending", "Stopping", "Restoring", "Starting", "Completed", "Failed"}

			for _, phase := range validPhases {
				Expect(phase).Should(BeElementOf(validPhases))
			}
		})
	})
})
