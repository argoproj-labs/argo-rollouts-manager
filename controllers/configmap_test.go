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

	It("Test to verify that the default ConfigMap is created if it is not present", func() {
		desiredConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: DefaultRolloutsConfigMapName,
			},
		}

		Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

		By("Fetch the ConfigMap")
		fetchedConfigMap := &corev1.ConfigMap{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, desiredConfigMap.Name, fetchedConfigMap)).To(Succeed())

		By("Verify that the fetched ConfigMap matches the desired one")
		Expect(fetchedConfigMap.Name).To(Equal(desiredConfigMap.Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
	})

	It("Test to verify the presence of the ConfigMap and updates the traffic router plugins", func() {
		desiredConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultRolloutsConfigMapName,
				Namespace: a.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": DefaultRolloutsConfigMapName,
				},
			},
		}

		trafficRouterPlugins := []types.PluginItem{
			{
				Name:     OpenShiftRolloutPluginName,
				Location: r.OpenShiftRoutePluginURL,
			},
		}
		pluginString, err := yaml.Marshal(trafficRouterPlugins)
		Expect(err).ToNot(HaveOccurred())

		desiredConfigMap.Data = map[string]string{
			TrafficRouterPluginKey: string(pluginString),
		}

		By("test to verify when Openshift Route Plugin already present")
		Expect(r.Client.Create(ctx, desiredConfigMap)).To(Succeed())
		Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

		By("Fetch the ConfigMap")
		fetchedConfigMap := &corev1.ConfigMap{}
		Expect(fetchObject(ctx, r.Client, a.Namespace, desiredConfigMap.Name, fetchedConfigMap)).To(Succeed())
		Expect(fetchedConfigMap.Name).To(Equal(desiredConfigMap.Name))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginKey]).To(ContainSubstring(OpenShiftRolloutPluginName))

		By("update trafficRouterPlugins to test the update plugin logic")
		trafficRouterPlugins = []types.PluginItem{
			{
				Name:     "test/plugin",
				Location: "https://test-path",
			},
		}
		pluginString, err = yaml.Marshal(trafficRouterPlugins)
		Expect(err).ToNot(HaveOccurred())

		desiredConfigMap.Data = map[string]string{
			TrafficRouterPluginKey: string(pluginString),
		}

		By("Fetch the ConfigMap")
		Expect(fetchObject(ctx, r.Client, a.Namespace, desiredConfigMap.Name, fetchedConfigMap)).To(Succeed())

		Expect(r.Client.Update(ctx, desiredConfigMap)).To(Succeed())
		Expect(r.reconcileConfigMap(ctx, a)).To(Succeed())

		By("Fetch the ConfigMap")
		Expect(fetchObject(ctx, r.Client, a.Namespace, desiredConfigMap.Name, fetchedConfigMap)).To(Succeed())

		By("Verify that the fetched ConfigMap is updated with the existing plugin")
		Expect(fetchedConfigMap.Data[TrafficRouterPluginKey]).To(ContainSubstring("test/plugin"))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginKey]).To(ContainSubstring(OpenShiftRolloutPluginName))
		Expect(fetchedConfigMap.Data[TrafficRouterPluginKey]).To(ContainSubstring(r.OpenShiftRoutePluginURL))

	})
})
