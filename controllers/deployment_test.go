package rollouts

import (
	"context"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Deployment Test", func() {
	var ctx context.Context
	var a *v1alpha1.RolloutManager
	var r *RolloutManagerReconciler
	var sa *corev1.ServiceAccount

	BeforeEach(func() {
		ctx = context.Background()
		a = makeTestRolloutManager()

		r = makeTestReconciler(a)
		Expect(createNamespace(r, a.Namespace)).To(Succeed())

		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: a.Namespace,
			},
		}
		Expect(r.Client.Create(ctx, sa)).To(Succeed())
	})

	It("should create a new deployment if it does not exist", func() {

		By("calling reconcileRolloutsDeployment")
		Expect(r.reconcileRolloutsDeployment(ctx, a, sa)).To(Succeed())

		By("fetch the Deployment")
		fetchedDeployment := &appsv1.Deployment{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())

		expectedDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, "tmp", "linux", DefaultArgoRolloutsResourceName, a)

		By("verify that the fetched Deployment matches the desired one")
		Expect(fetchedDeployment.Name).To(Equal(expectedDeployment.Name))
		Expect(fetchedDeployment.Labels).To(Equal(expectedDeployment.Labels))
		Expect(fetchedDeployment.Spec.Template.Spec.ServiceAccountName).To(Equal(expectedDeployment.Spec.Template.Spec.ServiceAccountName))
		Expect(fetchedDeployment.Spec.Template.Labels).To(Equal(expectedDeployment.Spec.Template.Labels))
		Expect(fetchedDeployment.Spec.Selector).To(Equal(expectedDeployment.Spec.Selector))
		Expect(fetchedDeployment.Spec.Template.Spec.NodeSelector).To(Equal(expectedDeployment.Spec.Template.Spec.NodeSelector))
		Expect(fetchedDeployment.Spec.Template.Spec.Tolerations).To(Equal(expectedDeployment.Spec.Template.Spec.Tolerations))
		Expect(fetchedDeployment.Spec.Template.Spec.SecurityContext).To(Equal(expectedDeployment.Spec.Template.Spec.SecurityContext))
		Expect(fetchedDeployment.Spec.Template.Spec.Volumes).To(Equal(expectedDeployment.Spec.Template.Spec.Volumes))
	})

	It("should update the Deployment back to default values, if deployment already exists and has been modified away from default", func() {

		By("create a new Deployment with custom values")
		existingDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, "test-resource-name", "tmp-test", "linux-test", sa.Name, a)

		Expect(r.Client.Create(ctx, existingDeployment)).To(Succeed())

		By("calling reconcileRolloutsDeployment")
		Expect(r.reconcileRolloutsDeployment(ctx, a, sa)).To(Succeed())

		By("fetch the Deployment")
		fetchedDeployment := &appsv1.Deployment{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())

		expectedDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, "tmp", "linux", sa.Name, a)

		By("verify that the Deployment has been reconciled back to default values")
		Expect(fetchedDeployment.Name).To(Equal(expectedDeployment.Name))
		Expect(fetchedDeployment.Labels).To(Equal(expectedDeployment.Labels))
		Expect(fetchedDeployment.Spec.Template.Spec.ServiceAccountName).To(Equal(expectedDeployment.Spec.Template.Spec.ServiceAccountName))
		Expect(fetchedDeployment.Spec.Template.Labels).To(Equal(expectedDeployment.Spec.Template.Labels))
		Expect(fetchedDeployment.Spec.Selector).To(Equal(expectedDeployment.Spec.Selector))
		Expect(fetchedDeployment.Spec.Template.Spec.NodeSelector).To(Equal(expectedDeployment.Spec.Template.Spec.NodeSelector))
		Expect(fetchedDeployment.Spec.Template.Spec.Tolerations).To(Equal(expectedDeployment.Spec.Template.Spec.Tolerations))
		Expect(fetchedDeployment.Spec.Template.Spec.SecurityContext).To(Equal(expectedDeployment.Spec.Template.Spec.SecurityContext))
		Expect(fetchedDeployment.Spec.Template.Spec.Volumes).To(Equal(expectedDeployment.Spec.Template.Spec.Volumes))

	})

})

func deploymentCR(name string, namespace string, label string, volumeName string, nodeSelector string, serviceAccount string, rolloutManager *v1alpha1.RolloutManager) *appsv1.Deployment {
	runAsNonRoot := true
	deploymentCR := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	setRolloutsLabels(&deploymentCR.ObjectMeta)
	deploymentCR.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				DefaultRolloutsSelectorKey: label,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					DefaultRolloutsSelectorKey: label,
				},
			},
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: volumeName,
					},
				},
				NodeSelector: map[string]string{
					"kubernetes.io/os": nodeSelector,
				},
				Containers: []corev1.Container{
					rolloutsContainer(rolloutManager),
				},
				ServiceAccountName: serviceAccount,
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: &runAsNonRoot,
				},
			},
		},
	}

	return deploymentCR

}
