package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/k8s"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rmv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	controllers "github.com/argoproj-labs/argo-rollouts-manager/controllers"
	rv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DefaultOpenShiftRoutePluginURL = "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-openshift/releases/download/commit-2749e0ac96ba00ce6f4af19dc6d5358048227d77/rollouts-plugin-trafficrouter-openshift-linux-amd64"
	RolloutsActiveServiceName      = "rollout-bluegreen-active"
	RolloutsPreviewServiceName     = "rollout-bluegreen-preview"
	RolloutsName                   = "simple-rollout"
)

// Create namespace for tests having a specific label for identification
func CreateNamespace(ctx context.Context, k8sClient client.Client, name string) error {
	return k8sClient.Create(ctx,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: fixture.NamespaceLabels,
		}})
}

// Create RolloutManager CR
func CreateRolloutManager(ctx context.Context, k8sClient client.Client, name, namespace string, namespaceScoped bool) (rmv1alpha1.RolloutManager, error) {
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

// Create Service used by Rollout
func CreateService(ctx context.Context, k8sClient client.Client, name, namespace string, nodePort int32) (corev1.Service, error) {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: fixture.NamespaceLabels,
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

// Create Argo Rollout CR
func CreateArgoRollout(ctx context.Context, k8sClient client.Client, name, namespace, activeService, previewService string) (rv1alpha1.Rollout, error) {
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
				MatchLabels: fixture.NamespaceLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: fixture.NamespaceLabels,
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

// Check resources created after creating of RolloutManager CR and verify that they are healthy.
func ValidateArgoRolloutManagerResources(ctx context.Context, rolloutsManager rmv1alpha1.RolloutManager, k8sClient client.Client, namespaceScoped bool) {

	By("Verify that ServiceAccount is created.")
	validateServiceAccount(k8sClient, rolloutsManager)

	if namespaceScoped {
		By("Verify that argo-rollout Role is created.")
		validateArgoRolloutsRole(k8sClient, rolloutsManager)
	} else {
		By("Verify that argo-rollout ClusterRoles is created.")
		validateArgoRolloutsClusterRole(k8sClient)
	}

	By("Verify that aggregate-to-admin ClusterRole is created.")
	validateAggregateToAdminClusterRole(k8sClient)

	By("Verify that aggregate-to-edit ClusterRole is created.")
	validateAggregateToEditClusterRole(k8sClient)

	By("Verify that aggregate-to-view ClusterRole is created.")
	validateAggregateToViewClusterRole(k8sClient)

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

// Create Argo Rollout CR and Services required by it and verify that they are healthy.
func ValidateArgoRolloutsResources(ctx context.Context, k8sClient client.Client, nsName string, port1, port2 int32) {

	By("Create active Services in given namespace")
	rolloutServiceActive, err := CreateService(ctx, k8sClient, RolloutsActiveServiceName, nsName, port1)
	Expect(err).ToNot(HaveOccurred())
	Eventually(&rolloutServiceActive, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Create preview Services in given namespace")
	rolloutServicePreview, err := CreateService(ctx, k8sClient, RolloutsPreviewServiceName, nsName, port2)
	Expect(err).ToNot(HaveOccurred())
	Eventually(&rolloutServicePreview, "10s", "1s").Should(k8s.ExistByName(k8sClient))

	By("Create Argo Rollout CR in given namespace and check it is reconciled successfully.")
	rollout, err := CreateArgoRollout(ctx, k8sClient, RolloutsName, nsName, rolloutServiceActive.Name, rolloutServicePreview.Name)
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

func validateArgoRolloutsClusterRole(k8sClient client.Client) {
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

func validateAggregateToAdminClusterRole(k8sClient client.Client) {

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

func validateAggregateToEditClusterRole(k8sClient client.Client) {

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

func validateAggregateToViewClusterRole(k8sClient client.Client) {

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
