package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/k8s"
	rolloutManagerFixture "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/rolloutmanager"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	controllers "github.com/argoproj-labs/argo-rollouts-manager/controllers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("RolloutManager tests", func() {

	Context("RolloutManager tests", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		When("Reconcile is called on a new, basic, namespaced-scoped RolloutManager", func() {

			BeforeEach(func() {
				Expect(fixture.EnsureCleanSlate()).To(Succeed())

				var err error
				k8sClient, err = fixture.GetE2ETestKubeClient()
				Expect(err).ToNot(HaveOccurred())
				ctx = context.Background()

			})

			It("should create the appropriate K8s resources", func() {

				rolloutsManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-rollouts-manager",
						Namespace: fixture.TestE2ENamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{},
				}

				Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

				Eventually(rolloutsManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				Eventually(&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutsManager.Namespace},
				}, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				Eventually(&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutsManager.Namespace},
				}, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				Eventually(&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutsManager.Namespace},
				}, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				clusterRoles := []string{"argo-rollouts-aggregate-to-admin", "argo-rollouts-aggregate-to-edit", "argo-rollouts-aggregate-to-view"}

				for _, clusterRole := range clusterRoles {
					Eventually(&rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{Name: clusterRole},
					}, "30s", "1s").Should(k8s.ExistByName(k8sClient))
				}

				Eventually(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutsManager.Namespace},
				}, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				Eventually(&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsMetricsServiceName, Namespace: rolloutsManager.Namespace},
				}, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				Eventually(func() bool {

					depl := appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutsManager.Namespace},
					}
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&depl), &depl); err != nil {
						return false
					}

					return depl.Status.ReadyReplicas == 1

				}, "120s", "1s").Should(BeTrue())

				Eventually(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsNotificationSecretName, Namespace: rolloutsManager.Namespace},
				}, "30s", "1s").Should(k8s.ExistByName(k8sClient))

			})

			It("should create a Role in the namespace containing all required RBAC permissions", func() {

				rolloutsManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-rollouts-manager",
						Namespace: fixture.TestE2ENamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{},
				}

				Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

				Eventually(rolloutsManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				role := rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: fixture.TestE2ENamespace},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&role), &role)).To(Succeed())

				Expect(role.Rules).To(ConsistOf(controllers.GetPolicyRules()))

			})
		})
	})
})
