package rollouts

import (
	"context"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ConfigMap Test", func() {
	var ctx context.Context
	var a v1alpha1.RolloutManager
	var r *RolloutManagerReconciler
	var sa *corev1.ServiceAccount
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

		existingDeployment := deploymentCR(DefaultArgoRolloutsResourceName, a.Namespace, DefaultArgoRolloutsResourceName, []string{"plugin-bin-test", "tmp-test"}, "linux-test", sa.Name, a)
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

	// Commented out because we are overwriting user-defined plugin values in the ConfigMap
	// with the plugins defined in the CR. This will be removed once the PR has been reviewed.

	/*It("verifies that the config map reconciler will not overwrite a custom plugin that is added to the ConfigMap by the user", func() {

	    // By("creating a ConfigMap containing default Openshift")
	    expectedConfigMap := &corev1.ConfigMap{
	        ObjectMeta: metav1.ObjectMeta{
	            Name:      DefaultRolloutsConfigMapName,
	            Namespace: a.Namespace,
	            Labels: map[string]string{
	                "app.kubernetes.io/name": DefaultRolloutsConfigMapName,
	            },
	        },
	    }

	    By("calling reconcileConfigMap, which will add the default plugin to the ConfigMap")
	    Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

	    By("fetching the ConfigMap")
	    fetchedConfigMap := &corev1.ConfigMap{}
	    Expect(fetchObject(ctx, r.Client, a.Namespace, expectedConfigMap.Name, fetchedConfigMap)).To(Succeed())
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).ToNot(ContainSubstring("test/plugin"))

	    By("adding a new trafficRouter plugin to test the update plugin logic")
	    trafficRouterPlugins := []pluginItem{
	        {
	            Name:     "test/plugin",
	            Location: "https://test-path",
	        },
	    }

	    newConfigMap := fetchedConfigMap.DeepCopy()
	    {
	        pluginString, err := yaml.Marshal(trafficRouterPlugins)
	        Expect(err).ToNot(HaveOccurred())

	        newConfigMap.Data = map[string]string{
	            TrafficRouterPluginConfigMapKey: string(pluginString),
	        }
	    }

	    By("updating the ConfigMap to contain only a user provided plugin")
	    Expect(r.Client.Update(ctx, newConfigMap)).To(Succeed())

	    By("calling reconcileConfigMap")
	    Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

	    By("verifying that when ConfigMap is reconciled, it contains both plugins")

	    Expect(fetchObject(ctx, r.Client, a.Namespace, expectedConfigMap.Name, fetchedConfigMap)).To(Succeed())
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring("test/plugin"))
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(r.OpenShiftRoutePluginLocation))

	    By("calling reconcileConfigMap again, to verify nothing changes when reconcile is called again")
	    Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

	    Expect(fetchObject(ctx, r.Client, a.Namespace, expectedConfigMap.Name, fetchedConfigMap)).To(Succeed())
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring("test/plugin"))
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(r.OpenShiftRoutePluginLocation))

	    // overriding this value with new test url to verify whether it updated the existing configMap with the new url
	    r.OpenShiftRoutePluginLocation = "test-updated-url"

	    By("calling reconcileConfigMap")
	    Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

	    Expect(fetchObject(ctx, r.Client, a.Namespace, expectedConfigMap.Name, fetchedConfigMap)).To(Succeed())
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring("test/plugin"))
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
	    Expect(fetchedConfigMap.Data[TrafficRouterPluginConfigMapKey]).To(ContainSubstring("test-updated-url"))
	}) */

	It("verifies traffic and metric plugin creation/modification and ensures OpenShiftRolloutPlugin existence", func() {
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

		By("Call reconcileConfigMap again after update")
		Expect(r.reconcileConfigMap(ctx, a)).ToNot(Succeed(), "the plugin %s cannot be modified or added through the RolloutManager CR", OpenShiftRolloutPluginName)
	})
})
