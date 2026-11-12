package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/k8s"
	rolloutManagerFixture "github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture/rolloutmanager"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	controllers "github.com/argoproj-labs/argo-rollouts-manager/controllers"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// This file contains tests that should run in both namespace-scoped and cluster-scoped scenarios.
// As of this writing, these function is called from the 'tests/e2e/(cluster-scoped/namespace-scoped)' packages.
func RunRolloutsTests(namespaceScopedParam bool) {

	testType := "cluster-scoped"
	if namespaceScopedParam {
		testType = "namespace-scoped"
	}

	Context("RolloutManager tests - "+testType, func() {

		var (
			k8sClient      client.Client
			ctx            context.Context
			rolloutManager rolloutsmanagerv1alpha1.RolloutManager
		)

		BeforeEach(func() {
			Expect(fixture.EnsureCleanSlate()).To(Succeed())

			var err error
			k8sClient, _, err = fixture.GetE2ETestKubeClient()
			Expect(err).ToNot(HaveOccurred())
			ctx = context.Background()

			rolloutManager = rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-rollouts-manager",
					Namespace: fixture.TestE2ENamespace,
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
					NamespaceScoped: namespaceScopedParam,
				},
			}
		})

		When("Reconcile is called on a new, basic, namespaced-scoped RolloutManager", func() {
			It("should create the appropriate K8s resources", func() {
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())

				By("waiting for phase to be \"Available\"")
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				By("Verify that expected resources are created.")
				ValidateArgoRolloutManagerResources(ctx, rolloutManager, k8sClient, namespaceScopedParam)
				By("Verify that RolloutManager successfully created Network Policy.")
				ValidateNetworkPolicy(k8sClient, rolloutManager)
				err := k8s.UpdateWithoutConflict(ctx, &rolloutManager, k8sClient, func(obj client.Object) {
					goObj, ok := obj.(*rolloutsmanagerv1alpha1.RolloutManager)
					Expect(ok).To(BeTrue())

					if goObj.Spec.NetworkPolicy == nil {
						goObj.Spec.NetworkPolicy = &rolloutsmanagerv1alpha1.RolloutManagerNetworkPolicySpec{}
					}

					goObj.Spec.NetworkPolicy.Enabled = new(false)
				})
				Expect(err).ToNot(HaveOccurred())
				By("Verify that RolloutManager successfully removed Network Policy.")
				policy := &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllers.DefaultRolloutsNetworkPolicy,
						Namespace: rolloutManager.Namespace,
					},
				}
				Eventually(policy, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))
			})
		})

		When("A RolloutManager is deleted", func() {
			It("should delete all the associated resources", func() {
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				Expect(k8sClient.Delete(ctx, &rolloutManager)).To(Succeed())

				By("deleting the service account")
				Eventually(&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				if namespaceScopedParam {
					By("deleting the role")
					Eventually(&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
					}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

					By("deleting the role binding")
					Eventually(&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
					}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				} else {
					By("deleting the cluster role")
					Eventually(&rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName},
					}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

					By("deleting the cluster role binding")
					Eventually(&rbacv1.ClusterRoleBinding{
						ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName},
					}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

					By("deleting three aggregate cluster roles")
					clusterRoleSuffixes := []string{"aggregate-to-admin", "aggregate-to-edit", "aggregate-to-view"}
					for _, suffix := range clusterRoleSuffixes {
						clusterRoleName := "argo-rollouts-" + suffix
						Eventually(&rbacv1.ClusterRole{
							ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
						}, "30s", "1s").ShouldNot(k8s.ExistByName(k8sClient))
					}
				}

				By("deleting the deployment")
				Eventually(&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				By("deleting the argo-rollouts-config configmap")
				Eventually(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsConfigMapName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").Should(k8s.NotExistByName(k8sClient))

				By("deleting the service")
				Eventually(&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsMetricsServiceName, Namespace: rolloutManager.Namespace},
				}, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				By("deleting the secret")
				Eventually(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsNotificationSecretName, Namespace: rolloutManager.Namespace},
				}, "30s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

				By("deleting the serviceMonitor")
				Eventually(&monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}, "30s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

			})
		})

		When("A RolloutManager specifies an extra argument", func() {
			It("should reflect that argument in the deployment", func() {
				By("creating the deployment with the argument from the RolloutManager")
				rolloutManager.Spec = rolloutsmanagerv1alpha1.RolloutManagerSpec{
					ExtraCommandArgs: []string{
						"--loglevel",
						"error",
					},
					NamespaceScoped: namespaceScopedParam,
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				var expectedContainerArgs []string
				if namespaceScopedParam {
					expectedContainerArgs = []string{"--namespaced", "--loglevel", "error"}
				} else {
					expectedContainerArgs = []string{"--loglevel", "error"}
				}

				Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(Equal(expectedContainerArgs))

				By("updating the deployment when the argument in the RolloutManager is updated")

				err := k8s.UpdateWithoutConflict(ctx, &rolloutManager, k8sClient, func(obj client.Object) {
					goObj, ok := obj.(*rolloutsmanagerv1alpha1.RolloutManager)
					Expect(ok).To(BeTrue())

					goObj.Spec = rolloutsmanagerv1alpha1.RolloutManagerSpec{
						ExtraCommandArgs: []string{
							"--logformat",
							"text",
						},
						NamespaceScoped: namespaceScopedParam,
					}
				})
				Expect(err).ToNot(HaveOccurred())

				if namespaceScopedParam {
					expectedContainerArgs = []string{"--namespaced", "--logformat", "text"}
				} else {
					expectedContainerArgs = []string{"--logformat", "text"}
				}

				Eventually(func() []string {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].Args
				}, "10s", "1s").Should(Equal(expectedContainerArgs))
			})
		})

		When("A RolloutManager specifies environment variables", func() {
			It("should reflect those variables in the deployment", func() {
				By("creating the deployment with the environment variables specified in the RolloutManager")

				rolloutManager.Spec.Env = []corev1.EnvVar{
					{Name: "EDITOR", Value: "emacs"},
					{Name: "LANG", Value: "en_CA.UTF-8"},
				}

				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(SatisfyAll(
					HaveLen(2),
					ContainElements(
						corev1.EnvVar{Name: "EDITOR", Value: "emacs"},
						corev1.EnvVar{Name: "LANG", Value: "en_CA.UTF-8"},
					),
				))

				By("updating the deployment when the environment variables in the RolloutManager are updated")

				err := k8s.UpdateWithoutConflict(ctx, &rolloutManager, k8sClient, func(obj client.Object) {
					goObj, ok := obj.(*rolloutsmanagerv1alpha1.RolloutManager)
					Expect(ok).To(BeTrue())

					goObj.Spec.Env = []corev1.EnvVar{
						{Name: "LANG", Value: "en_US.UTF-8"},
						{Name: "TERM", Value: "xterm-256color"},
					}
				})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() []corev1.EnvVar {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].Env
				}, "10s", "1s").Should(SatisfyAll(
					HaveLen(2),
					ContainElements(
						corev1.EnvVar{Name: "LANG", Value: "en_US.UTF-8"},
						corev1.EnvVar{Name: "TERM", Value: "xterm-256color"},
					),
				))
			})
		})

		When("A RolloutManager specifies an image", func() {
			It("should reflect that image in the deployment", func() {
				By("creating the deployment with the image specified in the RolloutManager")

				rolloutManager.Spec.Image = "quay.io/prometheus/busybox"
				rolloutManager.Spec.Version = "latest"

				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhasePending))

				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
				expectedVersion := rolloutManager.Spec.Image + ":" + rolloutManager.Spec.Version
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
				Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(expectedVersion))

				By("updating the deployment when the image in the RolloutManager is updated")

				err := k8s.UpdateWithoutConflict(ctx, &rolloutManager, k8sClient, func(obj client.Object) {
					goObj, ok := obj.(*rolloutsmanagerv1alpha1.RolloutManager)
					Expect(ok).To(BeTrue())
					goObj.Spec.Image = controllers.DefaultArgoRolloutsImage
					goObj.Spec.Version = controllers.DefaultArgoRolloutsVersion

				})
				Expect(err).ToNot(HaveOccurred())

				expectedVersion = controllers.DefaultArgoRolloutsImage + ":" + controllers.DefaultArgoRolloutsVersion
				Eventually(func() string {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].Image
				}, "10s", "1s").Should(Equal(expectedVersion))

				expectedServiceMonitor := &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllers.DefaultArgoRolloutsResourceName,
						Namespace: fixture.TestE2ENamespace,
					},
					Spec: monitoringv1.ServiceMonitorSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/name": controllers.DefaultArgoRolloutsMetricsServiceName,
							},
						},
						Endpoints: []monitoringv1.Endpoint{
							{
								Port: "metrics",
							},
						},
					},
				}

				By("verify whether ServiceMonitor is created or not for RolloutManager")
				sm := &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllers.DefaultArgoRolloutsResourceName,
						Namespace: fixture.TestE2ENamespace,
					},
				}

				Eventually(sm, "10s", "1s").Should(k8s.ExistByName(k8sClient))
				Expect(sm.Name).To(Equal(expectedServiceMonitor.Name))
				Expect(sm.Namespace).To(Equal(expectedServiceMonitor.Namespace))
				Expect(sm.Spec).To(Equal(expectedServiceMonitor.Spec))

			})
		})

		When("a label or annotation is added to Rollout's Deployment after the Deployment has been created", func() {

			It("should not ovewrite the label/annotation with operator labels/annotations, and should instead merge them", func() {

				By("creating default RolloutManager")
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllers.DefaultArgoRolloutsResourceName,
						Namespace: rolloutManager.Namespace,
					},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())

				deploymentExistingLabels := deployment.DeepCopy().GetLabels()
				deploymentExistingAnnotations := deployment.DeepCopy().GetAnnotations()

				By("updating the default Rollouts deployment with new labels")

				Expect(k8s.UpdateWithoutConflict(ctx, &deployment, k8sClient, func(o client.Object) {

					annots := o.GetAnnotations()
					annots["new-annotation"] = "new-annotation-value"
					o.SetAnnotations(annots)

					labels := o.GetLabels()
					labels["new-label"] = "new-label-value"
					o.SetLabels(labels)

				})).To(Succeed())

				Consistently(&deployment, "10s", "1s").Should(k8s.HaveLabel("new-label", "new-label-value", k8sClient), "user labels should still be present")
				Consistently(&deployment, "10s", "1s").Should(k8s.HaveAnnotation("new-annotation", "new-annotation-value", k8sClient), "user labels should still be present")

				for k, v := range deploymentExistingLabels {
					Expect(&deployment).To(k8s.HaveLabel(k, v, k8sClient), "operator labels should also still be present")
				}
				for k, v := range deploymentExistingAnnotations {
					Expect(&deployment).To(k8s.HaveAnnotation(k, v, k8sClient), "operator labels should also still be present")
				}

				By("removing the used-defined labels from the Deployment")

				Expect(k8s.UpdateWithoutConflict(ctx, &deployment, k8sClient, func(o client.Object) {

					annots := o.GetAnnotations()
					delete(annots, "new-annotation")
					o.SetAnnotations(annots)

					labels := o.GetLabels()
					delete(labels, "new-label")
					o.SetLabels(labels)

				})).To(Succeed())

				for k, v := range deploymentExistingLabels {
					Consistently(&deployment, "5s", "1s").Should(k8s.HaveLabel(k, v, k8sClient), "operator labels should also still be present")
				}
				for k, v := range deploymentExistingAnnotations {
					Consistently(&deployment, "5s", "1s").Should(k8s.HaveAnnotation(k, v, k8sClient), "operator annotations should also still be present")
				}

			})
		})

		When("A RolloutManager specifies metadata", func() {

			It("should create the controller with the correct labels and annotations", func() {

				rolloutsManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-rollouts-manager-with-metadata",
						Namespace: fixture.TestE2ENamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						AdditionalMetadata: &rolloutsmanagerv1alpha1.ResourceMetadata{
							Annotations: map[string]string{
								"foo-annotation":  "bar-annotation",
								"foo-annotation2": "bar-annotation2",
							},
							Labels: map[string]string{
								"foo-label":  "bar-label",
								"foo-label2": "bar-label2",
							},
						},
						NamespaceScoped: namespaceScopedParam,
					},
				}

				Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

				Eventually(rolloutsManager, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())

				expectMetadataOnObjectMeta(&deployment.ObjectMeta, rolloutsManager.Spec.AdditionalMetadata)
				expectMetadataOnObjectMeta(&deployment.Spec.Template.ObjectMeta, rolloutsManager.Spec.AdditionalMetadata)

				configMap := corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsConfigMapName, Namespace: rolloutManager.Namespace},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&configMap), &configMap)).To(Succeed())
				expectMetadataOnObjectMeta(&configMap.ObjectMeta, rolloutsManager.Spec.AdditionalMetadata)

				serviceAccount := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rolloutManager.Namespace},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&serviceAccount), &serviceAccount)).To(Succeed())
				expectMetadataOnObjectMeta(&serviceAccount.ObjectMeta, rolloutsManager.Spec.AdditionalMetadata)

				secret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsNotificationSecretName, Namespace: rolloutManager.Namespace},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).To(Succeed())
				expectMetadataOnObjectMeta(&secret.ObjectMeta, rolloutsManager.Spec.AdditionalMetadata)

				service := corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsMetricsServiceName, Namespace: rolloutManager.Namespace},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&service), &service)).To(Succeed())
				expectMetadataOnObjectMeta(&service.ObjectMeta, rolloutsManager.Spec.AdditionalMetadata)
			})
		})

		When("A RolloutManager specifies controller resources under .spec.controllerResources", func() {

			It("should create the controller with the correct resources requests/limits", func() {

				By("creating a RolloutManager containing resource requirements")

				rmWithResources := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-rollouts-manager-with-resources",
						Namespace: fixture.TestE2ENamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						ControllerResources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						NamespaceScoped: namespaceScopedParam,
					},
				}

				Expect(k8sClient.Create(ctx, &rmWithResources)).To(Succeed())

				Eventually(rmWithResources, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmWithResources.Namespace},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())

				Expect(deployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(*rmWithResources.Spec.ControllerResources))

				By("updating RolloutManager to use a different CPU limit")

				err := k8s.UpdateWithoutConflict(ctx, &rmWithResources, k8sClient, func(obj client.Object) {
					rm, ok := obj.(*rolloutsmanagerv1alpha1.RolloutManager)
					Expect(ok).To(BeTrue())

					rm.Spec.ControllerResources.Limits[corev1.ResourceCPU] = resource.MustParse("555m")

				})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() bool {
					deployment := appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmWithResources.Namespace},
					}
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment); err != nil {
						return false
					}
					return deployment.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU] == resource.MustParse("555m")

				}, "1m", "1s").Should(BeTrue(), "Deployment should switch to the new CPU limit on update of RolloutManager CR")
			})
		})

		DescribeTable("RolloutManager is initially created with a given SkipNotificationSecretDeployment (true/false), then it swaps", func(initialSkipNotificationValue bool) {

			By(fmt.Sprintf("creating RolloutManager with SkipNotificationSecretDeployment set to '%v'", initialSkipNotificationValue))

			rolloutsManager := rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-rollouts-manager-with-skip-notification-secret",
					Namespace: fixture.TestE2ENamespace,
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
					NamespaceScoped:                  namespaceScopedParam,
					SkipNotificationSecretDeployment: initialSkipNotificationValue,
				},
			}

			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			Eventually(rolloutsManager, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

			secret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsNotificationSecretName, Namespace: rolloutManager.Namespace},
			}
			if rolloutsManager.Spec.SkipNotificationSecretDeployment {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).ToNot(Succeed())
			} else {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).To(Succeed())
			}

			By(fmt.Sprintf("setting the SkipNotificationSecretDeployment to '%v'", !initialSkipNotificationValue))
			err := k8s.UpdateWithoutConflict(ctx, &rolloutsManager, k8sClient, func(obj client.Object) {
				rmObj, ok := obj.(*rolloutsmanagerv1alpha1.RolloutManager)
				Expect(ok).To(BeTrue())
				rmObj.Spec.SkipNotificationSecretDeployment = !initialSkipNotificationValue
			})
			Expect(err).ToNot(HaveOccurred())

			if rolloutsManager.Spec.SkipNotificationSecretDeployment {
				Eventually(&secret, "10s", "1s").ShouldNot(k8s.ExistByName(k8sClient))

			} else {
				Eventually(&secret, "10s", "1s").Should(k8s.ExistByName(k8sClient))
			}

		},
			Entry("skipNotification is initially true, then set to false", true),
			Entry("skipNotification is initially false, then set to true", false),
		)

		When("A RolloutManager is deleted but the notification secret is owned by another controller", func() {
			It("should not delete the secret", func() {

				rolloutsManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-rollouts-manager-without-secret",
						Namespace: fixture.TestE2ENamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						NamespaceScoped:                  namespaceScopedParam,
						SkipNotificationSecretDeployment: true,
					},
				}

				Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

				Eventually(rolloutsManager, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				secret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsNotificationSecretName, Namespace: rolloutManager.Namespace},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&secret), &secret)).ToNot(Succeed())

				By("Creating the secret with another owner")
				secret.OwnerReferences = append(secret.OwnerReferences, metav1.OwnerReference{
					Name:       "another-owner",
					APIVersion: "v1",
					Kind:       "OwnerKind",
					UID:        "1234",
				})
				Expect(k8sClient.Create(ctx, &secret)).To(Succeed())
				Eventually(&secret, "10s", "1s").Should(k8s.ExistByName(k8sClient))

				By("Deleting the RolloutManager")
				Expect(k8sClient.Delete(ctx, &rolloutsManager)).To(Succeed())

				Eventually(&secret, "10s", "1s").Should(k8s.ExistByName(k8sClient))

			})
		})

		It("Should add, update, and remove traffic and metric plugins through RolloutManager CR", func() {
			rolloutsManager := rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollouts-manager",
					Namespace: fixture.TestE2ENamespace,
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
					NamespaceScoped: namespaceScopedParam,
					Plugins: rolloutsmanagerv1alpha1.Plugins{
						TrafficManagement: []rolloutsmanagerv1alpha1.Plugin{
							{
								Name:     "argoproj-labs/gatewayAPI",
								Location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.4.0/gatewayapi-plugin-linux-amd64",
								SHA256:   "d00ce783dc0eb30ac2e203bdfdac5300a4b47695267a7fa57474e0a4a7376afe",
							},
						},
						Metric: []rolloutsmanagerv1alpha1.Plugin{
							{
								Name:     "argoproj-labs/prometheus",
								Location: "https://github.com/argoproj-labs/sample-rollouts-metric-plugin/releases/download/v0.0.3/metric-plugin-linux-amd64",
								SHA256:   "4acc0e7a2c99bd8fe2d9b8ee6da08d531ee3a4373c55c77e3b5ee55c866fae69",
							}},
					},
				},
			}

			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify that RolloutManager is successfully created.")
			Eventually(rolloutsManager, "4m", "5s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

			By("Verify traffic and metric plugin is added to ConfigMap")
			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsConfigMapName, Namespace: rolloutsManager.Namespace},
			}

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&configMap), &configMap)).To(Succeed())
			Expect(configMap.Data[controllers.TrafficRouterPluginConfigMapKey]).To(ContainSubstring(rolloutsManager.Spec.Plugins.TrafficManagement[0].Name))
			Expect(configMap.Data[controllers.TrafficRouterPluginConfigMapKey]).To(ContainSubstring(rolloutsManager.Spec.Plugins.TrafficManagement[0].Location))
			Expect(configMap.Data[controllers.TrafficRouterPluginConfigMapKey]).To(ContainSubstring(rolloutsManager.Spec.Plugins.TrafficManagement[0].SHA256))
			Expect(configMap.Data[controllers.MetricPluginConfigMapKey]).To(ContainSubstring(rolloutsManager.Spec.Plugins.Metric[0].Name))
			Expect(configMap.Data[controllers.MetricPluginConfigMapKey]).To(ContainSubstring(rolloutsManager.Spec.Plugins.Metric[0].Location))
			Expect(configMap.Data[controllers.MetricPluginConfigMapKey]).To(ContainSubstring(rolloutsManager.Spec.Plugins.Metric[0].SHA256))
			Expect(configMap.Data[controllers.MetricPluginConfigMapKey_PreviousInvalidKey]).ToNot(ContainSubstring(rolloutsManager.Spec.Plugins.Metric[0].Name))
			Expect(configMap.Data[controllers.MetricPluginConfigMapKey_PreviousInvalidKey]).ToNot(ContainSubstring(rolloutsManager.Spec.Plugins.Metric[0].Location))

			By("Update traffic and metric plugins")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManager), &rolloutsManager)).To(Succeed())

			rolloutsManager.Spec.Plugins.TrafficManagement[0].Location = "https://test-update-traffic-plugin"
			rolloutsManager.Spec.Plugins.Metric[0].Location = "https://test-update-metric-plugin"

			By("Get existing Rollouts Pod(s) before update")
			var oldPods corev1.PodList
			Expect(k8sClient.List(ctx, &oldPods, client.InNamespace(rolloutsManager.Namespace), client.MatchingLabels{"app.kubernetes.io/name": "argo-rollouts"})).To(Succeed())
			Expect(oldPods.Items).To(HaveLen(1))

			By("Update RolloutManager CR to use placeholder values, so we can verify ConfigMap is updated")
			Expect(k8sClient.Update(ctx, &rolloutsManager)).To(Succeed())

			By("Verify traffic and metric plugin is updated in ConfigMap")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&configMap), &configMap); err != nil {
					return false
				}
				return strings.Contains(configMap.Data[controllers.TrafficRouterPluginConfigMapKey], rolloutsManager.Spec.Plugins.TrafficManagement[0].Name) &&
					strings.Contains(configMap.Data[controllers.TrafficRouterPluginConfigMapKey], rolloutsManager.Spec.Plugins.TrafficManagement[0].Location) &&
					strings.Contains(configMap.Data[controllers.TrafficRouterPluginConfigMapKey], rolloutsManager.Spec.Plugins.TrafficManagement[0].SHA256) &&
					strings.Contains(configMap.Data[controllers.MetricPluginConfigMapKey], rolloutsManager.Spec.Plugins.Metric[0].Name) &&
					strings.Contains(configMap.Data[controllers.MetricPluginConfigMapKey], rolloutsManager.Spec.Plugins.Metric[0].Location) &&
					strings.Contains(configMap.Data[controllers.MetricPluginConfigMapKey], rolloutsManager.Spec.Plugins.Metric[0].SHA256) &&
					!strings.Contains(configMap.Data[controllers.MetricPluginConfigMapKey_PreviousInvalidKey], rolloutsManager.Spec.Plugins.Metric[0].Name)

			}, "1m", "1s").Should(BeTrue())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManager), &rolloutsManager)).To(Succeed())

			By("Remove plugins from RolloutManager CR")

			Expect(k8s.UpdateWithoutConflict(ctx, &rolloutsManager, k8sClient, func(obj client.Object) {

				rmo, ok := obj.(*rolloutsmanagerv1alpha1.RolloutManager)
				Expect(ok).To(BeTrue())

				rmo.Spec.Plugins.TrafficManagement = []rolloutsmanagerv1alpha1.Plugin{}
				rmo.Spec.Plugins.Metric = []rolloutsmanagerv1alpha1.Plugin{}

			})).To(Succeed())

			By("Verify that traffic and metric plugins are removed from ConfigMap")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&configMap), &configMap); err != nil {
					return false
				}
				return !strings.Contains(configMap.Data[controllers.TrafficRouterPluginConfigMapKey], "gatewayAPI") &&
					!strings.Contains(configMap.Data[controllers.MetricPluginConfigMapKey], "prometheus")
			}, "1m", "1s").Should(BeTrue())

			By("Verifying the old pod is deleted")
			Eventually(&oldPods.Items[0], "60s", "1s").Should(k8s.NotExistByName(k8sClient))

			var newPods corev1.PodList

			By("Verify Rollouts Pod is restarted")
			Eventually(func() bool {

				if err := k8sClient.List(ctx, &newPods, client.InNamespace(rolloutsManager.Namespace), client.MatchingLabels{"app.kubernetes.io/name": "argo-rollouts"}); err != nil {
					return false
				}

				return len(newPods.Items) == 1

			}, "60s", "1s").Should(BeTrue())

			Expect(newPods.Items[0].Name).NotTo(Equal(oldPods.Items[0].Name)) // Ensure the Pod names are different
		})

		When("the argo-rollouts-config contains the old, incorrect metric plugin key in the ConfiMap", func() {
			It("should remove the key when the RolloutManager CR is reconciled", func() {
				rolloutsManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rollouts-manager-old-key",
						Namespace: fixture.TestE2ENamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						NamespaceScoped: namespaceScopedParam,
						Plugins: rolloutsmanagerv1alpha1.Plugins{
							Metric: []rolloutsmanagerv1alpha1.Plugin{
								{
									Name:     "argoproj-labs/prometheus",
									Location: "https://github.com/argoproj-labs/sample-rollouts-metric-plugin/releases/download/v0.0.3/metric-plugin-linux-amd64",
									SHA256:   "4acc0e7a2c99bd8fe2d9b8ee6da08d531ee3a4373c55c77e3b5ee55c866fae69",
								},
							},
						},
					},
				}

				Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

				By("Verify that RolloutManager is successfully created")
				Eventually(rolloutsManager, "4m", "5s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				configMap := corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: controllers.DefaultRolloutsConfigMapName, Namespace: rolloutsManager.Namespace},
				}

				By("Verify metric plugin is under the new key and not the old key")
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&configMap), &configMap)).To(Succeed())
				Expect(configMap.Data[controllers.MetricPluginConfigMapKey]).To(ContainSubstring("prometheus"))
				Expect(configMap.Data[controllers.MetricPluginConfigMapKey]).To(ContainSubstring(rolloutsManager.Spec.Plugins.Metric[0].SHA256))
				Expect(configMap.Data[controllers.MetricPluginConfigMapKey_PreviousInvalidKey]).NotTo(ContainSubstring("prometheus"))

				By("Manually inject data under the old invalid key into the ConfigMap")
				Expect(k8s.UpdateWithoutConflict(ctx, &configMap, k8sClient, func(obj client.Object) {
					cm, ok := obj.(*corev1.ConfigMap)
					Expect(ok).To(BeTrue())
					cm.Data[controllers.MetricPluginConfigMapKey_PreviousInvalidKey] = cm.Data[controllers.MetricPluginConfigMapKey]
				})).To(Succeed())

				By("Trigger re-reconcile of RolloutManager by updating an annotation")
				Expect(k8s.UpdateWithoutConflict(ctx, &rolloutsManager, k8sClient, func(obj client.Object) {
					annots := obj.GetAnnotations()
					if annots == nil {
						annots = map[string]string{}
					}
					annots["trigger-reconcile"] = "true"
					obj.SetAnnotations(annots)
				})).To(Succeed())

				By("Verify the old invalid key is removed and the new key still has the correct data")
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&configMap), &configMap); err != nil {
						GinkgoWriter.Println(err)
						return false
					}

					_, oldKeyExists := configMap.Data[controllers.MetricPluginConfigMapKey_PreviousInvalidKey]
					res := !oldKeyExists &&
						strings.Contains(configMap.Data[controllers.MetricPluginConfigMapKey], "prometheus") &&
						strings.Contains(configMap.Data[controllers.MetricPluginConfigMapKey], rolloutsManager.Spec.Plugins.Metric[0].Location) &&
						strings.Contains(configMap.Data[controllers.MetricPluginConfigMapKey], rolloutsManager.Spec.Plugins.Metric[0].SHA256)

					if !res {
						GinkgoWriter.Println("configMap:", configMap.Data)
					}

					return res
				}, "1m", "1s").Should(BeTrue())
			})
		})

		When("a namespace-scoped RolloutManager is installed into a namespace that previously contained a cluster-scoped RolloutManager, or vice versa", func() {

			It("should cleanup any cluster/role/rolebinding resources that are present in the namespace, that do not match the current .spec.namespaceScoped value of the RolloutManager CR", func() {

				var fakeRole rbacv1.Role
				var fakeRoleBinding rbacv1.RoleBinding

				var fakeClusterRole rbacv1.ClusterRole
				var fakeClusterRoleBinding rbacv1.ClusterRoleBinding

				By("creating ClusterRole/Binding in the namespace-scoped case, and Role/Binding in the cluster-scoped case")

				if namespaceScopedParam {

					fakeClusterRole = rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name:      controllers.DefaultArgoRolloutsResourceName,
							Namespace: rolloutManager.Namespace,
						},
					}
					Expect(k8sClient.Create(ctx, &fakeClusterRole)).To(Succeed())

					fakeClusterRoleBinding = rbacv1.ClusterRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      controllers.DefaultArgoRolloutsResourceName,
							Namespace: rolloutManager.Namespace,
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.GroupName,
							Kind:     "ClusterRole",
							Name:     fakeClusterRole.Name,
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:      rbacv1.ServiceAccountKind,
								Name:      controllers.DefaultArgoRolloutsResourceName,
								Namespace: rolloutManager.Namespace,
							},
						},
					}
					Expect(k8sClient.Create(ctx, &fakeClusterRoleBinding)).To(Succeed())

				} else {

					fakeRole = rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      controllers.DefaultArgoRolloutsResourceName,
							Namespace: rolloutManager.Namespace,
						},
					}
					Expect(k8sClient.Create(ctx, &fakeRole)).To(Succeed())

					fakeRoleBinding = rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      controllers.DefaultArgoRolloutsResourceName,
							Namespace: rolloutManager.Namespace,
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.GroupName,
							Kind:     "Role",
							Name:     fakeRole.Name,
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:      rbacv1.ServiceAccountKind,
								Name:      controllers.DefaultArgoRolloutsResourceName,
								Namespace: rolloutManager.Namespace,
							},
						},
					}
					Expect(k8sClient.Create(ctx, &fakeRoleBinding)).To(Succeed())

				}

				By("creating RolloutManager and waiting for it to be available")
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "1m", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				if namespaceScopedParam {

					By("verifying that in the namespace-scoped case, the cluster-scoped resources are deleted after reconciliation")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&fakeClusterRole), &fakeClusterRole)).ToNot(Succeed())
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&fakeClusterRoleBinding), &fakeClusterRoleBinding)).ToNot(Succeed())

				} else {

					By("verifying that in the cluster-scoped case, the namespace-scoped resources are deleted after reconciliation")
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&fakeRole), &fakeRole)).ToNot(Succeed())
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&fakeRoleBinding), &fakeRoleBinding)).ToNot(Succeed())

				}

			})
		})

		When("A RolloutManager is created", func() {
			It("should not have a readiness probe on the deployment, and should remove one if it already exists", func() {
				By("creating a RolloutManager and waiting for it to be available")
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))
				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllers.DefaultArgoRolloutsResourceName,
						Namespace: rolloutManager.Namespace,
					},
				}
				Eventually(&deployment, "10s", "1s").Should(k8s.ExistByName(k8sClient))
				By("verifying that the deployment does not have a readiness probe on new install")
				Expect(deployment.Spec.Template.Spec.Containers[0].ReadinessProbe).To(BeNil())
				By("manually adding a readiness probe to the deployment to simulate an existing install")
				err := k8s.UpdateWithoutConflict(ctx, &deployment, k8sClient, func(obj client.Object) {
					depl, ok := obj.(*appsv1.Deployment)
					Expect(ok).To(BeTrue())
					depl.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
						FailureThreshold: int32(5),
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/metrics",
								Port: intstr.FromString("metrics"),
							},
						},
						InitialDelaySeconds: int32(10),
						PeriodSeconds:       int32(5),
						SuccessThreshold:    int32(1),
						TimeoutSeconds:      int32(4),
					}
				})
				Expect(err).ToNot(HaveOccurred())
				By("verifying that the operator removes the readiness probe on the next reconciliation")
				Eventually(func() *corev1.Probe {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&deployment), &deployment)).To(Succeed())
					return deployment.Spec.Template.Spec.Containers[0].ReadinessProbe
				}, "30s", "1s").Should(BeNil())
			})
		})
		It("should contain two replicas and the '--leader-elect' argument set to true, and verify that the anti-affinity rule is added by default when HA is enabled", func() {
			By("Create cluster-scoped RolloutManager in a namespace.")

			rolloutsManager := rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollouts-manager",
					Namespace: fixture.TestE2ENamespace,
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
					NamespaceScoped: namespaceScopedParam,
					HA: &rolloutsmanagerv1alpha1.RolloutManagerHASpec{
						Enabled: true,
					},
				},
			}

			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			depl := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controllers.DefaultArgoRolloutsResourceName,
					Namespace: fixture.TestE2ENamespace,
				},
			}

			// In this test we don't check whether RolloutManager has phase: Available, and we don't check if Deployment is ready.
			// This is because our E2E tests run in a single node cluster, which prevents HA deployments from being fully scheduled.
			Eventually(&depl, "60s", "1s").Should(k8s.ExistByName(k8sClient))

			replicas := int32(2)
			Expect(depl.Spec.Replicas).To(Equal(&replicas))
			Expect(depl.Spec.Template.Spec.Containers[0].Args).To(ContainElements("--leader-elect", "true"))

			By("verifying that the anti-affinity rules are set correctly")
			affinity := depl.Spec.Template.Spec.Affinity
			Expect(affinity).NotTo(BeNil())
			Expect(affinity.PodAntiAffinity).NotTo(BeNil())

			By("Verify PreferredDuringSchedulingIgnoredDuringExecution")
			preferred := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			Expect(preferred).To(HaveLen(1))
			Expect(preferred[0].Weight).To(Equal(int32(100)))
			Expect(preferred[0].PodAffinityTerm.TopologyKey).To(Equal(controllers.TopologyKubernetesZoneLabel))
			Expect(preferred[0].PodAffinityTerm.LabelSelector.MatchLabels).To(Equal(depl.Spec.Selector.MatchLabels))

			By("Verify RequiredDuringSchedulingIgnoredDuringExecution")
			required := affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			Expect(required).To(HaveLen(1))
			Expect(required[0].TopologyKey).To(Equal(controllers.KubernetesHostnameLabel))
			Expect(required[0].LabelSelector.MatchLabels).To(Equal(depl.Spec.Selector.MatchLabels))
		})

		When("an image pull secret is labeled for propagation in the operator namespace", func() {
			managedRef := corev1.LocalObjectReference{Name: controllers.RolloutManagerImagePullSecretName}

			var (
				operatorNS  string
				rmNamespace string
			)

			// Marks the source secrets these tests create in the operator namespace, so cleanup never touches other secrets.
			const e2eTestSecretLabel = "argo-rollouts-manager-e2e/image-pull-secret-test" // #nosec G101

			BeforeEach(func() {
				// OPERATOR_NAMESPACE controls where the controller looks for labeled pull secrets.
				// Default must match hack/start-rollouts-manager-for-e2e-tests.sh and hack/run-rollouts-manager-e2e-tests.sh.
				operatorNS = os.Getenv("OPERATOR_NAMESPACE")
				if operatorNS == "" {
					operatorNS = "argo-rollouts"
				}

				// RM lives in its own namespace so the cross-namespace copy path is always exercised.
				// Must be listed in CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES (hack/start-rollouts-manager-for-e2e-tests.sh)
				// so the cluster-scoped run accepts it.
				rmNamespace = "test-rom-ns-1"
				Expect(operatorNS).ToNot(Equal(rmNamespace))
				Expect(CreateNamespace(ctx, k8sClient, rmNamespace)).To(Succeed())

				// EnsureCleanSlate recreates fixture.TestE2ENamespace; any other operator namespace must be ensured here.
				if operatorNS != fixture.TestE2ENamespace {
					ns := &corev1.Namespace{}
					err := k8sClient.Get(ctx, client.ObjectKey{Name: operatorNS}, ns)
					if apierrors.IsNotFound(err) {
						Expect(k8sClient.Create(ctx, &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{Name: operatorNS},
						})).To(Succeed())
					} else {
						Expect(err).ToNot(HaveOccurred())
					}
				}

				// Delete only the secrets these tests create, before and after each spec (including failed specs),
				// so a leftover labeled secret can't leak into later specs when operatorNS is not recreated by EnsureCleanSlate.
				// Pre-existing secrets in operatorNS are preserved: sources are matched by e2eTestSecretLabel, and the
				// operator's copy (made by the same-namespace test) by its fixed name plus the copied-secret label.
				deleteTestSecrets := func() {
					Expect(k8sClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(operatorNS),
						client.MatchingLabels{e2eTestSecretLabel: "true"})).To(Succeed())

					copied := &corev1.Secret{}
					err := k8sClient.Get(ctx, client.ObjectKey{Name: controllers.RolloutManagerImagePullSecretName, Namespace: operatorNS}, copied)
					if apierrors.IsNotFound(err) {
						return
					}
					Expect(err).ToNot(HaveOccurred())
					if _, ok := copied.Labels[controllers.RolloutManagerImagePullSecretCopiedLabel]; ok {
						Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, copied))).To(Succeed())
					}
				}
				deleteTestSecrets()
				DeferCleanup(deleteTestSecrets)
			})

			It("should add a labeled pull secret to the rollouts SA", func() {
				By("creating a pull secret with the propagate label in the operator namespace")
				pullSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pull-secret",
						Namespace: operatorNS,
						Labels: map[string]string{
							controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
							e2eTestSecretLabel: "true",
						},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
					},
				}
				Expect(k8sClient.Create(ctx, pullSecret)).To(Succeed())

				By("creating a RolloutManager")
				rolloutManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "imagepullsecret-rm",
						Namespace: rmNamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						NamespaceScoped: namespaceScopedParam,
					},
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				By("verifying the secret is copied to the RM namespace")
				copiedSecret := &corev1.Secret{}
				Eventually(func() error {
					return k8sClient.Get(ctx, client.ObjectKey{Name: controllers.RolloutManagerImagePullSecretName, Namespace: rmNamespace}, copiedSecret)
				}, "15s", "1s").Should(Succeed())
				Expect(copiedSecret.Labels[controllers.RolloutManagerImagePullSecretCopiedLabel]).To(Equal("test-pull-secret"))
				Expect(copiedSecret.Data).To(Equal(pullSecret.Data))

				By("verifying the rollouts SA has imagePullSecrets set")
				sa := &corev1.ServiceAccount{}
				Eventually(func() []corev1.LocalObjectReference {
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
					return sa.ImagePullSecrets
				}, "15s", "1s").Should(ContainElement(managedRef))

				By("verifying the Deployment pod template is annotated so pods are rolled to pick up the pull secret")
				Eventually(func() map[string]string {
					depl := &appsv1.Deployment{}
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, depl)
					return depl.Spec.Template.Annotations
				}, "15s", "1s").Should(HaveKeyWithValue(controllers.RolloutManagerImagePullSecretAnnotation, controllers.RolloutManagerImagePullSecretName))
			})

			It("should remove only the managed SA imagePullSecrets entry when the label is removed", func() {
				By("creating a pull secret with the propagate label")
				pullSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pull-secret-removal",
						Namespace: operatorNS,
						Labels: map[string]string{
							controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
							e2eTestSecretLabel: "true",
						},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
					},
				}
				Expect(k8sClient.Create(ctx, pullSecret)).To(Succeed())

				By("creating a RolloutManager")
				rolloutManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "imagepullsecret-removal-rm",
						Namespace: rmNamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						NamespaceScoped: namespaceScopedParam,
					},
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				By("waiting for SA to have imagePullSecrets")
				sa := &corev1.ServiceAccount{}
				Eventually(func() []corev1.LocalObjectReference {
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
					return sa.ImagePullSecrets
				}, "15s", "1s").Should(ContainElement(managedRef))

				By("adding a user/platform injected pull secret to the SA")
				foreignRef := corev1.LocalObjectReference{Name: "user-injected-pull-secret"}
				Expect(k8s.UpdateWithoutConflict(ctx, sa, k8sClient, func(obj client.Object) {
					o := obj.(*corev1.ServiceAccount)
					o.ImagePullSecrets = append(o.ImagePullSecrets, foreignRef)
				})).To(Succeed())

				By("verifying the operator keeps the injected entry alongside the managed one")
				Consistently(func() []corev1.LocalObjectReference {
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
					return sa.ImagePullSecrets
				}, "10s", "2s").Should(ContainElements(managedRef, foreignRef))

				By("removing the propagate label from the source secret")
				Expect(k8s.UpdateWithoutConflict(ctx, pullSecret, k8sClient, func(obj client.Object) {
					delete(obj.(*corev1.Secret).Labels, controllers.RolloutManagerImagePullSecretPropagateLabel)
				})).To(Succeed())

				By("verifying only the managed entry is removed from the SA")
				Eventually(func() []corev1.LocalObjectReference {
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
					return sa.ImagePullSecrets
				}, "30s", "1s").Should(SatisfyAll(Not(ContainElement(managedRef)), ContainElement(foreignRef)))

				By("verifying the copied secret is deleted from the RM namespace")
				copiedSecret := &corev1.Secret{}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: controllers.RolloutManagerImagePullSecretName, Namespace: rmNamespace}, copiedSecret)
					return apierrors.IsNotFound(err)
				}, "30s", "1s").Should(BeTrue())
			})

			It("should remove the propagated secret and fail the RolloutManager when multiple secrets have the propagation label", func() {
				newLabeledSecret := func(name string) *corev1.Secret {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: operatorNS,
							Labels: map[string]string{
								controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
								e2eTestSecretLabel: "true",
							},
						},
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
						},
					}
				}
				saPullSecrets := func() []corev1.LocalObjectReference {
					sa := &corev1.ServiceAccount{}
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
					return sa.ImagePullSecrets
				}

				By("creating one labeled pull secret and a RolloutManager")
				Expect(k8sClient.Create(ctx, newLabeledSecret("pull-secret-a"))).To(Succeed())
				rolloutManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "imagepullsecret-multi-rm",
						Namespace: rmNamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						NamespaceScoped: namespaceScopedParam,
					},
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))
				Eventually(saPullSecrets, "15s", "1s").Should(ContainElement(managedRef))

				By("labeling a second pull secret")
				Expect(k8sClient.Create(ctx, newLabeledSecret("pull-secret-b"))).To(Succeed())

				By("verifying the RolloutManager reports the error")
				Eventually(func() string {
					rm := &rolloutsmanagerv1alpha1.RolloutManager{}
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutManager), rm); err != nil {
						return ""
					}
					for _, c := range rm.Status.Conditions {
						if c.Type == rolloutsmanagerv1alpha1.RolloutManagerConditionType {
							return c.Message
						}
					}
					return ""
				}, "30s", "1s").Should(ContainSubstring("found 2 secrets labeled"))

				By("verifying the managed SA entry and the copy are removed")
				Eventually(saPullSecrets, "15s", "1s").ShouldNot(ContainElement(managedRef))
				err := k8sClient.Get(ctx, client.ObjectKey{Name: controllers.RolloutManagerImagePullSecretName, Namespace: rmNamespace}, &corev1.Secret{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected no image pull secret copy in RM namespace")

				By("removing one label so only one secret remains labeled")
				secretA := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "pull-secret-a", Namespace: operatorNS}}
				Expect(k8s.UpdateWithoutConflict(ctx, secretA, k8sClient, func(obj client.Object) {
					delete(obj.(*corev1.Secret).Labels, controllers.RolloutManagerImagePullSecretPropagateLabel)
				})).To(Succeed())

				By("verifying the RolloutManager recovers and the SA has the managed entry again")
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HaveSuccessCondition())
				Eventually(saPullSecrets, "15s", "1s").Should(ContainElement(managedRef))

				By("verifying the copy is sourced from pull-secret-b")
				copiedSecret := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: controllers.RolloutManagerImagePullSecretName, Namespace: rmNamespace}, copiedSecret)).To(Succeed())
				Expect(copiedSecret.Labels[controllers.RolloutManagerImagePullSecretCopiedLabel]).To(Equal("pull-secret-b"))
			})

			It("should not overwrite an existing unmanaged secret with the same name", func() {
				By("creating an unmanaged secret with the copy's fixed name in the RM namespace")
				preExisting := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllers.RolloutManagerImagePullSecretName,
						Namespace: rmNamespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{"key": []byte("original")},
				}
				Expect(k8sClient.Create(ctx, preExisting)).To(Succeed())

				By("creating a labeled pull secret in the operator namespace")
				Expect(k8sClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "conflict-pull-secret",
						Namespace: operatorNS,
						Labels: map[string]string{
							controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
							e2eTestSecretLabel: "true",
						},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
					},
				})).To(Succeed())

				By("creating a RolloutManager")
				rolloutManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "conflict-rm",
						Namespace: rmNamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						NamespaceScoped: namespaceScopedParam,
					},
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				By("verifying the pre-existing secret is not overwritten")
				existing := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: controllers.RolloutManagerImagePullSecretName, Namespace: rmNamespace}, existing)).To(Succeed())
				Expect(existing.Data["key"]).To(Equal([]byte("original")))
				Expect(existing.Labels).ToNot(HaveKey(controllers.RolloutManagerImagePullSecretCopiedLabel))

				By("verifying the SA does not reference the unmanaged secret")
				sa := &corev1.ServiceAccount{}
				Consistently(func() []corev1.LocalObjectReference {
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
					return sa.ImagePullSecrets
				}, "10s", "2s").ShouldNot(ContainElement(managedRef))
			})

			It("should update the copied secret when source pull secret data changes", func() {
				By("creating a pull secret")
				pullSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "data-change-secret",
						Namespace: operatorNS,
						Labels: map[string]string{
							controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
							e2eTestSecretLabel: "true",
						},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
					},
				}
				Expect(k8sClient.Create(ctx, pullSecret)).To(Succeed())

				By("creating a RolloutManager")
				rolloutManager := rolloutsmanagerv1alpha1.RolloutManager{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "data-change-rm",
						Namespace: rmNamespace,
					},
					Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
						NamespaceScoped: namespaceScopedParam,
					},
				}
				Expect(k8sClient.Create(ctx, &rolloutManager)).To(Succeed())
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				By("waiting for SA to have imagePullSecrets")
				sa := &corev1.ServiceAccount{}
				Eventually(func() []corev1.LocalObjectReference {
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: rmNamespace}, sa)
					return sa.ImagePullSecrets
				}, "15s", "1s").Should(ContainElement(managedRef))

				By("updating the pull secret data")
				newData := []byte(`{"auths":{"registry2.example.com":{"auth":"bmV3cGFzcw=="}}}`)
				Expect(k8s.UpdateWithoutConflict(ctx, pullSecret, k8sClient, func(obj client.Object) {
					obj.(*corev1.Secret).Data[corev1.DockerConfigJsonKey] = newData
				})).To(Succeed())

				By("verifying the copied secret data is updated")
				copiedSecret := &corev1.Secret{}
				Eventually(func() []byte {
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.RolloutManagerImagePullSecretName, Namespace: rmNamespace}, copiedSecret)
					return copiedSecret.Data[corev1.DockerConfigJsonKey]
				}, "15s", "1s").Should(Equal(newData))
			})

			It("should copy the source secret even when the RM is in the operator namespace", func() {
				By("creating a pull secret with the propagate label in the operator namespace")
				Expect(k8sClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pull-secret",
						Namespace: operatorNS,
						Labels: map[string]string{
							controllers.RolloutManagerImagePullSecretPropagateLabel: "true",
							e2eTestSecretLabel: "true",
						},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`),
					},
				})).To(Succeed())

				By("creating a RolloutManager in the operator namespace")
				rolloutManager, err := CreateRolloutManager(ctx, k8sClient, "same-ns-rm", operatorNS, namespaceScopedParam)
				Expect(err).ToNot(HaveOccurred())
				// EnsureCleanSlate only recreates the default operator namespace; with a custom OPERATOR_NAMESPACE
				// this RM would otherwise outlive the spec and break later cluster-scoped tests.
				DeferCleanup(func() {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &rolloutManager))).To(Succeed())
					Eventually(&rolloutManager, "60s", "1s").Should(k8s.NotExistByName(k8sClient))
				})
				Eventually(rolloutManager, "60s", "1s").Should(rolloutManagerFixture.HavePhase(rolloutsmanagerv1alpha1.PhaseAvailable))

				By("verifying the rollouts SA references the managed copy")
				sa := &corev1.ServiceAccount{}
				Eventually(func() []corev1.LocalObjectReference {
					_ = k8sClient.Get(ctx, client.ObjectKey{Name: controllers.DefaultArgoRolloutsResourceName, Namespace: operatorNS}, sa)
					return sa.ImagePullSecrets
				}, "15s", "1s").Should(ContainElement(managedRef))

				By("verifying the copy exists in the operator namespace")
				copiedSecret := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: controllers.RolloutManagerImagePullSecretName, Namespace: operatorNS}, copiedSecret)).To(Succeed())
				Expect(copiedSecret.Labels[controllers.RolloutManagerImagePullSecretCopiedLabel]).To(Equal("test-pull-secret"))
			})
		})
	})
}
