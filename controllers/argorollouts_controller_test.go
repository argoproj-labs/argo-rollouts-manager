package rollouts

import (
	"context"
	"fmt"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("RolloutManagerReconciler tests", func() {
	var (
		ctx context.Context
		rm  *rolloutsmanagerv1alpha1.RolloutManager
	)

	BeforeEach(func() {
		ctx = context.Background()
		rm = makeTestRolloutManager()
	})

	It("Should create expected resource when namespace scoped RolloutManager CR is reconciled.", func() {
		// Make RolloutManager namespace scoped
		rm.Spec.NamespaceScoped = true

		r := makeTestReconciler(rm)
		Expect(createNamespace(r, rm.Namespace)).To(Succeed())

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      rm.Name,
				Namespace: rm.Namespace,
			},
		}

		res, err := r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).Should(BeFalse(), "reconcile should not requeue request")

		By("Check if RolloutManager's Status.Conditions are set.")
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: rm.Name, Namespace: rm.Namespace}, rm)).To(Succeed())
		Expect(rm.Status.Conditions[0].Type == rolloutsmanagerv1alpha1.RolloutManagerConditionType &&
			rm.Status.Conditions[0].Reason == rolloutsmanagerv1alpha1.RolloutManagerReasonSuccess &&
			rm.Status.Conditions[0].Message == "" &&
			rm.Status.Conditions[0].Status == metav1.ConditionTrue).To(BeTrue())

		By("Check expected resources are created.")
		validateArgoRolloutManagerResources(ctx, rm, r.Client, true)

		By("Check ClusterRole and ClusterRoleBinding are not created.")

		clusterRole := &rbacv1.ClusterRole{}
		err = r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: testNamespace,
		}, clusterRole)
		Expect(errors.IsNotFound(err)).To(BeTrue(), "ClusterRole should not be created")

		clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		err = r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: testNamespace,
		}, clusterRoleBinding)
		Expect(errors.IsNotFound(err)).To(BeTrue(), "ClusterRoleBinding should not be created")
	})

	It("Should create expected resource when cluster scoped RolloutManager CR is reconciled.", func() {
		r := makeTestReconciler(rm)
		Expect(createNamespace(r, rm.Namespace)).To(Succeed())

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      rm.Name,
				Namespace: rm.Namespace,
			},
		}

		res, err := r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).Should(BeFalse(), "reconcile should not requeue request")

		By("Check if RolloutManager's Status.Conditions are set.")
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: rm.Name, Namespace: rm.Namespace}, rm)).To(Succeed())
		Expect(rm.Status.Conditions[0].Type == rolloutsmanagerv1alpha1.RolloutManagerConditionType &&
			rm.Status.Conditions[0].Reason == rolloutsmanagerv1alpha1.RolloutManagerReasonSuccess &&
			rm.Status.Conditions[0].Message == "" &&
			rm.Status.Conditions[0].Status == metav1.ConditionTrue).To(BeTrue())

		By("Check expected resources are created.")
		validateArgoRolloutManagerResources(ctx, rm, r.Client, false)

		By("Check Role and RoleBinding are not created.")

		role := &rbacv1.Role{}
		err = r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: testNamespace,
		}, role)
		Expect(errors.IsNotFound(err)).To(BeTrue(), "Role should not be created")

		roleBinding := &rbacv1.RoleBinding{}
		err = r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: testNamespace,
		}, roleBinding)
		Expect(errors.IsNotFound(err)).To(BeTrue(), "RoleBinding should not be created")
	})

	It("Should not allow cluster and namespace scoped RolloutManager CRs together.", func() {
		r := makeTestReconciler(rm)
		Expect(createNamespace(r, rm.Namespace)).To(Succeed())

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      rm.Name,
				Namespace: rm.Namespace,
			},
		}

		res, err := r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).Should(BeFalse(), "reconcile should not requeue request")

		By("Check if RolloutManager's Status.Conditions are set.")
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: rm.Name, Namespace: rm.Namespace}, rm)).To(Succeed())
		Expect(rm.Status.Conditions[0].Type == rolloutsmanagerv1alpha1.RolloutManagerConditionType &&
			rm.Status.Conditions[0].Reason == rolloutsmanagerv1alpha1.RolloutManagerReasonSuccess &&
			rm.Status.Conditions[0].Message == "" &&
			rm.Status.Conditions[0].Status == metav1.ConditionTrue).To(BeTrue())

		rm2 := makeTestRolloutManager()
		rm2.Name = "test-rm"
		rm2.Namespace = "test-ns"

		r2 := makeTestReconciler(rm)

		Expect(createNamespace(r2, rm2.Namespace)).To(Succeed())
		Expect(r.Client.Create(ctx, rm2)).To(Succeed())

		req2 := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      rm2.Name,
				Namespace: rm2.Namespace,
			},
		}

		res2, err := r.Reconcile(ctx, req2)
		Expect(err).ToNot(HaveOccurred())
		Expect(res2.Requeue).Should(BeFalse(), "reconcile should not requeue request")

		By("Check if RolloutManager's Status.Conditions are set.")
		Expect(r.Client.Get(ctx, types.NamespacedName{Name: rm2.Name, Namespace: rm2.Namespace}, rm2)).To(Succeed())
		Expect(rm2.Status.Conditions[0].Type == rolloutsmanagerv1alpha1.RolloutManagerConditionType &&
			rm2.Status.Conditions[0].Reason == rolloutsmanagerv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager &&
			rm2.Status.Conditions[0].Message == UnsupportedRolloutManagerConfiguration &&
			rm2.Status.Conditions[0].Status == metav1.ConditionFalse).To(BeTrue())
	})
})

func validateArgoRolloutManagerResources(ctx context.Context, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager, k8sClient client.Client, namespaceScoped bool) {

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

	By("Verify that argo rollouts Deployment is created and it is in Ready state.")
	validateDeployment(ctx, k8sClient, rolloutsManager)
}

func validateServiceAccount(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(sa, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ServiceAccount has correct labels.")
	ensureLabels(&sa.ObjectMeta)
}

func validateArgoRolloutsRole(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(role, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that Role has correct labels.")
	ensureLabels(&role.ObjectMeta)

	By("Verify that Role has correct policy rules.")
	Expect(role.Rules).To(ConsistOf(GetPolicyRules()))
}

func validateArgoRolloutsClusterRole(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultArgoRolloutsResourceName,
		},
	}
	Eventually(clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRole has correct labels.")
	ensureLabels(&clusterRole.ObjectMeta)

	By("Verify that ClusterRole has correct policy rules.")
	Expect(clusterRole.Rules).To(ConsistOf(GetPolicyRules()))
}

func validateAggregateToAdminClusterRole(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {

	aggregationType := "aggregate-to-admin"
	clusterRoleName := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
		Name: clusterRoleName},
	}
	Eventually(clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRole has correct labels.")
	ensureAggregateLabels(&clusterRole.ObjectMeta, aggregationType)

	By("Verify that ClusterRole has correct policy rules.")
	Expect(clusterRole.Rules).To(ConsistOf(GetAggregateToAdminPolicyRules()))
}

func validateAggregateToEditClusterRole(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {

	aggregationType := "aggregate-to-edit"
	clusterRoleName := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
		Name: clusterRoleName,
	},
	}
	Eventually(clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRole has correct labels.")
	ensureAggregateLabels(&clusterRole.ObjectMeta, aggregationType)

	By("Verify that ClusterRole has correct policy rules.")
	Expect(clusterRole.Rules).To(ConsistOf(GetAggregateToEditPolicyRules()))
}

func validateAggregateToViewClusterRole(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {

	aggregationType := "aggregate-to-view"
	clusterRoleName := fmt.Sprintf("%s-%s", DefaultArgoRolloutsResourceName, aggregationType)

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
		Name: clusterRoleName,
	},
	}
	Eventually(clusterRole, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRole has correct labels.")
	ensureAggregateLabels(&clusterRole.ObjectMeta, aggregationType)

	By("Verify that ClusterRole has correct policy rules.")
	Expect(clusterRole.Rules).To(ConsistOf(GetAggregateToViewPolicyRules()))
}

func validateRoleBinding(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(binding, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that RoleBinding has correct labels.")
	ensureLabels(&binding.ObjectMeta)

	By("Verify that RoleBinding has correct RoleRef.")
	Expect(binding.RoleRef).To(Equal(rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     DefaultArgoRolloutsResourceName,
	}))

	By("Verify that RoleBinding has correct Subjects.")
	Expect(binding.Subjects).To(Equal(
		[]rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: rolloutsManager.Namespace,
			},
		},
	))
}

func validateClusterRoleBinding(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultArgoRolloutsResourceName,
		},
	}
	Eventually(clusterRoleBinding, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that ClusterRoleBinding has correct labels.")
	ensureLabels(&clusterRoleBinding.ObjectMeta)

	By("Verify that ClusterRoleBinding has correct RoleRef.")
	Expect(clusterRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     DefaultArgoRolloutsResourceName,
	}))

	By("Verify that ClusterRoleBinding has correct Subjects.")
	Expect(clusterRoleBinding.Subjects).To(Equal(
		[]rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: rolloutsManager.Namespace,
			},
		},
	))
}

func validateService(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsMetricsServiceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(service, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that Service has correct labels.")
	Expect(service.Labels["app.kubernetes.io/name"]).To(Equal(DefaultArgoRolloutsMetricsServiceName))
	Expect(service.Labels["app.kubernetes.io/part-of"]).To(Equal(DefaultArgoRolloutsResourceName))
	Expect(service.Labels["app.kubernetes.io/component"]).To(Equal("server"))

	By("Verify that Service has correct Ports.")
	Expect(service.Spec.Ports).To(Equal([]corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8090,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8090),
		},
	}))

	By("Verify that Service has correct Selector.")
	Expect(service.Spec.Selector).To(Equal(map[string]string{
		DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName,
	}))
}

func validateSecret(k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultRolloutsNotificationSecretName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(secret, "30s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that Secret has correct Type.")
	Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
}

func validateDeployment(ctx context.Context, k8sClient client.Client, rolloutsManager *rolloutsmanagerv1alpha1.RolloutManager) {
	depl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: rolloutsManager.Namespace,
		},
	}
	Eventually(&depl, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Verify that Deployment has correct labels.")
	ensureLabels(&depl.ObjectMeta)

	By("Verify that Deployment has correct Selector.")
	Expect(depl.Spec.Selector).To(Equal(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName,
		}}))

	By("Verify that Deployment Template has correct Template.")
	Expect(depl.Spec.Template.Labels).To(Equal(map[string]string{DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName}))

	By("Verify that Deployment Template has correct NodeSelector.")
	Expect(depl.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"kubernetes.io/os": "linux"}))

	By("Verify that Deployment Template has correct SecurityContext.")
	Expect(*depl.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(BeTrue())

	By("Verify that Deployment Template has correct ServiceAccountName.")
	Expect(depl.Spec.Template.Spec.ServiceAccountName).To(Equal(DefaultArgoRolloutsResourceName))

	By("Verify that Deployment Template Container is not empty.")
	Expect(depl.Spec.Template.Spec.Containers[0]).ToNot(Equal(corev1.Container{}))
}

func ensureLabels(object *metav1.ObjectMeta) {
	GinkgoHelper()
	Expect(len(object.Labels)).To(Equal(3))
	Expect(object.Labels["app.kubernetes.io/name"]).To(Equal(DefaultArgoRolloutsResourceName))
	Expect(object.Labels["app.kubernetes.io/part-of"]).To(Equal(DefaultArgoRolloutsResourceName))
	Expect(object.Labels["app.kubernetes.io/component"]).To(Equal(DefaultArgoRolloutsResourceName))
}

func ensureAggregateLabels(object *metav1.ObjectMeta, aggregationType string) {
	GinkgoHelper()
	Expect(len(object.Labels)).To(Equal(4))
	Expect(object.Labels["app.kubernetes.io/name"]).To(Equal(object.Name))
	Expect(object.Labels["app.kubernetes.io/part-of"]).To(Equal(DefaultArgoRolloutsResourceName))
	Expect(object.Labels["app.kubernetes.io/component"]).To(Equal("aggregate-cluster-role"))
	Expect(object.Labels["rbac.authorization.k8s.io/"+aggregationType]).To(Equal("true"))
}
