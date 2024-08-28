package rollouts

import (
	"context"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ConfigMap Test", func() {
	var ctx context.Context
	var a v1alpha1.RolloutManager
	var r *RolloutManagerReconciler
	var sa *corev1.ServiceAccount
	var existingDeployment *appsv1.Deployment
	const trafficrouterPluginLocation = "https://custom-traffic-plugin-location"
	const metricPluginLocation = "https://custom-metric-plugin-location"

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

		existingDeployment = deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin-test", "tmp-test"}, "linux-test", sa.Name, a)
		Expect(r.Client.Create(ctx, existingDeployment)).To(Succeed())

	})

	It("verifies that the default ConfigMap is created if it is not present", func() {
		expectedConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: DefaultRolloutsConfigMapName,
			},
		}

		By("Call reconcileConfigMap")
		Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

		By("Verify that the fetched ConfigMap matches the desired one")

		fetchedConfigMap := &corev1.ConfigMap{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, expectedConfigMap.Name, fetchedConfigMap)).To(Succeed())

		Expect(fetchedConfigMap.Name).To(Equal(expectedConfigMap.Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(r.OpenShiftRoutePluginLocation))

		By("Call reconcileConfigMap again")
		Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

		By("verifying that the data is still present")
		Expect(fetchedConfigMap.Name).To(Equal(expectedConfigMap.Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(r.OpenShiftRoutePluginLocation))

	})

	It("verifies traffic and metric plugin creation/modification and ensures OpenShiftRolloutPlugin existence", func() {
		By("Add a pod that matches the deployment's selector")
		addTestPodToFakeClient(r, a.Namespace, existingDeployment)

		expectedConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: DefaultRolloutsConfigMapName,
			},
		}

		By("Adding traffic and metric plugins through the CR")
		a.Spec.Plugins.TrafficManagement = []v1alpha1.Plugin{
			{Name: "custom-traffic-plugin", Location: trafficrouterPluginLocation},
		}
		a.Spec.Plugins.Metric = []v1alpha1.Plugin{
			{Name: "custom-metric-plugin", Location: metricPluginLocation, SHA256: "sha256-test"},
		}

		Expect(r.Client.Update(ctx, &a)).To(Succeed())

		By("Call reconcileConfigMap")
		Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

		By("Fetched ConfigMap")
		fetchedConfigMap := &corev1.ConfigMap{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, expectedConfigMap.Name, fetchedConfigMap)).To(Succeed())

		By("Verify that the fetched ConfigMap contains OpenShiftRolloutPlugin")
		Expect(fetchedConfigMap.Name).To(Equal(expectedConfigMap.Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(r.OpenShiftRoutePluginLocation))

		By("Verify that the fetched ConfigMap contains plugins added by CR")
		Expect(fetchedConfigMap.Data[MetricPluginConfigMapKey]).To(ContainSubstring(a.Spec.Plugins.Metric[0].Name))
		Expect(fetchedConfigMap.Data[MetricPluginConfigMapKey]).To(ContainSubstring(a.Spec.Plugins.Metric[0].Location))

		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(a.Spec.Plugins.TrafficManagement[0].Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(a.Spec.Plugins.TrafficManagement[0].Location))

		By("Update metric and traffic plugins through RolloutManager CR")
		updatedPluginLocation := "https://test-updated-plugin-location"

		a.Spec.Plugins.TrafficManagement = []v1alpha1.Plugin{
			{Name: "custom-traffic-plugin", Location: updatedPluginLocation},
		}
		a.Spec.Plugins.Metric = []v1alpha1.Plugin{
			{Name: "custom-metric-plugin", Location: updatedPluginLocation, SHA256: "sha256-test"},
		}

		Expect(r.Client.Update(ctx, &a)).To(Succeed())

		By("Call reconcileConfigMap again after update")
		Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

		By("Fetched ConfigMap")
		Expect(fetchObject(ctx, r.Client, a.Namespace, expectedConfigMap.Name, fetchedConfigMap)).To(Succeed())

		By("Verify that the fetched ConfigMap contains OpenShiftRolloutPlugin")
		Expect(fetchedConfigMap.Name).To(Equal(expectedConfigMap.Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(r.OpenShiftRoutePluginLocation))

		By("Verify that ConfigMap is updated with the plugins modified by RolloutManger CR")
		Expect(fetchedConfigMap.Data[MetricPluginConfigMapKey]).To(ContainSubstring(a.Spec.Plugins.Metric[0].Name))
		Expect(fetchedConfigMap.Data[MetricPluginConfigMapKey]).To(ContainSubstring(updatedPluginLocation))

		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(a.Spec.Plugins.TrafficManagement[0].Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(updatedPluginLocation))

		By("Verify that OpenShiftRolloutPlugin is not updated through RolloutManager CR")
		a.Spec.Plugins = v1alpha1.Plugins{
			TrafficManagement: []v1alpha1.Plugin{
				{
					Name:     OpenShiftRolloutPluginName,
					Location: r.OpenShiftRoutePluginLocation,
				},
			},
		}

		Expect(r.Client.Update(ctx, &a)).To(Succeed())

		By("Calling reconcileConfigMap again after the attempt to update OpenShiftRolloutPlugin")
		Expect(r.reconcileConfigMap(ctx, a)).ToNot(Succeed(), "the plugin %s cannot be modified or added through the RolloutManager CR", OpenShiftRolloutPluginName)

		By("Remove plugins from RolloutManager spec should remove plugins from ConfigMap")
		a.Spec.Plugins.TrafficManagement = nil
		a.Spec.Plugins.Metric = nil

		Expect(r.Client.Update(ctx, &a)).To(Succeed())

		By("Call reconcileConfigMap after plugins are removed")
		Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

		By("Fetched ConfigMap after removing plugins")
		Expect(fetchObject(ctx, r.Client, a.Namespace, expectedConfigMap.Name, fetchedConfigMap)).To(Succeed())

		By("Verify that the fetched ConfigMap contains OpenShiftRolloutPlugin after removing plugins from CR")
		Expect(fetchedConfigMap.Name).To(Equal(expectedConfigMap.Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(r.OpenShiftRoutePluginLocation))

		By("Verify that the ConfigMap no longer contains removed plugins")
		Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).NotTo(ContainSubstring("custom-traffic-plugin"))
		Expect(fetchedConfigMap.Data[MetricPluginConfigMapKey]).NotTo(ContainSubstring("custom-metric-plugin"))

		By("Verify that the pod has been deleted after the above update.")
		rolloutsPodList := &corev1.PodList{}
		err := r.Client.List(ctx, rolloutsPodList, client.InNamespace(a.Namespace), client.MatchingLabels(existingDeployment.Spec.Selector.MatchLabels))
		Expect(err).NotTo(HaveOccurred())
		Expect(len(rolloutsPodList.Items)).To(BeNumerically("==", 0))
	})
})

func addTestPodToFakeClient(r *RolloutManagerReconciler, namespace string, deployment *appsv1.Deployment) {
	// Create a test pod with labels that match the deployment's selector
	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rollouts-pod",
			Namespace: namespace,
			Labels:    deployment.Spec.Selector.MatchLabels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  DefaultArgoRolloutsResourceName,
					Image: "argoproj/argo-rollouts:latest",
				},
			},
		},
	}

	// Add the pod to the fake client
	err := r.Client.Create(context.TODO(), testPod)
	Expect(err).ToNot(HaveOccurred())
}
