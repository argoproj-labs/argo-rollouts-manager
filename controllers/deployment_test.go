package rollouts

import (
	"context"
	"fmt"
	"reflect"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Deployment Test", func() {
	var ctx context.Context
	var a v1alpha1.RolloutManager
	var r *RolloutManagerReconciler
	var sa *corev1.ServiceAccount

	BeforeEach(func() {
		ctx = context.Background()
		a = *makeTestRolloutManager()

		r = makeTestReconciler(&a)
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

		By("calling reconcileRolloutsDeployment to create the initial set of rollout resources")
		Expect(r.reconcileRolloutsDeployment(ctx, a, *sa)).To(Succeed())

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

		existingDeployment.Labels["new-label"] = "new-label-value"
		existingDeployment.Annotations["new-annotation"] = "new-annotation-value"

		Expect(r.Client.Create(ctx, existingDeployment)).To(Succeed())

		By("calling reconcileRolloutsDeployment")
		Expect(r.reconcileRolloutsDeployment(ctx, a, *sa)).To(Succeed())

		By("fetch the Deployment")
		fetchedDeployment := &appsv1.Deployment{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())

		expectedDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, "tmp", "linux", sa.Name, a)

		By("verify that the Deployment has been reconciled back to default values")
		Expect(fetchedDeployment.Name).To(Equal(expectedDeployment.Name))
		for k, v := range expectedDeployment.Labels {
			Expect(fetchedDeployment.Labels).To(HaveKeyWithValue(k, v), "operator-added labels should still be present")
		}

		Expect(fetchedDeployment.Labels).To(HaveKeyWithValue("new-label", "new-label-value"), "user label should still be present")
		Expect(fetchedDeployment.Annotations).To(HaveKeyWithValue("new-annotation", "new-annotation-value"), "user annotation should still be present")

		Expect(fetchedDeployment.Spec.Template.Spec.ServiceAccountName).To(Equal(expectedDeployment.Spec.Template.Spec.ServiceAccountName))
		Expect(fetchedDeployment.Spec.Template.Labels).To(Equal(expectedDeployment.Spec.Template.Labels))
		Expect(fetchedDeployment.Spec.Selector).To(Equal(expectedDeployment.Spec.Selector))
		Expect(fetchedDeployment.Spec.Template.Spec.NodeSelector).To(Equal(expectedDeployment.Spec.Template.Spec.NodeSelector))
		Expect(fetchedDeployment.Spec.Template.Spec.Tolerations).To(Equal(expectedDeployment.Spec.Template.Spec.Tolerations))
		Expect(fetchedDeployment.Spec.Template.Spec.SecurityContext).To(Equal(expectedDeployment.Spec.Template.Spec.SecurityContext))
		Expect(fetchedDeployment.Spec.Template.Spec.Volumes).To(Equal(expectedDeployment.Spec.Template.Spec.Volumes))

	})

	When("the deployment has not changed", func() {
		It("should not report any difference, both before and after normalization", func() {

			expectedDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, "tmp", "linux", DefaultArgoRolloutsResourceName, a)
			Expect(identifyDeploymentDifference(*expectedDeployment, *expectedDeployment)).To(Equal(""))

			expectedDeployment = deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, "tmp", "linux", DefaultArgoRolloutsResourceName, a)
			expectedDeploymentNormalized, err := normalizeDeployment(*expectedDeployment, a)
			Expect(err).To(Succeed())

			Expect(identifyDeploymentDifference(expectedDeploymentNormalized, *expectedDeployment)).To(Equal(""))
			Expect(identifyDeploymentDifference(*expectedDeployment, expectedDeploymentNormalized)).To(Equal(""))
			Expect(identifyDeploymentDifference(*expectedDeployment, *expectedDeployment)).To(Equal(""))

		})
	})

	When("normalizeDeployment is called with a Deployment containing additional, user-defined labels/annotations", func() {

		It("should ensure the user-defiend labels/annotations are be removed by called to normalizeDeployment, while preserving the values contributed by the operation", func() {

			originalDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, "tmp", "linux", DefaultArgoRolloutsResourceName, a)

			By("creating a new object with user added labels/annotations")
			new := originalDeployment.DeepCopy()

			new.Annotations["newAnnotation"] = "newAnnotationValue"
			Expect(identifyDeploymentDifference(*originalDeployment, *new)).To(Equal("Annotations"), "identifyDeploymentDifference should correctly identify that the annotations value has changed")

			new.Labels["newLabel"] = "newLabelValue"
			Expect(identifyDeploymentDifference(*originalDeployment, *new)).To(Equal("Labels"), "identifyDeploymentDifference should correctly identify that the labels value has changed")

			By("calling normalizeDeployment on the object with user defined values")
			res, err := normalizeDeployment(*new, a)
			Expect(err).To(BeNil())

			Expect(res.Labels).ToNot(HaveKey("newLabel"), "user label should not be present")
			Expect(res.Annotations).ToNot(HaveKey("newAnnotation"), "user annotation should not be present")
			for k := range originalDeployment.Annotations {
				Expect(res.Annotations).To(HaveKey(k), "default operator annotations should still be present")
			}
			for k := range originalDeployment.Labels {
				Expect(res.Labels).To(HaveKey(k), "default operator labels should still be present")
			}

		})
	})

	When("the Rollouts Deployment resource is changed by the user, outside of the operator", func() {

		areEqual := func(x appsv1.Deployment, y appsv1.Deployment, rm v1alpha1.RolloutManager) bool {

			xRes, xErr := normalizeDeployment(x, rm)
			yRes, yErr := normalizeDeployment(y, rm)

			if fmt.Sprintf("%v", xErr) != fmt.Sprintf("%v", yErr) {
				return false
			}

			res := reflect.DeepEqual(xRes, yRes)

			// Sanity test that identifyDeploymentDifference gives the same result as reflect.DeepEqual
			deploymentDiff := identifyDeploymentDifference(x, y)
			ExpectWithOffset(0, res == (deploymentDiff == "")).To(BeTrue())

			return res
		}

		DescribeTable("controller should detect and revert the change", func(fxn func(deployment *appsv1.Deployment)) {

			By("ensuring that deploymentCR properly detects the change")
			defaultDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, "tmp", "linux", DefaultArgoRolloutsResourceName, a)

			defaultDeploymentModified := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, "tmp", "linux", DefaultArgoRolloutsResourceName, a)

			Expect(identifyDeploymentDifference(*defaultDeployment, *defaultDeploymentModified)).To(BeEmpty(), "they should be the same before one is modified")

			fxn(defaultDeploymentModified)
			Expect(identifyDeploymentDifference(*defaultDeployment, *defaultDeploymentModified)).ToNot(BeEmpty(), "after being modified, they should no longer be the same")

			By("ensuring the reconcileRolloutsDeployment will detect and revert the change")
			{
				By("calling reconcileRolloutsDeployment to create a default Deployment")
				expectedDepl := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: DefaultArgoRolloutsResourceName, Namespace: a.Namespace},
				}
				Expect(r.reconcileRolloutsDeployment(context.Background(), a, *sa)).To(Succeed())
				Expect(r.Client.Get(context.Background(), client.ObjectKeyFromObject(&expectedDepl), &expectedDepl)).To(Succeed())
				updatedDepl := expectedDepl.DeepCopy()
				Expect(areEqual(*updatedDepl, expectedDepl, a)).To(BeTrue(), "copy should be same as original")

				By("updating the Deployment using the function, and then updating the cluster resource")
				fxn(updatedDepl)
				Expect(r.Client.Update(context.Background(), updatedDepl)).To(Succeed())

				By("retrieving the cluster resource after the update, to ensure it is equal to the updated version")
				updatedDeplFromClient := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: DefaultArgoRolloutsResourceName, Namespace: a.Namespace},
				}
				Expect(r.Client.Get(context.Background(), client.ObjectKeyFromObject(&updatedDeplFromClient), &updatedDeplFromClient)).To(Succeed())
				Expect(areEqual(*updatedDepl, updatedDeplFromClient, a)).To(BeTrue(), "resource on cluster should match the resource we called Update with")

				Expect(areEqual(updatedDeplFromClient, expectedDepl, a)).ToNot(BeTrue(), "resource on cluster should NOT match the original Deployment that was created by the call to reconcileRolloutsDeployment")

				By("calling reconcileRolloutsDeployment again, it should revert the change back to default")
				Expect(r.reconcileRolloutsDeployment(context.Background(), a, *sa)).To(Succeed())

				finalDeplFromClient := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: DefaultArgoRolloutsResourceName, Namespace: a.Namespace},
				}

				By("retrieving the Deployment version from the cluster")
				Expect(r.Client.Get(context.Background(), client.ObjectKeyFromObject(&finalDeplFromClient), &finalDeplFromClient)).To(Succeed())
				Expect(areEqual(finalDeplFromClient, expectedDepl, a)).To(BeTrue(), "version from cluster should have been reconciled back to the default")

			}

		},
			Entry("spec.selector", func(deployment *appsv1.Deployment) {
				deployment.Spec.Selector = &metav1.LabelSelector{
					MatchLabels: map[string]string{"my": "label"},
				}
			}),
			Entry(".spec.template.spec.containers.args", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Containers[0].Args = []string{"new", "args"}
			}),
			Entry(".spec.template.spec.containers.env", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
					{Name: "my-env", Value: "my-env-value"}}
			}),
			Entry(".spec.template.spec.serviceAccountName", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.ServiceAccountName = "different-service-account-name"
			}),
			Entry(".spec.template.labels", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Labels = map[string]string{"new": "label"}
			}),
			Entry(".spec.template.spec.nodeSelector", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.NodeSelector = map[string]string{"my": "node"}
			}),
			Entry(".spec.template.spec.tolerations", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Tolerations = []corev1.Toleration{{Key: "value"}}
			}),
			Entry(".spec.template.spec.securityContext", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeLocalhost},
				}
			}),
			Entry(".spec.template.spec.volumes", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "my-volume"}}
			}),
		)

	})

})

func deploymentCR(name string, namespace string, label string, volumeName string, nodeSelector string, serviceAccount string, rolloutManager v1alpha1.RolloutManager) *appsv1.Deployment {
	runAsNonRoot := true
	deploymentCR := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	setRolloutsLabelsAndAnnotationsToObject(&deploymentCR.ObjectMeta, rolloutManager)
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
