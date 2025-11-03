package e2e

import (
	"context"
	"os"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	controllers "github.com/argoproj-labs/argo-rollouts-manager/controllers"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/k8s"
	rolloutManagerFixture "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/rolloutmanager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Argo RolloutManager ImagePullPolicy E2E tests", func() {
	var (
		k8sClient      client.Client
		ctx            context.Context
		rolloutManager v1alpha1.RolloutManager
	)

	BeforeEach(func() {
		Expect(fixture.EnsureCleanSlate()).To(Succeed())

		var err error
		k8sClient, _, err = fixture.GetE2ETestKubeClient()
		Expect(err).ToNot(HaveOccurred())
		ctx = context.Background()
	})

	Context("ImagePullPolicy tests", func() {
		It("Verify imagePullPolicy is used from CR spec when specified", func() {
			By("creating a RolloutManager with imagePullPolicy set to Always")
			rolloutManager = v1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "imagepullpolicy-rollouts-manager",
					Namespace: fixture.TestE2ENamespace,
				},
				Spec: v1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
					ImagePullPolicy: corev1.PullAlways,
				},
			}

			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the deployment exists")
			deployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controllers.DefaultArgoRolloutsResourceName,
					Namespace: rolloutManager.Namespace,
				},
			}
			Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
			By("verifying the deployment has the correct imagePullPolicy Always")
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))

			By("changing the RolloutManager imagePullPolicy to Never")
			rolloutManager.Spec.ImagePullPolicy = corev1.PullNever

			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the deployment has the correct imagePullPolicy Never")
			Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullNever))

			By("changing the RolloutManager imagePullPolicy to IfNotPresent")
			rolloutManager.Spec.ImagePullPolicy = corev1.PullIfNotPresent

			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the deployment has the correct imagePullPolicy IfNotPresent")
			Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		})

		It("should use the environment variable IMAGE_PULL_POLICY when specified and check the precedence", func() {
			if fixture.EnvLocalRun() {
				Skip("This test does not support local run, as when the controller is running locally there is no env var to modify")
				return
			}
			By("adding image pull policy env variable to IMAGE_PULL_POLICY in Subscription")
			Expect(os.Setenv("IMAGE_PULL_POLICY", "Always")).To(Succeed())
			defer os.Unsetenv("IMAGE_PULL_POLICY")

			By("creating a RolloutManager without imagePullPolicy")
			rolloutManager = v1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "imagepullpolicy-rollouts-manager",
					Namespace: fixture.TestE2ENamespace,
				},
				Spec: v1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}

			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the deployment exists")
			deployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controllers.DefaultArgoRolloutsResourceName,
					Namespace: rolloutManager.Namespace,
				},
			}
			Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
			By("verifying the deployment has the correct imagePullPolicy Always")
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))

			By("updating the env var IMAGE_PULL_POLICY to Never")
			Expect(os.Setenv("IMAGE_PULL_POLICY", "Never")).To(Succeed())

			By("verifying the deployment has the correct imagePullPolicy Never")
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullNever))

			By("updating the env var IMAGE_PULL_POLICY to IfNotPresent")
			Expect(os.Setenv("IMAGE_PULL_POLICY", "IfNotPresent")).To(Succeed())

			By("verifying the deployment has the correct imagePullPolicy IfNotPresent")
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

			By("setting the imagePullPolicy to Always in the RolloutManager CR")
			rolloutManager.Spec.ImagePullPolicy = corev1.PullAlways

			Expect(k8sClient.Update(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the deployment has the correct imagePullPolicy Always")
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))

			By("changing the imagePullPolicy to Never in the RolloutManager CR")
			rolloutManager.Spec.ImagePullPolicy = corev1.PullNever

			Expect(k8sClient.Update(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the deployment has the correct imagePullPolicy Never")
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullNever))
		})

		It("should default to IfNotPresent when imagePullPolicy if not specified in either the CR spec or the environment variable", func() {
			By("creating a RolloutManager without imagePullPolicy")
			rolloutManager = v1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "imagepullpolicy-rollouts-manager",
					Namespace: fixture.TestE2ENamespace,
				},
				Spec: v1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}

			Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
			Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(v1alpha1.PhaseAvailable))

			By("verifying the deployment exists")
			deployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controllers.DefaultArgoRolloutsResourceName,
					Namespace: rolloutManager.Namespace,
				},
			}
			Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
			By("verifying the deployment has the correct imagePullPolicy IfNotPresent")
			Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		})

	})
})
