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

	BeforeEach(func() {
		ctx = context.Background()
		a = makeTestRolloutManager()

		r = makeTestReconciler(a)
		Expect(createNamespace(r, a.Namespace)).To(Succeed())
	})

	It("should create a new deployment if it does not exist", func() {
		desiredDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: a.Namespace,
			},
		}

		Expect(r.reconcileRolloutsDeployment(ctx, a, &corev1.ServiceAccount{})).To(Succeed())

		By("fetch the Deployment")
		fetchedDeployment := &appsv1.Deployment{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, desiredDeployment.Name, fetchedDeployment)).To(Succeed())

		By("verify that the fetched Deployment matches the desired one")
		Expect(fetchedDeployment.Name).To(Equal(desiredDeployment.Name))

	})

	It("should update the deployment if it already exists", func() {
		runAsNonRoot := true
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: a.Namespace,
			},
		}
		Expect(r.Client.Create(ctx, sa)).To(Succeed())

		By("create a Deployment")
		existingDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: a.Namespace,
			},
		}
		setRolloutsLabels(&existingDeployment.ObjectMeta)
		existingDeployment.Spec = appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName,
					},
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					Containers: []corev1.Container{
						rolloutsContainer(a),
					},
					ServiceAccountName: sa.Name,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
					},
				},
			},
		}

		Expect(r.Client.Create(ctx, existingDeployment)).To(Succeed())
		Expect(r.reconcileRolloutsDeployment(ctx, a, sa)).To(Succeed())

		By("fetch the Deployment")
		fetchedDeployment := &appsv1.Deployment{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, existingDeployment.Name, fetchedDeployment)).To(Succeed())

		By("verify that the fetched Deployment matches the existing one")
		Expect(fetchedDeployment.Name).To(Equal(existingDeployment.Name))
		Expect(fetchedDeployment.Labels).To(Equal(existingDeployment.Labels))
		Expect(fetchedDeployment.Spec.Template.Spec.ServiceAccountName).To(Equal(existingDeployment.Spec.Template.Spec.ServiceAccountName))
		Expect(fetchedDeployment.Spec.Template.Labels).To(Equal(existingDeployment.Spec.Template.Labels))
		Expect(fetchedDeployment.Spec.Selector).To(Equal(existingDeployment.Spec.Selector))
		Expect(fetchedDeployment.Spec.Template.Spec.NodeSelector).To(Equal(existingDeployment.Spec.Template.Spec.NodeSelector))
		Expect(fetchedDeployment.Spec.Template.Spec.Tolerations).To(Equal(existingDeployment.Spec.Template.Spec.Tolerations))
		Expect(fetchedDeployment.Spec.Template.Spec.SecurityContext).To(Equal(existingDeployment.Spec.Template.Spec.SecurityContext))
		Expect(fetchedDeployment.Spec.Template.Spec.Volumes).To(Equal(existingDeployment.Spec.Template.Spec.Volumes))

	})

})
