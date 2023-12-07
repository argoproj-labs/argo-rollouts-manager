package rollouts

import (
	"context"
	"fmt"

	rolloutsApi "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"github.com/argoproj/argo-rollouts/utils/plugin/types"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Reconcile the Rollouts Default Config Map.
func (r *RolloutManagerReconciler) reconcileConfigMap(cr *rolloutsApi.RolloutManager) error {

	desiredConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultRolloutsConfigMapName,
			Namespace: cr.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": DefaultRolloutsConfigMapName,
			},
		},
	}
	trafficRouterPlugins := []types.PluginItem{
		{
			Name:     OpenshiftRolloutPluginName,
			Location: "file://" + OpenshiftRolloutPluginPath,
		},
	}
	pluginString, err := yaml.Marshal(trafficRouterPlugins)
	if err != nil {
		return fmt.Errorf("error marshalling trafficRouterPlugin to string %s", err)
	}
	desiredConfigMap.Data["trafficRouterPlugins"] = string(pluginString)

	actualConfiMap := &corev1.ConfigMap{}

	if err := fetchObject(r.Client, cr.Namespace, desiredConfigMap.Name, actualConfiMap); err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap is not present, create default config map
			log.Info("configMap not found, creating default configmap with openshift route plugin information")
			return r.Client.Create(context.TODO(), desiredConfigMap)
		}
		return fmt.Errorf("failed to get the serviceAccount associated with %s : %s", desiredConfigMap.Name, err)
	}

	var actualTrafficRouterPlugins []types.PluginItem
	if err = yaml.Unmarshal([]byte(actualConfiMap.Data["trafficRouterPlugins"]), &actualTrafficRouterPlugins); err != nil {
		return fmt.Errorf("failed to unmarshal traffic router plugins from ConfigMap: %s", err)
	}

	for _, plugin := range actualTrafficRouterPlugins {
		if plugin.Name == OpenshiftRolloutPluginName {
			// Openshift Route Plugin already present, nothing to do
			return nil
		}
	}

	updatedTrafficRouterPlugins := append(actualTrafficRouterPlugins, trafficRouterPlugins...)

	pluginString, err = yaml.Marshal(updatedTrafficRouterPlugins)
	if err != nil {
		return fmt.Errorf("error marshalling trafficRouterPlugin to string %s", err)
	}

	actualConfiMap.Data["trafficRouterPlugins"] = string(pluginString)

	return r.Client.Update(context.TODO(), actualConfiMap)
}
