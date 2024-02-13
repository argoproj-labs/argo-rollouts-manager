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
			k8sClient      client.Client
			ctx            context.Context
			rolloutManager rolloutsmanagerv1alpha1.RolloutManager
		)

		BeforeEach(func() {
			Expect(fixture.EnsureCleanSlate()).To(Succeed())

			var err error
			k8sClient, err = fixture.GetE2ETestKubeClient()
			Expect(err).ToNot(HaveOccurred())
			ctx = context.Background()

			rolloutManager = rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-rollouts-manager",
					Namespace: fixture.TestE2ENamespace,
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{},
			}
		})

		When("Reconcile is called on a new, basic, namespaced-scoped RolloutManager", func() {
			It("should create the appropriate K8s resources", func() {
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())

				By("setting the phase to \"Available\"")
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				By("creating a service account")
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&sa, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				By("ensuring the service account has the correct labels")
				ensureLabels(&sa.ObjectMeta)

				By("creating a role")
				role := rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&role, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				By("ensuring the role has the correct labels")
				ensureLabels(&role.ObjectMeta)

				By("ensuring the role has the correct policy rules")
				Expect(role.Rules).To(ConsistOf(controllers.GetPolicyRules()))

				By("creating a role binding")
				binding := rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&binding, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				By("ensuring the role binding has the correct labels")
				ensureLabels(&binding.ObjectMeta)

				By("creating three cluster roles")
				clusterRoleSuffixes := []string{"aggregate-to-admin", "aggregate-to-edit", "aggregate-to-view"}
				for _, suffix := range clusterRoleSuffixes {
					clusterRoleName := "argo-rollouts-" + suffix
					clusterRole := rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
					}
					Eventually(&clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

					Expect(len(binding.Labels)).To(Equal(3))
					Expect(clusterRole.Labels["app.kubernetes.io/name"]).To(Equal(clusterRoleName))
					Expect(clusterRole.Labels["app.kubernetes.io/part-of"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
					Expect(clusterRole.Labels["app.kubernetes.io/component"]).To(Equal("aggregate-cluster-role"))
					Expect(clusterRole.Labels["rbac.authorization.k8s.io/"+suffix]).To(Equal("true"))
				}

				By("creating a deployment")
				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				By("ensuring the deployment has the correct labels")
				ensureLabels(&deployment.ObjectMeta)

				By("creating a service")
				service := corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsMetricsServiceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&service, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				By("ensuring the service has the correct labels")
				Expect(service.Labels["app.kubernetes.io/name"]).To(Equal(controllers.DefaultArgoRolloutsMetricsServiceName))
				Expect(service.Labels["app.kubernetes.io/part-of"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
				Expect(service.Labels["app.kubernetes.io/component"]).To(Equal("server"))

				By("having the deployment become ready")
				Eventually(func() bool {

					depl := appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
					}
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&depl), &depl); err != nil {
						return false
					}

					return depl.Status.ReadyReplicas == 1

				}, "120s", "1s").Should(BeTrue())

				By("creating a secret")
				Eventually(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsNotificationSecretName, Namespace: rolloutManager.Namespace},
				}, "30s", "1s").Should(k8s.ExistByName(k8sClient))
			})
		})

		When("A RolloutManager is deleted", func() {
			It("should delete all the associated resources", func() {
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				Expect(k8sClient.Delete(ctx, &rolloutManager)).To(Succeed())

				By("deleting the service account")
				Eventually(&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				By("deleting the role")
				Eventually(&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				By("deleting the role binding")
				Eventually(&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				By("deleting the deployment")
				Eventually(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				By("deleting the service")
				Eventually(&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsMetricsServiceName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				By("deleting the secret")
				Eventually(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsNotificationSecretName, Namespace: rolloutManager.Namespace},
				}, "30s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				// Make sure the cluster roles have not been deleted
				By("NOT deleting the three cluster roles")
				clusterRoleSuffixes := []string{"aggregate-to-admin", "aggregate-to-edit", "aggregate-to-view"}
				for _, suffix := range clusterRoleSuffixes {
					clusterRoleName := "argo-rollouts-" + suffix
					Consistently(&rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
					}, "5s", "1s").Should(k8s.ExistByName(k8sClient))
				}
			})
		})

		When("A RolloutManager specifies an extra argument", func() {
			It("should reflect that argument in the deployment", func() {
				By("creating the deployment with the argument from the RolloutManager")
				rolloutManager.Spec = rolloutsmanagerv1alpha1.RolloutManagerSpec{
					ExtraCommandArgs: []string{
						"--loglevel",
						"error",
					},
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
				Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(Equal([]string{"--namespaced", "--loglevel", "error"}))

				By("updating the deployment when the argument in the RolloutManager is updated")
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutManager), &rolloutManager)).To(Succeed())
				rolloutManager.Spec = rolloutsmanagerv1alpha1.RolloutManagerSpec{
					ExtraCommandArgs: []string{
						"--logformat",
						"text",
					},
				}
				Expect(k8sClient.Update(ctx, &rolloutManager)).To(Succeed())
				Eventually(func() []string {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].Args
				}, "10s", "1s").Should(Equal([]string{"--namespaced", "--logformat", "text"}))
			})
		})

		When("A RolloutManager specifies environment variables", func() {
			It("should reflect those variables in the deployment", func() {
				By("creating the deployment with the environment variables specified in the RolloutManager")
				rolloutManager.Spec = rolloutsmanagerv1alpha1.RolloutManagerSpec{
					Env: []corev1.EnvVar{
						{Name: "EDITOR", Value: "emacs"},
						{Name: "LANG", Value: "en_CA.UTF-8"},
					},
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(SatisfyAll(
					HaveLen(2),
					ContainElements(
						corev1.EnvVar{Name: "EDITOR", Value: "emacs"},
						corev1.EnvVar{Name: "LANG", Value: "en_CA.UTF-8"},
					),
				))

				By("updating the deployment when the environment variables in the RolloutManager are updated")
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutManager), &rolloutManager)).To(Succeed())
				rolloutManager.Spec = rolloutsmanagerv1alpha1.RolloutManagerSpec{
					Env: []corev1.EnvVar{
						{Name: "LANG", Value: "en_US.UTF-8"},
						{Name: "TERM", Value: "xterm-256color"},
					},
				}
				Expect(k8sClient.Update(ctx, &rolloutManager)).To(Succeed())
				Eventually(func() []corev1.EnvVar {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].Env
				}, "10s", "1s").Should(SatisfyAll(
					HaveLen(2),
					ContainElements(
						corev1.EnvVar{Name: "LANG", Value: "en_US.UTF-8"},
						corev1.EnvVar{Name: "TERM", Value: "xterm-256color"},
					),
				))
			})
		})

		When("A RolloutManager specifies an image", func() {
			It("should reflect that image in the deployment", func() {
				By("creating the deployment with the image specified in the RolloutManager")
				rolloutManager.Spec = rolloutsmanagerv1alpha1.RolloutManagerSpec{
					Image:   "quay.io/prometheus/busybox",
					Version: "latest",
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())

				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
				expectedVersion := rolloutManager.Spec.Image + ":" + rolloutManager.Spec.Version
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
				Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(expectedVersion))

				By("updating the deployment when the image in the RolloutManager is updated")
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutManager), &rolloutManager)).To(Succeed())
				rolloutManager.Spec = rolloutsmanagerv1alpha1.RolloutManagerSpec{
					Image:   controllers.DefaultArgoRolloutsImage,
					Version: controllers.DefaultArgoRolloutsVersion,
				}
				Expect(k8sClient.Update(ctx, &rolloutManager)).To(Succeed())
				expectedVersion = controllers.DefaultArgoRolloutsImage + ":" + controllers.DefaultArgoRolloutsVersion
				Eventually(func() string {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].Image
				}, "10s", "1s").Should(Equal(expectedVersion))
			})
		})
	})
})

func ensureLabels(object *metav1.ObjectMeta) {
	GinkgoHelper()
	Expect(len(object.Labels)).To(Equal(3))
	Expect(object.Labels["app.kubernetes.io/name"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
	Expect(object.Labels["app.kubernetes.io/part-of"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
	Expect(object.Labels["app.kubernetes.io/component"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
}
