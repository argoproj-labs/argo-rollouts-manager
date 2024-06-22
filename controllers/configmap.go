package rollouts

import (
	"context"
	"fmt"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// From https://argo-rollouts.readthedocs.io/en/stable/features/traffic-management/plugins/
const TrafficRouterPluginConfigMapKey = "trafficRouterPlugins"

// Reconcile the Rollouts Default Config Map.
func (r *RolloutManagerReconciler) reconcileConfigMap(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) error {

	if r.OpenShiftRoutePluginLocation == "" { // sanity test the plugin value
		return fmt.Errorf("OpenShift Route Plugin location is not set")
	}

	desiredConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultRolloutsConfigMapName,
			Namespace: cr.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": DefaultRolloutsConfigMapName,
			},
		},
	}

	setRolloutsLabelsAndAnnotationsToObject(&desiredConfigMap.ObjectMeta, cr)

	trafficRouterPlugins := []pluginItem{
		{
			Name:     OpenShiftRolloutPluginName,
			Location: r.OpenShiftRoutePluginLocation,
		},
	}
	pluginString, err := yaml.Marshal(trafficRouterPlugins)
	if err != nil {
		return fmt.Errorf("error marshalling trafficRouterPlugin to string %s", err)
	}
	desiredConfigMap.Data = map[string]string{
		TrafficRouterPluginConfigMapKey: string(pluginString),
	}

	actualConfigMap := &corev1.ConfigMap{}

	if err := fetchObject(ctx, r.Client, cr.Namespace, desiredConfigMap.Name, actualConfigMap); err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap is not present, create default config map
			log.Info("configMap not found, creating default configmap with openshift route plugin information")
			return r.Client.Create(ctx, desiredConfigMap)
		}
		return fmt.Errorf("failed to get the serviceAccount associated with %s: %w", desiredConfigMap.Name, err)
	}

	var actualTrafficRouterPlugins []pluginItem
	if err = yaml.Unmarshal([]byte(actualConfigMap.Data[TrafficRouterPluginConfigMapKey]), &actualTrafficRouterPlugins); err != nil {
		return fmt.Errorf("failed to unmarshal traffic router plugins from ConfigMap: %s", err)
	}

	// Check if the plugin already exists and if the URL is different, update the ConfigMap
	for i, plugin := range actualTrafficRouterPlugins {
		if plugin.Name == OpenShiftRolloutPluginName {
			if plugin.Location != r.OpenShiftRoutePluginLocation {
				actualTrafficRouterPlugins[i].Location = r.OpenShiftRoutePluginLocation
				pluginBytes, err := yaml.Marshal(actualTrafficRouterPlugins)
				if err != nil {
					return fmt.Errorf("error marshalling trafficRouterPlugin to string %s", err)
				}

				actualConfigMap.Data = map[string]string{
					TrafficRouterPluginConfigMapKey: string(pluginBytes),
				}

				return r.Client.Update(ctx, actualConfigMap)
			} else {
				// Plugin URL is the same, nothing to do
				return nil
			}
		}
	}

	updatedTrafficRouterPlugins := append(actualTrafficRouterPlugins, trafficRouterPlugins...)

	pluginString, err = yaml.Marshal(updatedTrafficRouterPlugins)
	if err != nil {
		return fmt.Errorf("error marshalling trafficRouterPlugin to string %w", err)
	}

	actualConfigMap.Data[TrafficRouterPluginConfigMapKey] = string(pluginString)

	return r.Client.Update(ctx, actualConfigMap)
}
