package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/k8s"
	rmFixture "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/rolloutmanager"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rmv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	controllers "github.com/argoproj-labs/argo-rollouts-manager/controllers"
	rv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Cluster Scoped RolloutManager tests", func() {

	Context("Testing cluster scoped RolloutManager behaviour", func() {

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
			In this test a cluster scoped RolloutManager is created in argo-rollouts namespace.
			After creation of RM operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in argo-rollouts namespace.
			Now when a Rollouts CR is created in a different namespace, operator should still be able to reconcile.
		*/
		It("After creating cluster scoped RolloutManager in default namespace i.e argo-rollouts, operator should create appropriate K8s resources and watch argo rollouts CR in different namespace.", func() {

			nsName := "test-ro-ns"
			labels := map[string]string{"app": "test-argo-app"}

			By("Create cluster scoped RolloutManager in default namespace.")
			rolloutsManager, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, false)
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
			validateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, false)

			By("Verify argo rollout controller able to reconcile CR of other namespace.")

			By("Create a different namespace.")
			Expect(createNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("Create and validate rollouts.")
			validateArgoRolloutsResources(ctx, k8sClient, nsName, labels, 31000, 32000)
		})

		/*
			In this test a cluster scoped RolloutManager is created in namespace other than argo-rollouts.
			After creation of RM operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in other namespace.
			Now when a Rollouts CR is created in a another namespace, operator should still be able to reconcile.
		*/
		It("After creating cluster scoped RolloutManager in namespace other than argo-rollouts, operator should create appropriate K8s resources and watch argo rollouts CR in another namespace.", func() {

			nsName1, nsName2 := "test-rom-ns", "test-ro-ns"
			labels := map[string]string{"app": "test-argo-app"}

			By("Create a different namespace for rollout manager.")
			Expect(createNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("Create cluster scoped RolloutManager.")
			rolloutsManager, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", nsName1, false)
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
			validateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, false)

			By("Verify argo rollout controller able to reconcile CR of other namespace.")

			By("Create a different namespace for rollout.")
			Expect(createNamespace(ctx, k8sClient, nsName2)).To(Succeed())

			By("Create and validate rollouts.")
			validateArgoRolloutsResources(ctx, k8sClient, nsName2, labels, 31000, 32000)
		})

		/*
			In this test a cluster scoped RolloutManager is created in a namespace.
			After creation of RM operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in namespace.
			Now when a Rollouts CR is created in multiple namespaces, operator should still be able to reconcile all of them.
		*/
		It("After creating cluster scoped RolloutManager in a namespace, operator should create appropriate K8s resources and watch argo rollouts CR in multiple namespace.", func() {

			nsName1, nsName2, nsName3 := "rom-ns-1", "ro-ns-1", "ro-ns-2"
			labels := map[string]string{"app": "test-argo-app"}

			By("Create a namespace for rollout manager.")
			Expect(createNamespace(ctx, k8sClient, nsName1)).To(Succeed())

			By("Create cluster scoped RolloutManager.")
			rolloutsManager, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", nsName1, false)
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
			validateArgoRolloutManagerResources(ctx, rolloutsManager, k8sClient, false)

			By("Verify argo rollout controller able to reconcile CR of multiple namespaces.")

			By("1st rollout: Create a different namespace for rollout.")
			Expect(createNamespace(ctx, k8sClient, nsName2)).To(Succeed())

			By("1st rollout: Create active and preview services in 1st namespace.")
			validateArgoRolloutsResources(ctx, k8sClient, nsName2, labels, 31000, 32000)

			By("2nd rollout: Create a another namespace for 2nd rollout.")
			Expect(createNamespace(ctx, k8sClient, nsName3)).To(Succeed())

			By("2nd rollout: Create active and preview services in 2nd namespace.")
			validateArgoRolloutsResources(ctx, k8sClient, nsName3, labels, 31001, 32002)
		})

		/*
			In this test a cluster scoped RolloutManager is created in a namespace.
			After creation of RM operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in namespace.
			Now when a namespace scoped RolloutManager is created, it should not be accepted by operator, since there in an existing RolloutManager watching entire cluster.
			When 1st cluster scoped RolloutManager is reconciled again it should also have error, because only one cluster scoped or all namespace scoped RolloutManagers are supported.
		*/
		It("After creating cluster scoped RolloutManager in a namespace, another namespace scoped RolloutManager should not be allowed.", func() {

			nsName := "test-ro-ns"

			By("Create cluster scoped RolloutManager in a namespace.")
			rolloutsManagerCl, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, false)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("Verify that Status.Condition is set.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("Create a different namespace.")
			Expect(createNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("Create namespace scoped RolloutManager in different namespace.")
			rolloutsManagerNs, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-2", nsName, true)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is not working.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("Verify that Status.Condition is having error message.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager,
					Message: controllers.UnsupportedRolloutManagerConfiguration,
				}))

			By("Update cluster scoped RolloutManager, after reconciliation it should also stop working.")

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManagerCl), &rolloutsManagerCl)).To(Succeed())
			rolloutsManagerCl.Spec.Env = append(rolloutsManagerCl.Spec.Env, corev1.EnvVar{Name: "test-name", Value: "test-value"})
			Expect(k8sClient.Update(ctx, &rolloutsManagerCl)).To(Succeed())

			By("Verify that now cluster scoped RolloutManager is also not working.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("Verify that Status.Condition is now having error message.")
			Eventually(rolloutsManagerCl, "3m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager,
					Message: controllers.UnsupportedRolloutManagerConfiguration,
				}))
		})

		/*
			In this test a cluster scoped RolloutManager is created in a namespace.
			After creation of RM operator should create required resources (ServiceAccount, ClusterRoles, ClusterRoleBinding, Service, Secret, Deployment) in namespace.
			Now when another cluster scoped RolloutManager is created, it should not be accepted by operator, since there in an existing RolloutManager watching entire cluster.
			When cluster scoped RolloutManager is reconciled again it should also have error, because only one cluster scoped or all namespace scoped RolloutManagers are supported.
		*/
		It("After creating cluster scoped RolloutManager in a namespace, another cluster scoped RolloutManager should not be allowed.", func() {

			nsName := "test-ro-ns"

			By("Create cluster scoped RolloutManager in a namespace.")
			rolloutsManagerCl, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, false)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("Verify that Status.Condition is set.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("Create a different namespace.")
			Expect(createNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("Create cluster scoped RolloutManager in different namespace.")
			rolloutsManagerNs, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-2", nsName, false)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is not working.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("Verify that Status.Condition is having error message.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager,
					Message: controllers.UnsupportedRolloutManagerConfiguration,
				}))

			By("Update first RolloutManager, after reconciliation it should also stop working.")

			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManagerCl), &rolloutsManagerCl)
			Expect(err).ToNot(HaveOccurred())
			rolloutsManagerCl.Spec.Env = append(rolloutsManagerCl.Spec.Env, corev1.EnvVar{Name: "test-name", Value: "test-value"})
			err = k8sClient.Update(ctx, &rolloutsManagerCl)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that now first RolloutManager is also not working.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("Verify that Status.Condition is now having error message.")
			Eventually(rolloutsManagerCl, "3m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager,
					Message: controllers.UnsupportedRolloutManagerConfiguration,
				}))
		})

		/*
			In this test a namespace scoped RolloutManager is created in a namespace.
			After creation of RM operator should create required resources (ServiceAccount, Roles, RoleBinding, Service, Secret, Deployment) in namespace.
			Now when a cluster scoped RolloutManager is created, it should not be accepted by operator, since there in an existing RolloutManager watching a namespace.
			When namespace scoped RolloutManager is reconciled again it should also have error, because only one cluster scoped or all namespace scoped RolloutManagers are supported.
		*/
		It("After creating namespace scoped RolloutManager, if a cluster scoped RolloutManager is created, both should not be allowed.", func() {

			nsName := "test-ro-ns"

			By("Create namespace scoped RolloutManager in a namespace.")
			rolloutsManagerNs, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-1", fixture.TestE2ENamespace, true)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseAvailable))

			By("Verify that Status.Condition is set.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  rmv1alpha1.RolloutManagerReasonSuccess,
					Message: "",
				}))

			By("Create a different namespace.")
			Expect(createNamespace(ctx, k8sClient, nsName)).To(Succeed())

			By("Create cluster scoped RolloutManager in different namespace.")
			rolloutsManagerCl, err := createRolloutManager(ctx, k8sClient, "test-rollouts-manager-2", nsName, false)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that RolloutManager is not working.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("Verify that Status.Condition is having error message.")
			Eventually(rolloutsManagerCl, "1m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager,
					Message: controllers.UnsupportedRolloutManagerConfiguration,
				}))

			By("Update namespace scoped RolloutManager, after reconciliation it should also stop working.")

			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManagerNs), &rolloutsManagerNs)
			Expect(err).ToNot(HaveOccurred())
			rolloutsManagerNs.Spec.Env = append(rolloutsManagerNs.Spec.Env, corev1.EnvVar{Name: "test-name", Value: "test-value"})
			err = k8sClient.Update(ctx, &rolloutsManagerNs)
			Expect(err).ToNot(HaveOccurred())

			By("Verify that now namespace scoped RolloutManager is also not working.")
			Eventually(rolloutsManagerNs, "1m", "1s").Should(rmFixture.HavePhase(rmv1alpha1.PhaseFailure))

			By("Verify that Status.Condition is now having error message.")
			Eventually(rolloutsManagerNs, "3m", "1s").Should(rmFixture.HaveCondition(
				metav1.Condition{
					Type:    rmv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionFalse,
					Reason:  rmv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager,
					Message: controllers.UnsupportedRolloutManagerConfiguration,
				}))
		})
	})
})

func createNamespace(ctx context.Context, k8sClient client.Client, name string) error {
	return k8sClient.Create(ctx,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{fixture.NamespaceLabelsKey: fixture.NamespaceLabelsValue},
		}})
}

func createRolloutManager(ctx context.Context, k8sClient client.Client, name, namespace string, namespaceScoped bool) (rmv1alpha1.RolloutManager, error) {
	rolloutsManager := rmv1alpha1.RolloutManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: rmv1alpha1.RolloutManagerSpec{
			NamespaceScoped: namespaceScoped,
		},
	}
	return rolloutsManager, k8sClient.Create(ctx, &rolloutsManager)
}

func createService(ctx context.Context, k8sClient client.Client, name, namespace string, nodePort int32, selector map[string]string) (corev1.Service, error) {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: selector,
			Ports: []corev1.ServicePort{
				{
					NodePort: nodePort,
					Protocol: corev1.ProtocolTCP,
					Port:     8080,
				},
			},
		},
	}
	return service, k8sClient.Create(ctx, &service)
}

func createArgoRollout(ctx context.Context, k8sClient client.Client, name, namespace, activeService, previewService string, labels map[string]string) (rv1alpha1.Rollout, error) {
	var num int32 = 2
	autoPromotionEnabled := false

	rollout := rv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: rv1alpha1.RolloutSpec{
			Replicas: &num,
			Strategy: rv1alpha1.RolloutStrategy{
				BlueGreen: &rv1alpha1.BlueGreenStrategy{
					ActiveService:        activeService,
					PreviewService:       previewService,
					AutoPromotionEnabled: &autoPromotionEnabled,
				},
			},
			RevisionHistoryLimit: &num,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "webserver-simple",
							Image: "docker.io/kostiscodefresh/gitops-canary-app:v1.0",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}
	return rollout, k8sClient.Create(ctx, &rollout)
}

func validateArgoRolloutManagerResources(ctx context.Context, rolloutsManager rmv1alpha1.RolloutManager, k8sClient client.Client, namespaceScoped bool) {

	By("Verify that ServiceAccount is created.")
	validateServiceAccount(k8sClient, rolloutsManager)

	if namespaceScoped {
		By("Verify that argo-rollout Role is created.")
		validateArgoRolloutsRole(k8sClient, rolloutsManager)
	} else {
		By("Verify that argo-rollout ClusterRoles is created.")
		validateArgoRolloutsClusterRole(k8sClient, rolloutsManager)
	}

	By("Verify that aggregate-to-admin ClusterRole is created.")
	validateAggregateToAdminClusterRole(k8sClient, rolloutsManager)

	By("Verify that aggregate-to-edit ClusterRole is created.")
	validateAggregateToEditClusterRole(k8sClient, rolloutsManager)

	By("Verify that aggregate-to-view ClusterRole is created.")
	validateAggregateToViewClusterRole(k8sClient, rolloutsManager)

	if namespaceScoped {
		By("Verify that RoleBinding is created.")
		validateRoleBinding(k8sClient, rolloutsManager)
	} else {
		By("Verify that ClusterRoleBinding is created.")
		validateClusterRoleBinding(k8sClient, rolloutsManager)
	}

	By("Verify that Service is created.")
	validateService(k8sClient, rolloutsManager)

	By("Verify that Secret is created.")
	validateSecret(k8sClient, rolloutsManager)

	By("Verify that argo rollouts deployment is created and it is in Ready state.")
	validateDeployment(ctx, k8sClient, rolloutsManager)
}

func validateArgoRolloutsResources(ctx context.Context, k8sClient client.Client, nsName string, labels map[string]string, port1, port2 int32) {

	By("Create active and preview services in new namespace")
	rolloutServiceActive, err := createService(ctx, k8sClient, "rollout-bluegreen-active", nsName, port1, labels)
	Expect(err).ToNot(HaveOccurred())
	Eventually(&rolloutServiceActive, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	rolloutServicePreview, err := createService(ctx, k8sClient, "rollout-bluegreen-preview", nsName, port2, labels)
	Expect(err).ToNot(HaveOccurred())
	Eventually(&rolloutServicePreview, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Create Argo Rollout CR in new namespace and check it is reconciled successfully.")
	rollout, err := createArgoRollout(ctx, k8sClient, "simple-rollout", nsName, rolloutServiceActive.Name, rolloutServicePreview.Name, labels)
	Expect(err).ToNot(HaveOccurred())
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&rollout), &rollout); err != nil {
			return false
		}
		return rollout.Status.Phase == rv1alpha1.RolloutPhaseHealthy
	}, "3m", "1s").Should(BeTrue())
}

func validateServiceAccount(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.DefaultArgoRolloutsResourceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(sa, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ServiceAccount has correct labels.")
	validateLabels(&sa.ObjectMeta)
}

func validateArgoRolloutsRole(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.DefaultArgoRolloutsResourceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(role, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that Role has correct labels.")
	validateLabels(&role.ObjectMeta)

	By("Verify that Role has correct policy rules.")
	Expect(role.Rules).To(ConsistOf(controllers.GetPolicyRules()))
}

func validateArgoRolloutsClusterRole(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllers.DefaultArgoRolloutsResourceName,
		},
	}
	Eventually(clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRole has correct labels.")
	validateLabels(&clusterRole.ObjectMeta)

	By("Verify that ClusterRole has correct policy rules.")
	Expect(clusterRole.Rules).To(ConsistOf(controllers.GetPolicyRules()))
}

func validateAggregateToAdminClusterRole(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {

	aggregationType := "aggregate-to-admin"
	clusterRoleName := fmt.Sprintf("%s-%s", controllers.DefaultArgoRolloutsResourceName, aggregationType)

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
		Name: clusterRoleName,
	},
	}
	Eventually(clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRole has correct labels.")
	validateAggregateLabels(&clusterRole.ObjectMeta, aggregationType)

	By("Verify that ClusterRole has correct policy rules.")
	Expect(clusterRole.Rules).To(ConsistOf(controllers.GetAggregateToAdminPolicyRules()))
}

func validateAggregateToEditClusterRole(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {

	aggregationType := "aggregate-to-edit"
	clusterRoleName := fmt.Sprintf("%s-%s", controllers.DefaultArgoRolloutsResourceName, aggregationType)

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
		Name: clusterRoleName,
	},
	}
	Eventually(clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRole has correct labels.")
	validateAggregateLabels(&clusterRole.ObjectMeta, aggregationType)

	By("Verify that ClusterRole has correct policy rules.")
	Expect(clusterRole.Rules).To(ConsistOf(controllers.GetAggregateToEditPolicyRules()))
}

func validateAggregateToViewClusterRole(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {

	aggregationType := "aggregate-to-view"
	clusterRoleName := fmt.Sprintf("%s-%s", controllers.DefaultArgoRolloutsResourceName, aggregationType)

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
		Name: clusterRoleName,
	},
	}
	Eventually(clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRole has correct labels.")
	validateAggregateLabels(&clusterRole.ObjectMeta, aggregationType)

	By("Verify that ClusterRole has correct policy rules.")
	Expect(clusterRole.Rules).To(ConsistOf(controllers.GetAggregateToViewPolicyRules()))
}

func validateRoleBinding(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.DefaultArgoRolloutsResourceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(binding, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that RoleBinding has correct labels.")
	validateLabels(&binding.ObjectMeta)

	By("Verify that RoleBinding has correct RoleRef.")
	Expect(binding.RoleRef).To(Equal(rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     controllers.DefaultArgoRolloutsResourceName,
	}))

	By("Verify that RoleBinding has correct Subjects.")
	Expect(binding.Subjects).To(Equal(
		[]rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      controllers.DefaultArgoRolloutsResourceName,
				Namespace: rolloutsManager.Namespace,
			},
		},
	))
}

func validateClusterRoleBinding(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: controllers.DefaultArgoRolloutsResourceName,
		},
	}
	Eventually(clusterRoleBinding, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRoleBinding has correct labels.")
	validateLabels(&clusterRoleBinding.ObjectMeta)

	By("Verify that ClusterRoleBinding has correct RoleRef.")
	Expect(clusterRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     controllers.DefaultArgoRolloutsResourceName,
	}))

	By("Verify that ClusterRoleBinding has correct Subjects.")
	Expect(clusterRoleBinding.Subjects).To(Equal(
		[]rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      controllers.DefaultArgoRolloutsResourceName,
				Namespace: rolloutsManager.Namespace,
			},
		},
	))
}

func validateService(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.DefaultArgoRolloutsMetricsServiceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(service, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that Service has correct labels.")
	Expect(service.Labels["app.kubernetes.io/name"]).To(Equal(controllers.DefaultArgoRolloutsMetricsServiceName))
	Expect(service.Labels["app.kubernetes.io/part-of"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
	Expect(service.Labels["app.kubernetes.io/component"]).To(Equal("server"))

	By("Verify that ClusterRoleBinding has correct Ports.")
	Expect(service.Spec.Ports).To(Equal([]corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8090,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8090),
		},
	}))

	By("Verify that ClusterRoleBinding has correct Ports.")
	Expect(service.Spec.Selector).To(Equal(map[string]string{
		controllers.DefaultRolloutsSelectorKey: controllers.DefaultArgoRolloutsResourceName,
	}))
}

func validateSecret(k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.DefaultRolloutsNotificationSecretName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(secret, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that Secret has correct Type.")
	Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
}

func validateDeployment(ctx context.Context, k8sClient client.Client, rolloutsManager rmv1alpha1.RolloutManager) {
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.DefaultArgoRolloutsResourceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(&depl, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that Deployment replica is in Ready state.")
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&depl), &depl); err != nil {
			return false
		}
		return depl.Status.ReadyReplicas == 1
	}, "3m", "1s").Should(BeTrue())

	By("Verify that Deployment has correct labels.")
	validateLabels(&depl.ObjectMeta)

	By("Verify that Deployment has correct Selector.")
	Expect(depl.Spec.Selector).To(Equal(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			controllers.DefaultRolloutsSelectorKey: controllers.DefaultArgoRolloutsResourceName,
		}}))

	By("Verify that Deployment Template has correct Template.")
	Expect(depl.Spec.Template.Labels).To(Equal(map[string]string{controllers.DefaultRolloutsSelectorKey: controllers.DefaultArgoRolloutsResourceName}))

	By("Verify that Deployment Template has correct NodeSelector.")
	Expect(depl.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"kubernetes.io/os": "linux"}))

	By("Verify that Deployment Template has correct SecurityContext.")
	Expect(*depl.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(BeTrue())

	By("Verify that Deployment Template has correct ServiceAccountName.")
	Expect(depl.Spec.Template.Spec.ServiceAccountName).To(Equal(controllers.DefaultArgoRolloutsResourceName))

	By("Verify that Deployment Template Container is not empty.")
	Expect(depl.Spec.Template.Spec.Containers[0]).ToNot(Equal(corev1.Container{}))
}

func validateLabels(object *metav1.ObjectMeta) {
	GinkgoHelper()
	Expect(len(object.Labels)).To(Equal(3))
	Expect(object.Labels["app.kubernetes.io/name"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
	Expect(object.Labels["app.kubernetes.io/part-of"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
	Expect(object.Labels["app.kubernetes.io/component"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
}

func validateAggregateLabels(object *metav1.ObjectMeta, aggregationType string) {
	GinkgoHelper()
	Expect(len(object.Labels)).To(Equal(4))
	Expect(object.Labels["app.kubernetes.io/name"]).To(Equal(object.Name))
	Expect(object.Labels["app.kubernetes.io/part-of"]).To(Equal(controllers.DefaultArgoRolloutsResourceName))
	Expect(object.Labels["app.kubernetes.io/component"]).To(Equal("aggregate-cluster-role"))
	Expect(object.Labels["rbac.authorization.k8s.io/"+aggregationType]).To(Equal("true"))
}
