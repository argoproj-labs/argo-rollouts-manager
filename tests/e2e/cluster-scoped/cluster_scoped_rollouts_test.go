package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	utils "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	rmFixture "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/rolloutmanager"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rmv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	controllers "github.com/argoproj-labs/argo-rollouts-manager/controllers"
	rv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Cluster-scoped RolloutManager tests", func() {

	Context("Testing cluster-scoped RolloutManager behaviour", func() {

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
			In this test a cluster-scoped RolloutManager is created in argo-rollouts namespace.
			Then operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in argo-rollouts namespace.
			Now when a Rollout CR is created in a different namespace, operator should still be able to reconcile it.
		*/
		It("After creating cluster-scoped RolloutManager in default namespace i.e argo-rollouts, operator should create appropriate K8s resources and watch argo rollouts CR in different namespace.", func() {

			nsName := "test-ro-ns"

			By("Create cluster-scoped RolloutManager in default namespace.")
			rolloutsManager, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, false)
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
			utils.ValidateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, false)

			By("Verify argo Rollouts controller is able to reconcile CR of other namespace.")

			By("Create a different namespace.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("Create and validate rollouts.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName, 31000, 32000)
		})

		/*
			In this test a cluster-scoped RolloutManager is created in namespace other than argo-rollouts.
			Then operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in other namespace.
			Now when a Rollouts CR is created in a another namespace, operator should still be able to reconcile it.
		*/
		It("After creating cluster-scoped RolloutManager in namespace other than argo-rollouts, operator should create appropriate K8s resources and watch argo rollouts CR in another namespace.", func() {

			nsName1, nsName2 := "test-rom-ns", "test-ro-ns"

			By("Create a different namespace for RolloutManager.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("Create cluster-scoped RolloutManager.")
			rolloutsManager, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", nsName1, false)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManager, "3m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("Verify that Status.Condition is set.")
			Eventually(rolloutsManager, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("Verify that expected resources are created.")
			utils.ValidateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, false)

			By("Verify argo Rollouts controller is able to reconcile CR of other namespace.")

			By("Create a different namespace for Rollout.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName2)).To(Succeed())

			By("Create and validate Rollout.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName2, 31000, 32000)
		})

		/*
			In this test a cluster-scoped RolloutManager is created in a namespace.
			Then operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in one namespace.
			Now when a Rollouts CR is created in multiple namespaces, operator should still be able to reconcile all of them.
		*/
		It("After creating cluster-scoped RolloutManager in a namespace, operator should create appropriate K8s resources and watch argo rollouts CR in multiple namespace.", func() {

			nsName1, nsName2, nsName3 := "rom-ns-1", "ro-ns-1", "ro-ns-2"

			By("Create a namespace for RolloutManager.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("Create cluster-scoped RolloutManager.")
			rolloutsManager, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", nsName1, false)
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
			utils.ValidateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, false)

			By("Verify argo Rollouts controller is able to reconcile CR of multiple namespaces.")

			By("1st Namespace: Create a different namespace for Rollout.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName2)).To(Succeed())

			By("1st Namespace: Create and validate Rollout in 1st namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName2, 31000, 32000)

			By("2nd Namespace: Create a another namespace for 2nd Rollout.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName3)).To(Succeed())

			By("2nd Namespace: Create and validate Rollout in 2nd namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName3, 31001, 32002)
		})

		/*
			In this test a cluster-scoped RolloutManager is created in a namespace.
			Then operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in namespace.
			Now when a namespace-scoped RolloutManager is created in 2nd namespace, it should not be accepted by operator,
			since NAMESPACE_SCOPED_ARGO_ROLLOUTS is set to false, it means user can only create cluster-scoped RolloutManagers,
			but Rollouts controller of 1st namespace should still be able to reconcile Rollout CR of 2nd namespace and failed RolloutManager of 2nd namespace should not cause any issues.
			When 1st cluster-scoped RolloutManager is reconciled again it should still work, and should be able to reconcile Rollout CR created in any namespace.
		*/
		It("Should allow cluster-scoped RolloutManager, but not namespace-scoped.", func() {

			nsName1, nsName2 := "test-rom-ns-1", "test-ro-ns-2"

			By("1st RM: Create cluster-scoped RolloutManager in a namespace.")
			rolloutsManagerCl, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, false)
			Expect(err).ToNot(HaveOccurred())

			By("1st RM: Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("1st RM: Verify that Status.Condition is set.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("1st RM: Create and validate Rollout in 1st namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, fixture.TestE2ENamespace, 31000, 32000)

			By("2nd RM: Create 2nd namespace.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("2nd RM: Create namespace-scoped RolloutManager in 2nd namespace.")
			rolloutsManagerNs, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-2", nsName1, true)
			Expect(err).ToNot(HaveOccurred())

			By("2nd RM: Verify that RolloutManager is not working.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("2nd RM: Verify that Status.Condition is having error message.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonInvalidScoped,
					Message: controllers.UnsupportedRolloutManagerNamespaceScoped,
				}))

			By("2nd RM: Create and validate Rollout in 2nd namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName1, 31001, 32002)

			By("1st RM: Update cluster-scoped RolloutManager, after reconciliation it should still work.")

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManagerCl), &rolloutsManagerCl)).To(Succeed())
			rolloutsManagerCl.Spec.Env = append(rolloutsManagerCl.Spec.Env, corev1.EnvVar{Name: "test-name", Value: "test-value"})
			Expect(k8sClient.Update(ctx, &rolloutsManagerCl)).To(Succeed())

			By("1st RM: Verify that cluster-scoped RolloutManager is still working.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("1st RM: Verify that Status.Condition is not having error message.")
			Eventually(rolloutsManagerCl, "3m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("3rd RM: Create 3rd namespace.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName2)).To(Succeed())

			By("2nd RM: Create and validate Rollout in 3rd namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName2, 31002, 32003)
		})

		/*
			In this test a cluster-scoped RolloutManager is created in a namespace.
			Then operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in namespace.
			Now when another cluster-scoped RolloutManager is created in 2nd namespace, it should not be accepted by operator,
			since there in an existing RolloutManager watching entire cluster,
			but Rollouts controller of 1st namespace should still be able to reconcile Rollout CR of 2nd namespace and failed RolloutManager of 2nd namespace should not cause any issues.
			When cluster-scoped RolloutManager is reconciled again it should also have error, because only one cluster-scoped or all namespace-scoped RolloutManagers are supported.
			but Rollouts controller deployed in 1st namespace should still work and reconcile Rollout CR created in any namespace,
			it means even both cluster-scoped RolloutManagers are having error in status, but Rollout controller created before error occurred should still work normally.
		*/
		It("After creating cluster-scoped RolloutManager in a namespace, another cluster-scoped RolloutManager should not be allowed.", func() {

			nsName1, nsName2 := "test-rom-ns-1", "test-ro-ns-2"

			By("1st RM: Create cluster-scoped RolloutManager in 1st namespace.")
			rolloutsManagerCl, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, false)
			Expect(err).ToNot(HaveOccurred())

			By("1st RM: Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("1st RM: Verify that Status.Condition is set.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("1st RM: Create and validate Rollout in 1st namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, fixture.TestE2ENamespace, 31000, 32000)

			By("2nd RM: Create 2nd namespace.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("2nd RM: Create cluster-scoped RolloutManager in 2nd namespace.")
			rolloutsManagerCl2, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-2", nsName1, false)
			Expect(err).ToNot(HaveOccurred())

			By("2nd RM: Verify that RolloutManager is not working.")
			Eventually(rolloutsManagerCl2, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("2nd RM: Verify that Status.Condition is having error message.")
			Eventually(rolloutsManagerCl2, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager,
					Message: controllers.UnsupportedRolloutManagerConfiguration,
				}))

			By("2nd RM: Create and validate Rollout in 2nd namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName1, 31001, 32001)

			By("1st RM: Update first RolloutManager, after reconciliation it should also stop working.")

			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManagerCl), &rolloutsManagerCl)
			Expect(err).ToNot(HaveOccurred())
			rolloutsManagerCl.Spec.Env = append(rolloutsManagerCl.Spec.Env, corev1.EnvVar{Name: "test-name", Value: "test-value"})
			err = k8sClient.Update(ctx, &rolloutsManagerCl)
			Expect(err).ToNot(HaveOccurred())

			By("1st RM: Verify that now first RolloutManager is also not working.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("1st RM: Verify that Status.Condition is now having error message.")
			Eventually(rolloutsManagerCl, "3m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager,
					Message: controllers.UnsupportedRolloutManagerConfiguration,
				}))

			By("1st RM: Create 3rd namespace.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName2)).To(Succeed())

			By("1st RM: Create and validate Rollout in 3rd namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName2, 31002, 32002)
		})
	})
})
