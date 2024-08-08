package rollouts

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

		expectedDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", DefaultArgoRolloutsResourceName, a)

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
		Expect(fetchedDeployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(expectedDeployment.Spec.Template.Spec.Containers[0].Resources))
	})

	When("Rollouts Deployment already exists, but then is modified away from default values", func() {
		It("should update the Deployment back to default values, but preserve any added annotations/labels", func() {
			By("create a new Deployment with custom values")
			existingDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin-test", "tmp-test"}, "linux-test", sa.Name, a)

			existingDeployment.Labels["new-label"] = "new-label-value"
			existingDeployment.Annotations["new-annotation"] = "new-annotation-value"

			Expect(r.Client.Create(ctx, existingDeployment)).To(Succeed())

			By("calling reconcileRolloutsDeployment")
			Expect(r.reconcileRolloutsDeployment(ctx, a, *sa)).To(Succeed())

			By("fetch the Deployment")
			fetchedDeployment := &appsv1.Deployment{}
			Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())

			expectedDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", sa.Name, a)

			By("verifing that the Deployment has been reconciled back to default values")
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
			Expect(fetchedDeployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(expectedDeployment.Spec.Template.Spec.Containers[0].Resources))

		})
	})

	When("RolloutManagerCR has custom controller resources defined", func() {

		It("should create a Deployment that uses those controller resources", func() {

			By("setting resource requirements on RolloutsManager CR")
			a.Spec.ControllerResources = &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("500Mi"),
				},
			}
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("calling reconcileRolloutsDeployment to create the initial set of rollout resources")
			Expect(r.reconcileRolloutsDeployment(ctx, a, *sa)).To(Succeed())

			By("fetching the Deployment")
			fetchedDeployment := &appsv1.Deployment{}
			Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())

			By("verifying that the fetched Deployment matches the desired one")
			Expect(fetchedDeployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(*a.Spec.ControllerResources))
		})

		defaultContainerResources := defaultRolloutsContainerResources()

		nonDefaultContainerResourcesValue := &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
		}

		otherNonDefault := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1m"),
				corev1.ResourceMemory: resource.MustParse("1Mi"),
			},
		}

		DescribeTable("Deployment CR should always be updated to be consistent with RolloutsManager .spec.controllerResources field, in both default and non-default cases", func(initialDeployment *corev1.ResourceRequirements, crValue *corev1.ResourceRequirements, expectedDeployment *corev1.ResourceRequirements) {

			By("calling reconcileRolloutsDeployment to create the initial set of rollout resources")
			Expect(r.reconcileRolloutsDeployment(ctx, a, *sa)).To(Succeed())

			By("updating the default Deployment to resource value defined in 'initialDeployment'")
			fetchedDeployment := &appsv1.Deployment{}
			Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())

			fetchedDeployment.Spec.Template.Spec.Containers[0].Resources = *initialDeployment
			Expect(r.Update(ctx, fetchedDeployment)).To(Succeed())

			if crValue != nil {
				By("setting resource requirements on RolloutsManager CR to 'crValue'")
				a.Spec.ControllerResources = crValue
				Expect(r.Client.Update(ctx, &a)).To(Succeed())
			}

			By("calling reconcileRolloutsDeployment")
			Expect(r.reconcileRolloutsDeployment(ctx, a, *sa)).To(Succeed())

			By("fetching the Deployment")
			fetchedDeployment = &appsv1.Deployment{}
			Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())

			By("verifying that the fetched Deployment resource requirements matches the desired resource requirements")
			Expect(fetchedDeployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(*expectedDeployment))

		},
			Entry("default deployment, with a empty CR .spec.containerResources -> no change in deployment from default", &defaultContainerResources, nil, &defaultContainerResources),
			Entry("default deployment, with CR non-default value in .spec.containerResources -> deployment should now have value from CR", &defaultContainerResources, nonDefaultContainerResourcesValue, nonDefaultContainerResourcesValue),
			Entry("deployment with non-default container resources, empty value in CR .spec.containerResources -> Deployment should revert to default value from CR", nonDefaultContainerResourcesValue, nil, &defaultContainerResources),
			Entry("deployment with a different non-default container resources, non-default value in CR .spec.containerResources -> Deployment should use CR value", &otherNonDefault, nonDefaultContainerResourcesValue, nonDefaultContainerResourcesValue),
		)

	})

	When("Rollouts deployment already exists, but then RolloutManager is modified in a way that requires updating either .spec.selector of the existing Deployment", func() {

		It("should cause the existing Deployment to be deleted, and a new Deployment to be created with the updated .spec.selector", func() {

			By("create a basic Rollout Deployment")
			existingDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", DefaultArgoRolloutsResourceName, a)

			By("assigning a fake UID to the original deployment, so we can detected when it is deleted/recreated")
			existingDeployment.ObjectMeta.UID = "original-deployment"

			Expect(r.Client.Create(ctx, existingDeployment)).To(Succeed())

			By("calling reconcileRolloutsDeployment")
			Expect(r.reconcileRolloutsDeployment(ctx, a, *sa)).To(Succeed())

			fetchedDeployment := &appsv1.Deployment{}
			Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())
			Expect(fetchedDeployment.ObjectMeta.UID).To(Equal(types.UID("original-deployment")))

			// In this case, because we are updating .spec.additionalMetadata, that causes .spec.selector of the Deployment to be modified, which requires recreating the Deployment below

			By("adding a new label to RolloutManager .spec.additionalMetadata.labels field")
			a.Spec.AdditionalMetadata = &v1alpha1.ResourceMetadata{
				Labels: map[string]string{"new-label": "new-label-value"},
			}
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("calling reconcileRolloutsDeployment again")
			Expect(r.reconcileRolloutsDeployment(ctx, a, *sa)).To(Succeed())

			By("fetching the Deployment after reconcile was called, to verify it performed as expected")
			fetchedDeployment = &appsv1.Deployment{}
			Expect(fetchObject(ctx, r.Client, a.Namespace, DefaultArgoRolloutsResourceName, fetchedDeployment)).To(Succeed())
			Expect(fetchedDeployment.ObjectMeta.UID).To(Equal(types.UID("")), "UID should be empty, because the original Deployment was deleted and recreated")

			expectedDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", sa.Name, a)

			By("verifying that the Deployment has been reconciled back to default labels")
			Expect(fetchedDeployment.Name).To(Equal(expectedDeployment.Name))
			for k, v := range expectedDeployment.Labels {
				Expect(fetchedDeployment.Labels).To(HaveKeyWithValue(k, v), "operator-added labels should still be present")
			}

			Expect(fetchedDeployment.Labels).To(HaveKeyWithValue("new-label", "new-label-value"), "user label should still be present")
		})

	})

	When("the deployment has not changed", func() {
		It("should not report any difference, both before and after normalization", func() {

			expectedDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", DefaultArgoRolloutsResourceName, a)
			Expect(identifyDeploymentDifference(*expectedDeployment, *expectedDeployment)).To(Equal(""), "comparing the object with itself should always report no differences")

			expectedDeployment = deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", DefaultArgoRolloutsResourceName, a)
			expectedDeploymentNormalized, err := normalizeDeployment(*expectedDeployment, a)
			Expect(err).To(Succeed())

			Expect(identifyDeploymentDifference(expectedDeploymentNormalized, *expectedDeployment)).To(Equal(""))
			Expect(identifyDeploymentDifference(*expectedDeployment, expectedDeploymentNormalized)).To(Equal(""))
			Expect(identifyDeploymentDifference(*expectedDeployment, *expectedDeployment)).To(Equal(""))

		})
	})

	When("normalizeDeployment is called with a Deployment containing additional, user-defined labels/annotations", func() {

		It("should ensure the user-defiend labels/annotations are be removed by called to normalizeDeployment, while preserving the values contributed by the operation", func() {

			originalDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", DefaultArgoRolloutsResourceName, a)

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
			defaultDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", DefaultArgoRolloutsResourceName, a)

			defaultDeploymentModified := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", DefaultArgoRolloutsResourceName, a)

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
			Entry("spec.strategy", func(deployment *appsv1.Deployment) {
				deployment.Spec.Strategy = appsv1.DeploymentStrategy{
					Type: "not-a-real-strategy",
				}
			}),
			Entry(".spec.template.spec.containers.args", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Containers[0].Args = []string{"new", "args"}
			}),
			Entry(".spec.template.spec.containers.env", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
					{Name: "my-env", Value: "my-env-value"}}
			}),
			Entry(".spec.template.spec.containers.resources", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceEphemeralStorage: resource.MustParse("20000Gi"),
					},
				}
			}),
			Entry(".spec.template.spec.containers.volumeMounts", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
					{Name: "not-an-expected-name"},
				}
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
			Entry(".spec.template.spec.containers.resources", func(deployment *appsv1.Deployment) {
				deployment.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				}
			}),
		)

	})

})

var _ = Describe("generateDesiredRolloutsDeployment tests", func() {
	var (
		cr v1alpha1.RolloutManager
		sa corev1.ServiceAccount
	)

	BeforeEach(func() {
		cr = v1alpha1.RolloutManager{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
			},
			Spec: v1alpha1.RolloutManagerSpec{
				AdditionalMetadata: &v1alpha1.ResourceMetadata{
					Labels: map[string]string{
						"label": "value",
					},
					Annotations: map[string]string{
						"annotation": "value",
					},
				},
				NodePlacement: &v1alpha1.RolloutsNodePlacementSpec{
					NodeSelector: map[string]string{
						"key1": "value1",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "key1",
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			},
		}

		sa = corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: cr.Namespace,
			},
		}
	})

	Context("when generating the desired deployment", func() {
		It("should set the correct metadata on the deployment", func() {
			deployment := generateDesiredRolloutsDeployment(cr, sa)
			Expect(deployment.ObjectMeta.Name).To(Equal(DefaultArgoRolloutsResourceName))
			Expect(deployment.ObjectMeta.Namespace).To(Equal(cr.Namespace))

			// Verify whether labels and annotations are correctly set
			Expect(deployment.Spec.Template.Labels["label"]).To(Equal("value"))
			Expect(deployment.Spec.Template.Annotations["annotation"]).To(Equal("value"))
		})

		It("should set the NodeSelector and tolerations if NodePlacement is provided", func() {
			deployment := generateDesiredRolloutsDeployment(cr, sa)
			Expect(deployment.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"kubernetes.io/os": "linux", "key1": "value1"}))
			Expect(deployment.Spec.Template.Spec.Tolerations).To(ContainElement(corev1.Toleration{
				Key:      "key1",
				Operator: corev1.TolerationOpExists,
			}))
		})

		It("should set the default node selector if NodePlacement is not provided", func() {
			cr.Spec.NodePlacement = nil
			deployment := generateDesiredRolloutsDeployment(cr, sa)
			Expect(deployment.Spec.Template.Spec.NodeSelector).To(Equal(map[string]string{"kubernetes.io/os": "linux"}))
			Expect(deployment.Spec.Template.Spec.Tolerations).To(BeNil())
		})

		It("should set the service account name", func() {
			deployment := generateDesiredRolloutsDeployment(cr, sa)
			Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(sa.ObjectMeta.Name))
		})

		It("should add the correct volumes", func() {
			deployment := generateDesiredRolloutsDeployment(cr, sa)
			Expect(deployment.Spec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
				Name: "plugin-bin",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}))
			Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
				Name: "tmp",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}))
		})
	})
})

var _ = Describe("normalizeDeployment tests to verify that an error is returned", func() {
	var (
		a          v1alpha1.RolloutManager
		deployment *appsv1.Deployment
	)

	BeforeEach(func() {
		a = *makeTestRolloutManager()

		// Set up a valid deployment object
		deployment = deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin", "tmp"}, "linux", DefaultArgoRolloutsResourceName, a)
	})

	DescribeTable("should return an error when",
		func(modifyDeployment func(), expectedError string) {
			modifyDeployment()
			_, err := normalizeDeployment(*deployment, a)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
		},

		Entry("spec.selector is nil", func() {
			deployment.Spec.Selector = nil
		}, "missing .spec.selector"),

		Entry("spec.template.spec.securityContext is nil", func() {
			deployment.Spec.Template.Spec.SecurityContext = nil
		}, "missing .spec.template.spec.securityContext"),

		Entry("spec.template.spec.volumes is nil", func() {
			deployment.Spec.Template.Spec.Volumes = nil
		}, "missing .spec.template.spec.volumes"),

		Entry("spec.template.spec.volumes has incorrect length", func() {
			deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
				{Name: "volume1"},
			}
		}, "missing .spec.template.spec.volumes"),

		Entry("spec.template.spec.containers has incorrect length", func() {
			deployment.Spec.Template.Spec.Containers = []corev1.Container{
				{Name: "test-1"},
				{Name: "test-2"},
			}
		}, "incorrect number of .spec.template.spec.containers"),

		Entry("liveness probe is nil", func() {
			deployment.Spec.Template.Spec.Containers[0].LivenessProbe = nil
		}, "incorrect liveness probe"),

		Entry("liveness probe http get is nil", func() {
			deployment.Spec.Template.Spec.Containers[0].LivenessProbe.ProbeHandler.HTTPGet = nil
		}, "incorrect http get in liveness probe"),

		Entry("readiness probe is nil", func() {
			deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = nil
		}, "incorrect readiness probe"),

		Entry("readiness probe http get is nil", func() {
			deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.HTTPGet = nil
		}, "incorrect http get in readiness probe"),

		Entry("input ports is nil", func() {
			deployment.Spec.Template.Spec.Containers[0].Ports = nil
		}, "incorrect input ports"),

		Entry("input ports has incorrect length", func() {
			deployment.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
				{ContainerPort: 8080, Name: "http"},
			}
		}, "incorrect input ports"),

		Entry("security context is nil", func() {
			deployment.Spec.Template.Spec.Containers[0].SecurityContext = nil
		}, "incorrect security context"),

		Entry("security context capabilities is nil", func() {
			deployment.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities = nil
		}, "incorrect security context"),

		Entry("volume mounts is nil", func() {
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = nil
		}, "incorrect volume mounts"),

		Entry("volume mounts has incorrect length", func() {
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{Name: "volume1", MountPath: "/mnt/volume1"},
			}
		}, "incorrect volume mounts"),
	)
})

var _ = Describe("getRolloutsContainerImage tests", func() {
	var (
		a v1alpha1.RolloutManager
	)

	BeforeEach(func() {
		a = *makeTestRolloutManager()
		os.Unsetenv("ARGO_ROLLOUTS_IMAGE") // Ensure env variable is not set unless needed
	})

	When("the spec Image and Version are empty", func() {
		It("returns the default image and tag combined", func() {
			a.Spec.Image = ""
			a.Spec.Version = ""
			Expect(getRolloutsContainerImage(a)).To(Equal(DefaultArgoRolloutsImage + ":" + DefaultArgoRolloutsVersion))
		})
	})

	When("the spec Image is set but Version is empty", func() {
		It("returns the custom image with the default tag", func() {
			a.Spec.Image = "custom-image"
			Expect(getRolloutsContainerImage(a)).To(Equal("custom-image:" + DefaultArgoRolloutsVersion))
		})
	})

	When("the spec Image is empty but Version is set", func() {
		It("returns the default image with the custom tag", func() {
			a.Spec.Version = "custom-tag"
			Expect(getRolloutsContainerImage(a)).To(Equal(DefaultArgoRolloutsImage + ":custom-tag"))
		})
	})

	When("both spec Image and Version are set", func() {
		It("returns the custom image and custom tag combined", func() {
			a.Spec.Image = "custom-image"
			a.Spec.Version = "custom-tag"
			Expect(getRolloutsContainerImage(a)).To(Equal("custom-image:custom-tag"))
		})
	})

	When("the environment variable is set and spec is empty", func() {
		It("returns the environment variable image", func() {
			os.Setenv("ARGO_ROLLOUTS_IMAGE", "env-image")
			Expect(getRolloutsContainerImage(a)).To(Equal("env-image"))
		})
	})

	When("the environment variable is set but spec is not empty", func() {
		It("returns the custom image and tag ignoring the environment variable", func() {
			a.Spec.Image = "custom-image"
			a.Spec.Version = "custom-tag"
			os.Setenv("ARGO_ROLLOUTS_IMAGE", "env-image")
			Expect(getRolloutsContainerImage(a)).To(Equal("custom-image:custom-tag"))
		})
	})
})

var _ = Describe("rolloutsContainer tests", func() {
	It("should include HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables in the container", func() {
		By("Set environment variables")
		os.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")
		os.Setenv("HTTPS_PROXY", "https://proxy.example.com:8443")
		os.Setenv("NO_PROXY", "localhost,127.0.0.1")

		By("Create a RolloutManager CR")
		cr := v1alpha1.RolloutManager{
			Spec: v1alpha1.RolloutManagerSpec{
				Env: []corev1.EnvVar{
					{Name: "EXISTING_VAR", Value: "existing_value"},
				},
				ControllerResources: nil,
			},
		}

		By("Call rolloutsContainer function")
		container := rolloutsContainer(cr)

		By("Verify the environment variables")
		expectedEnvVars := map[string]string{
			"EXISTING_VAR": "existing_value",
			"HTTP_PROXY":   "http://proxy.example.com:8080",
			"HTTPS_PROXY":  "https://proxy.example.com:8443",
			"NO_PROXY":     "localhost,127.0.0.1",
		}

		for _, env := range container.Env {
			if val, exists := expectedEnvVars[env.Name]; exists {
				Expect(env.Value).To(Equal(val))
				delete(expectedEnvVars, env.Name)
			}
		}
	})
})

func deploymentCR(name string, namespace string, rolloutsSelectorLabel string, volumeNames []string, nodeSelector string, serviceAccount string, rolloutManager v1alpha1.RolloutManager) *appsv1.Deployment {
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
				DefaultRolloutsSelectorKey: rolloutsSelectorLabel,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					DefaultRolloutsSelectorKey: rolloutsSelectorLabel,
				},
				Annotations: make(map[string]string, 0),
			},
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: volumeNames[0],
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: volumeNames[1],
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
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
