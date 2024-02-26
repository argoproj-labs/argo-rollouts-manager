package e2e

import (
	"context"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/k8s"
	rmFixture "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/rolloutmanager"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rmv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	rv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Namespace Scoped RolloutManager tests", func() {

	Context("Testing namespace scoped RolloutManager behaviour", func() {

		var (
			err       error
			ctx       context.Context
			k8sClient client.Client
			scheme    *runtime.Scheme
		)

		BeforeEach(func() {
			Expect(fixture.EnsureCleanSlate()).To(Succeed())

			k8sClient, scheme, err = fixture.GetE2ETestKubeClient()
			Expect(err).ToNot(HaveOccurred())

			err = rv1alpha1.AddToScheme(scheme)
			Expect(err).ToNot(HaveOccurred())

			ctx = context.Background()
		})

		/*
			In this test a namespace scoped RolloutManager is created in a namespace.
			After creation of RM operator should create required resources (ServiceAccount, Roles, RoleBinding, Service, Secret, Deployment) in namespace.
			Now when a Rollouts CR is created in a same namespace, operator should be able to reconcile it.
		*/
		It("After creating namespace scoped RolloutManager in a namespace, operator should create appropriate K8s resources and watch argo rollouts CR in same namespace.", func() {

			nsName := "test-rom-ns"
			labels := map[string]string{"app": "test-argo-app"}

			By("Create a namespace for rollout manager.")
			Expect(createNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("Create namespace scoped RolloutManager in same namespace.")
			rolloutsManager, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", nsName, true)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManager, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("Verify that Status.Condition is set.")
			Eventually(rolloutsManager, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("Verify that expected resources are created.")
			validateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, true)

			By("Verify argo rollout controller able to reconcile CR.")

			By("Create and validate rollouts.")
			validateArgoRolloutsResources(ctx, k8sClient, nsName, labels, 31000, 32000)
		})

		/*
			In this test namespace scoped RolloutManagers are created in multiple namespaces.
			After creation of RMs operator should create required resources (ServiceAccount, Roles, RoleBinding, Service, Secret, Deployment) in each namespace.
			Now when a Rollouts CR is created in each namespace, operator should be able to reconcile all of them.
		*/
		It("After creating namespace scoped RolloutManager in a namespace, another namespace scoped RolloutManager should still be allowed.", func() {

			nsName := "test-rom-ns"

			By("1st RM: Create namespace scoped RolloutManager in 1st namespace.")
			rolloutsManagerNs1, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, true)
			Expect(err).ToNot(HaveOccurred())

			By("1st RM: Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManagerNs1, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("1st RM: Verify that Status.Condition is set.")
			Eventually(rolloutsManagerNs1, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("2nd RM: Create another namespace.")
			Expect(createNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("2nd RM: Create namespace scoped RolloutManager in 2nd namespace.")
			rolloutsManagerNs2, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-2", nsName, true)
			Expect(err).ToNot(HaveOccurred())

			By("2nd RM: Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManagerNs2, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("2nd RM: Verify that Status.Condition is having error message.")
			Eventually(rolloutsManagerNs2, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("1st RM: Update first RolloutManager, after reconciliation it should still work.")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManagerNs1), &rolloutsManagerNs1)).To(Succeed())
			rolloutsManagerNs1.Spec.Env = append(rolloutsManagerNs1.Spec.Env, corev1.EnvVar{Name: "test-name", Value: "test-value"})
			Expect(k8sClient.Update(ctx, &rolloutsManagerNs1)).To(Succeed())

			By("1st RM: Verify that now first RolloutManager is still working.")
			Eventually(rolloutsManagerNs1, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))
		})

		/*
			In this a namespace scoped RolloutManager is created in a namespaces.
			After creation of RMs operator should create required resources (ServiceAccount, Roles, RoleBinding, Service, Secret, Deployment) in namespace.
			Now when a Rollouts CR is created in another namespace (which doesn't have RM), operator should not reconcile it.
		*/
		It("After creating namespace scoped RolloutManager in a namespace, operator should create appropriate K8s resources but it should not watch argo rollouts CR in other namespace.", func() {

			nsName1, nsName2 := "test-rom-ns", "test-ro-ns"
			labels := map[string]string{"app": "test-argo-app"}

			By("Create a namespace for rollout manager.")
			Expect(createNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("Create namespace scoped RolloutManager in namespace.")
			rolloutsManager, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", nsName1, true)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManager, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("Verify that Status.Condition is set.")
			Eventually(rolloutsManager, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("Verify that expected resources are created.")

			validateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, true)

			By("Verify argo rollout controller is not able to reconcile CR from different namespace.")

			By("Create another namespace for rollout manager.")
			Expect(createNamespace(ctx, k8sClient, nsName2)).To(Succeed())

			By("Create active and preview services in new namespace.")
			rolloutServiceActive, err := createService(ctx, k8sClient, "rollout-bluegreen-active", nsName2, 31000, labels)
			Expect(err).ToNot(HaveOccurred())
			Eventually(&rolloutServiceActive, "10s", "1s").Should(k8s.ExistByName(k8sClient))

			rolloutServicePreview, err := createService(ctx, k8sClient, "rollout-bluegreen-preview", nsName2, 32000, labels)
			Expect(err).ToNot(HaveOccurred())
			Eventually(&rolloutServicePreview, "10s", "1s").Should(k8s.ExistByName(k8sClient))

			By("Create Argo Rollout CR in a different namespace and verify that it is not reconciled.")
			rollout, err := createArgoRollout(ctx, k8sClient, "simple-rollout", nsName2, rolloutServiceActive.Name, rolloutServicePreview.Name, labels)
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&rollout), &rollout); err != nil {
					return false
				}
				return reflect.DeepEqual(rollout.Status, rv1alpha1.RolloutStatus{})
			}, "1m", "1s").Should(
				BeTrue(),
			)
		})
	})
})
