package e2e

import (
	"context"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	utils "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/k8s"
	rmFixture "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/rolloutmanager"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rmv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	controllers "github.com/argoproj-labs/argo-rollouts-manager/controllers"
	rv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Namespace-scoped RolloutManager tests", func() {

	Context("Testing namespace-scoped RolloutManager behaviour", func() {

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
			In this test a namespace-scoped RolloutManager is created in a namespace.
			Then operator should create required resources (ServiceAccount, Roles, RoleBinding, Service, Secret, Deployment) in namespace.
			Now when a Rollouts CR is created in same namespace, operator should be able to reconcile it.
		*/
		It("After creating namespace-scoped RolloutManager in a namespace, operator should create appropriate K8s resources and watch argo rollouts CR in same namespace.", func() {

			nsName := "test-rom-ns"

			By("Create a namespace for RolloutManager.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("Create namespace-scoped RolloutManager in same namespace.")
			rolloutsManager, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", nsName, true)
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
			utils.ValidateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, true)

			By("Verify argo Rollouts controller is able to reconcile CR.")

			By("Create and validate rollouts.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName, 31000, 32000)
		})

		/*
			In this test namespace-scoped RolloutManagers are created in multiple namespaces.
			Then operator should create required resources (ServiceAccount, Roles, RoleBinding, Service, Secret, Deployment) in each namespace.
			Now when a Rollouts CR is created in each namespace, Rollouts controller running in respective should be able to reconcile them.
			When RolloutManagers is deleted in any namespace it also deletes Rollouts controller from that namespace.
			After that if any Rollout CR is created in that namespace, it should not be reconciled.
		*/
		It("After creating namespace-scoped RolloutManager in a namespace, another namespace-scoped RolloutManager in different namespace should also work.", func() {

			nsName1 := "test-rom-ns"

			By("1st RM: Create namespace-scoped RolloutManager in 1st namespace.")
			rolloutsManagerNs1, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, true)
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

			By("1st RM: Verify argo Rollouts controller of 1st namespace is able to reconcile CR created in 1st namespace.")

			By("1st RM: Create and validate rollouts.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, fixture.TestE2ENamespace, 31000, 32000)

			By("2nd RM: Create 2nd namespace.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("2nd RM: Create 2nd namespace-scoped RolloutManager in 2nd namespace.")
			rolloutsManagerNs2, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-2", nsName1, true)
			Expect(err).ToNot(HaveOccurred())

			By("2nd RM: Verify that RolloutManager of 2nd namespace is successfully created.")
			Eventually(rolloutsManagerNs2, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("2nd RM: Verify that Status.Condition is set.")
			Eventually(rolloutsManagerNs2, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("2nd RM: Verify argo Rollouts controller of 2nd namespace is able to reconcile CR created in 2nd namespace.")

			By("2nd RM: Create and validate rollouts in 2nd namespace.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, nsName1, 31001, 32001)

			By("1st RM: Update 1st RolloutManager, after reconciliation it should still work.")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManagerNs1), &rolloutsManagerNs1)).To(Succeed())
			rolloutsManagerNs1.Spec.Env = append(rolloutsManagerNs1.Spec.Env, corev1.EnvVar{Name: "test-name", Value: "test-value"})
			Expect(k8sClient.Update(ctx, &rolloutsManagerNs1)).To(Succeed())

			By("1st RM: Verify that now 1st RolloutManager is still working.")
			Eventually(rolloutsManagerNs1, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("2nd RM: Delete 2nd RolloutManager and ensure 2nd Rollouts controller is also deleted.")
			Expect(k8sClient.Delete(ctx, &rolloutsManagerNs2)).To(Succeed())
			Consistently(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controllers.DefaultArgoRolloutsResourceName,
					Namespace: nsName1,
				},
			}, "30s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

			By("2nd RM: Delete 2nd Rollout CR and ensure it is not recreated.")
			rollout2 := rv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{Name: utils.RolloutsName, Namespace: nsName1},
			}
			Expect(k8sClient.Delete(ctx, &rollout2)).To(Succeed())
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(&rollout2), &rollout2)
			}, "30s", "1s").ShouldNot(BeNil())

			By("2nd RM: Create 3rd Rollout in 2nd namespace and ensure it is not reconciled, since RolloutsManager is deleted from 2nd namespace.")
			rollout3, err := utils.CreateArgoRollout(ctx, k8sClient, "simple-rollout-1", nsName1, utils.RolloutsActiveServiceName, utils.RolloutsPreviewServiceName)
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&rollout3), &rollout3); err != nil {
					return false
				}
				return reflect.DeepEqual(rollout3.Status, rv1alpha1.RolloutStatus{})
			}, "30s", "1s").Should(
				BeTrue(),
			)
		})

		/*
			In this a namespace-scoped RolloutManager is created in a namespaces.
			Then operator should create required resources (ServiceAccount, Roles, RoleBinding, Service, Secret, Deployment) in namespace.
			Now when a Rollouts CR is created in another namespace (which doesn't have RolloutsManagers), operator should not reconcile it.
		*/
		It("After creating namespace-scoped RolloutManager in a namespace, operator should create appropriate K8s resources, but it should not watch argo rollouts CR in other namespace.", func() {

			nsName1, nsName2 := "test-rom-ns", "test-ro-ns"

			By("1st NS: Create a namespace for RolloutManager.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("1st NS: Create namespace-scoped RolloutManager in 1st namespace.")
			rolloutsManager, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", nsName1, true)
			Expect(err).ToNot(HaveOccurred())

			By("1st NS: Verify that RolloutManager is successfully created in 1st namespace.")
			Eventually(rolloutsManager, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("1st NS: Verify that Status.Condition is set.")
			Eventually(rolloutsManager, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("1st NS: Verify that expected resources are created in 1st namespace.")
			utils.ValidateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, true)

			By("2nd NS: Verify argo Rollouts controller of 1st namespace is not able to reconcile CR from a different namespace.")

			By("2nd NS: Create 2nd namespace for RolloutManager.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName2)).To(Succeed())

			By("2nd NS: Create active and preview services required for Rollout CR in 2nd namespace.")
			rolloutServiceActive, err := utils.CreateService(ctx, k8sClient, utils.RolloutsActiveServiceName, nsName2, 31000)
			Expect(err).ToNot(HaveOccurred())
			Eventually(&rolloutServiceActive, "10s", "1s").Should(k8s.ExistByName(k8sClient))

			rolloutServicePreview, err := utils.CreateService(ctx, k8sClient, utils.RolloutsPreviewServiceName, nsName2, 32000)
			Expect(err).ToNot(HaveOccurred())
			Eventually(&rolloutServicePreview, "10s", "1s").Should(k8s.ExistByName(k8sClient))

			By("2nd NS: Create Argo Rollout CR in a 2nd namespace and verify that it is not reconciled by Rollouts controller of 1st namespace.")
			rollout, err := utils.CreateArgoRollout(ctx, k8sClient, utils.RolloutsName, nsName2, rolloutServiceActive.Name, rolloutServicePreview.Name)
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

		/*
			In this test a namespace-scoped RolloutManager is created in a namespace.
			Then operator should create required resources (ServiceAccount, Roles, RoleBinding, Service, Secret, Deployment) in namespace.
			Now when a cluster-scoped RolloutManager is created, it should not be accepted by operator,
			since NAMESPACE_SCOPED_ARGO_ROLLOUTS is set to true, it means user can only create namespace-scoped RolloutManagers.
			but namespace-scoped Rollouts controller of 1st namespace should still be able to reconcile Rollout CR of 1st namespace and failed cluster-scoped RolloutManager of 2nd namespace should not cause any issues.
			When namespace-scoped RolloutManager is reconciled again it should still work, and should be able to reconcile Rollout CR created in 1st namespace.
		*/
		It("Should allow namespace-scoped RolloutManager, but not cluster-scoped.", func() {

			nsName := "test-rom-ns"

			By("1st RM: Create namespace-scoped RolloutManager in 1st namespace.")
			rolloutsManagerNs, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, true)
			Expect(err).ToNot(HaveOccurred())

			By("1st RM: Verify that RolloutManager is successfully created in 1st namespace.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("1st RM: Verify that Status.Condition is set.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("2nd RM: Create 2nd namespace.")
			Expect(utils.CreateNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("2nd RM: Create cluster-scoped RolloutManager in 2nd namespace.")
			rolloutsManagerCl, err := utils.CreateRolloutManager(ctx, k8sClient, "test-rollouts-manager-2", nsName, false)
			Expect(err).ToNot(HaveOccurred())

			By("2nd RM: Verify that RolloutManager is not working in 2nd namespace.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("2nd RM: Verify that Status.Condition is having error message.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonInvalidScoped,
					Message: controllers.UnsupportedRolloutManagerClusterScoped,
				}))

			By("1st RM: Update namespace-scoped RolloutManager of 1st namespace, after reconciliation it should still be working.")
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManagerNs), &rolloutsManagerNs)
			Expect(err).ToNot(HaveOccurred())
			rolloutsManagerNs.Spec.Env = append(rolloutsManagerNs.Spec.Env, corev1.EnvVar{Name: "test-name", Value: "test-value"})
			err = k8sClient.Update(ctx, &rolloutsManagerNs)
			Expect(err).ToNot(HaveOccurred())
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("1st RM: Verify that Status.Condition is not having any error.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("1st RM: Create Rollout CR in 1st namespace and ensure it is reconciled.")
			utils.ValidateArgoRolloutsResources(ctx, k8sClient, fixture.TestE2ENamespace, 31000, 32000)

			By("2nd RM: Create Rollout in 2nd namespace and it should not be reconciled as 2nd RolloutManager failed.")

			By("2nd RM: Create active and preview services in 2nd namespace.")
			rolloutServiceActive, err := utils.CreateService(ctx, k8sClient, utils.RolloutsActiveServiceName, nsName, 31001)
			Expect(err).ToNot(HaveOccurred())
			Eventually(&rolloutServiceActive, "10s", "1s").Should(k8s.ExistByName(k8sClient))

			rolloutServicePreview, err := utils.CreateService(ctx, k8sClient, utils.RolloutsPreviewServiceName, nsName, 32002)
			Expect(err).ToNot(HaveOccurred())
			Eventually(&rolloutServicePreview, "10s", "1s").Should(k8s.ExistByName(k8sClient))

			By("2nd RM: Create Argo Rollout CR in 2nd namespace and verify that it is not reconciled.")
			rollout, err := utils.CreateArgoRollout(ctx, k8sClient, "simple-rollout-1", nsName, rolloutServiceActive.Name, rolloutServicePreview.Name)
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&rollout), &rollout); err != nil {
					return false
				}
				return reflect.DeepEqual(rollout.Status, rv1alpha1.RolloutStatus{})
			}, "30s", "1s").Should(
				BeTrue(),
			)
		})
	})
})
