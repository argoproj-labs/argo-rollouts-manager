package e2e

import (
	"context"
	"os"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	controllers "github.com/argoproj-labs/argo-rollouts-manager/controllers"
	utils "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	rolloutManagerFixture "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/rolloutmanager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Argo RolloutManager ImagePullSecret E2E tests", func() {
	var (
		k8sClient   client.Client
		ctx         context.Context
		operatorNS  string
		rmNamespace string
	)

	BeforeEach(func() {
		Expect(fixture.EnsureCleanSlate()).To(Succeed())

		var err error
		k8sClient, _, err = fixture.GetE2ETestKubeClient()
		Expect(err).ToNot(HaveOccurred())
		ctx = context.Background()

		// OPERATOR_NAMESPACE controls where the controller looks for labeled pull secrets.
		// Default must match hack/start-rollouts-manager-for-e2e-tests.sh and hack/run-rollouts-manager-e2e-tests.sh.
		operatorNS = os.Getenv("OPERATOR_NAMESPACE")
		if operatorNS == "" {
			operatorNS = "argo-rollouts"
		}

		// RM lives in its own namespace so the cross-namespace copy path is always exercised.
		rmNamespace = "test-rom-ns-ips"
		Expect(operatorNS).ToNot(Equal(rmNamespace))
		Expect(utils.CreateNamespace(ctx, k8sClient, rmNamespace)).To(Succeed())

		// EnsureCleanSlate recreates fixture.TestE2ENamespace; any other operator namespace must be ensured here.
		if operatorNS != fixture.TestE2ENamespace {
			ns := &corev1.Namespace{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: operatorNS}, ns)
			if apierrors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: operatorNS},
				})).To(Succeed())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		}

		// Delete only the secrets created by these tests (by name) so we don't
		// disturb unrelated secrets that may exist in operatorNS.
		for _, name := range []string{
			"test-pull-secret",
			"test-pull-secret-removal",
			"pull-secret-a",
			"pull-secret-b",
			"conflict-pull-secret",
			"data-change-secret",
		} {
			s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: operatorNS}}
			_ = k8sClient.Delete(ctx, s)
		}
	})

	Context("ImagePullSecret propagation", func() {

		It("should add a labeled pull secret to the rollouts SA", func() {
			By("creating a pull secret with the propagate label in the operator namespace")
			pullSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pull-secret",
					Namespace: operatorNS,
					Labels: map[string]string{
						controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			}
			Expect(k8sClient.Create(ctx, pullSecret)).To(Succeed())

			By("creating a RolloutManager")
			rolloutManager := v1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "imagepullsecret-rm",
					Namespace: rmNamespace,
				},
				Spec: v1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the secret is copied to the RM namespace")
			copiedSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-pull-secret", Namespace: rmNamespace}, copiedSecret)
			}, "15s", "1s").Should(Succeed())
			Expect(copiedSecret.Labels[controllers.RolloutManagerImagePullSecretCopiedLabel]).To(Equal("test-pull-secret"))
			Expect(copiedSecret.Data).To(Equal(pullSecret.Data))

			By("verifying the rollouts SA has imagePullSecrets set")
			sa := &corev1.ServiceAccount{}
			Eventually(func() []corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
				return sa.ImagePullSecrets
			}, "15s", "1s").Should(ContainElement(corev1.LocalObjectReference{Name: "test-pull-secret"}))
		})

		It("should remove the SA imagePullSecrets when the label is removed", func() {
			By("creating a pull secret with the propagate label")
			pullSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pull-secret-removal",
					Namespace: operatorNS,
					Labels: map[string]string{
						controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			}
			Expect(k8sClient.Create(ctx, pullSecret)).To(Succeed())

			By("creating a RolloutManager")
			rolloutManager := v1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "imagepullsecret-removal-rm",
					Namespace: rmNamespace,
				},
				Spec: v1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("waiting for SA to have imagePullSecrets")
			sa := &corev1.ServiceAccount{}
			Eventually(func() []corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
				return sa.ImagePullSecrets
			}, "15s", "1s").Should(ContainElement(corev1.LocalObjectReference{Name: "test-pull-secret-removal"}))

			By("removing the propagate label from the source secret")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pullSecret), pullSecret)).To(Succeed())
			delete(pullSecret.Labels, controllers.RolloutManagerImagePullSecretPropagateLabel)
			Expect(k8sClient.Update(ctx, pullSecret)).To(Succeed())

			By("verifying SA imagePullSecrets is cleared")
			Eventually(func() []corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
				return sa.ImagePullSecrets
			}, "30s", "1s").ShouldNot(ContainElement(corev1.LocalObjectReference{Name: "test-pull-secret-removal"}))

			By("verifying the copied secret is deleted from the RM namespace")
			copiedSecret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-pull-secret-removal", Namespace: rmNamespace}, copiedSecret)
				return apierrors.IsNotFound(err)
			}, "30s", "1s").Should(BeTrue())
		})

		It("should not propagate when multiple secrets have the propagation label (ambiguous)", func() {
			By("creating two pull secrets with the propagate label in the operator namespace")
			for _, name := range []string{"pull-secret-a", "pull-secret-b"} {
				Expect(k8sClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: operatorNS,
						Labels: map[string]string{
							controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
						},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
					},
				})).To(Succeed())
			}

			By("creating a RolloutManager")
			rolloutManager := v1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "imagepullsecret-multi-rm",
					Namespace: rmNamespace,
				},
				Spec: v1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying SA has no imagePullSecrets (ambiguous state, operator skips propagation)")
			sa := &corev1.ServiceAccount{}
			Consistently(func() []corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
				return sa.ImagePullSecrets
			}, "10s", "2s").Should(BeEmpty())

			By("verifying no copies were created in the RM namespace")
			for _, name := range []string{"pull-secret-a", "pull-secret-b"} {
				s := &corev1.Secret{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: rmNamespace}, s)
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected secret %s not to exist in RM namespace", name)
			}

			By("removing one label so only one secret remains labeled")
			secretA := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "pull-secret-a", Namespace: operatorNS}, secretA)).To(Succeed())
			delete(secretA.Labels, controllers.RolloutManagerImagePullSecretPropagateLabel)
			Expect(k8sClient.Update(ctx, secretA)).To(Succeed())

			By("verifying SA now has the remaining pull secret")
			Eventually(func() []corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
				return sa.ImagePullSecrets
			}, "15s", "1s").Should(ConsistOf(corev1.LocalObjectReference{Name: "pull-secret-b"}))

			By("verifying only pull-secret-b is copied to the RM namespace")
			copiedSecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "pull-secret-b", Namespace: rmNamespace}, copiedSecret)).To(Succeed())
			Expect(copiedSecret.Labels[controllers.RolloutManagerImagePullSecretCopiedLabel]).To(Equal("pull-secret-b"))
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "pull-secret-a", Namespace: rmNamespace}, &corev1.Secret{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected pull-secret-a not to be copied to RM namespace")
		})

		It("should not overwrite an existing unmanaged secret with the same name", func() {
			By("creating an unmanaged secret with the same name in the RM namespace")
			preExisting := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conflict-pull-secret",
					Namespace: rmNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"key": []byte("original")},
			}
			Expect(k8sClient.Create(ctx, preExisting)).To(Succeed())

			By("creating a labeled pull secret with the same name in the operator namespace")
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conflict-pull-secret",
					Namespace: operatorNS,
					Labels: map[string]string{
						controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			})).To(Succeed())

			By("creating a RolloutManager")
			rolloutManager := v1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "conflict-rm",
					Namespace: rmNamespace,
				},
				Spec: v1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the pre-existing secret is not overwritten")
			existing := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "conflict-pull-secret", Namespace: rmNamespace}, existing)).To(Succeed())
			Expect(existing.Data["key"]).To(Equal([]byte("original")))
			Expect(existing.Labels).ToNot(HaveKey(controllers.RolloutManagerImagePullSecretCopiedLabel))

			By("verifying the SA does not include the conflicting secret name")
			sa := &corev1.ServiceAccount{}
			Consistently(func() []corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
				return sa.ImagePullSecrets
			}, "10s", "2s").ShouldNot(ContainElement(corev1.LocalObjectReference{Name: "conflict-pull-secret"}))
		})

		It("should update the copied secret when source pull secret data changes", func() {
			By("creating a pull secret")
			pullSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-change-secret",
					Namespace: operatorNS,
					Labels: map[string]string{
						controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			}
			Expect(k8sClient.Create(ctx, pullSecret)).To(Succeed())

			By("creating a RolloutManager")
			rolloutManager := v1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "data-change-rm",
					Namespace: rmNamespace,
				},
				Spec: v1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("waiting for SA to have imagePullSecrets")
			sa := &corev1.ServiceAccount{}
			Eventually(func() []corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
				return sa.ImagePullSecrets
			}, "15s", "1s").Should(ContainElement(corev1.LocalObjectReference{Name: "data-change-secret"}))

			By("updating the pull secret data")
			newData := []byte(`{"auths":{"registry2.example.com":{"auth":"bmV3cGFzcw=="}}}`)
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pullSecret), pullSecret)).To(Succeed())
			pullSecret.Data[corev1.DockerConfigJsonKey] = newData
			Expect(k8sClient.Update(ctx, pullSecret)).To(Succeed())

			By("verifying the copied secret data is updated")
			copiedSecret := &corev1.Secret{}
			Eventually(func() []byte {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: "data-change-secret", Namespace: rmNamespace}, copiedSecret)
				return copiedSecret.Data[corev1.DockerConfigJsonKey]
			}, "15s", "1s").Should(Equal(newData))
		})

		It("should reference the source secret directly when the RM is in the operator namespace", func() {
			By("creating a pull secret with the propagate label in the operator namespace")
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pull-secret",
					Namespace: operatorNS,
					Labels: map[string]string{
						controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
				},
			})).To(Succeed())

			By("creating a RolloutManager in the operator namespace")
			rolloutManager, err := utils.CreateRolloutManager(ctx, k8sClient, "same-ns-rm", operatorNS, true)
			Expect(err).ToNot(HaveOccurred())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the rollouts SA references the source secret")
			sa := &corev1.ServiceAccount{}
			Eventually(func() []corev1.LocalObjectReference {
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: operatorNS}, sa)
				return sa.ImagePullSecrets
			}, "15s", "1s").Should(ContainElement(corev1.LocalObjectReference{Name: "test-pull-secret"}))

			By("verifying no copy is created in the operator namespace")
			copies := &corev1.SecretList{}
			Expect(k8sClient.List(ctx, copies, client.InNamespace(operatorNS),
				client.HasLabels{controllers.RolloutManagerImagePullSecretCopiedLabel})).To(Succeed())
			Expect(copies.Items).To(BeEmpty())
		})
	})

})
