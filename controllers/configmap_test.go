package rollouts

import (
	"context"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"github.com/argoproj/argo-rollouts/utils/plugin/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ConfigMap Test", func() {
	var ctx context.Context
	var a *v1alpha1.RolloutManager
	var r *RolloutManagerReconciler

	BeforeEach(func() {
		ctx = context.Background()
		a = makeTestRolloutManager()

		r = makeTestReconciler(a)
		Expect(createNamespace(r, a.Namespace)).To(Succeed())
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

	It("verifies that the config map reconciler will not overwrite a custom plugin that is added to the ConfigMap by the user", func() {

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
		trafficRouterPlugins := []types.PluginItem{
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

	})
})
