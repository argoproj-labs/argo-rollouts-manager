package rollouts

import (
	"context"

	"fmt"
	"reflect"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// From https://argo-rollouts.readthedocs.io/en/stable/features/traffic-management/plugins/
const TrafficRouterPluginConfigMapKey = "trafficRouterPlugins"
const MetricPluginConfigMapKey = "metricPlugins"

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

	trafficRouterPluginsMap := map[string]pluginItem{
		OpenShiftRolloutPluginName: {
			Name:     OpenShiftRolloutPluginName,
			Location: r.OpenShiftRoutePluginLocation,
		},
	}

	// Append traffic management plugins specified in RolloutManager CR
	for _, plugin := range cr.Spec.Plugins.TrafficManagement {
		// Prevent adding or modifying the OpenShiftRoutePluginName through the CR
		if plugin.Name == OpenShiftRolloutPluginName {
			return fmt.Errorf("the plugin %s cannot be modified or added through the RolloutManager CR", OpenShiftRolloutPluginName)
		}
		// Check for duplicate traffic plugins
		if _, exists := trafficRouterPluginsMap[plugin.Name]; !exists {
			trafficRouterPluginsMap[plugin.Name] = pluginItem{
				Name:     plugin.Name,
				Location: plugin.Location,
			}
		}
	}

	// Convert traffic plugins map to slice
	trafficRouterPlugins := make([]pluginItem, 0, len(trafficRouterPluginsMap))
	for _, plugin := range trafficRouterPluginsMap {
		trafficRouterPlugins = append(trafficRouterPlugins, plugin)
	}

	// Append metric plugins specified in RolloutManager CR
	metricPluginsMap := map[string]pluginItem{}
	for _, plugin := range cr.Spec.Plugins.Metric {
		// Check for duplicate metric plugins
		if _, exists := metricPluginsMap[plugin.Name]; !exists {
			metricPluginsMap[plugin.Name] = pluginItem{
				Name:     plugin.Name,
				Location: plugin.Location,
				Sha256:   plugin.SHA256,
			}
		}
	}

	// Convert metric plugins map to slice
	metricPlugins := make([]pluginItem, 0, len(metricPluginsMap))
	for _, plugin := range metricPluginsMap {
		metricPlugins = append(metricPlugins, plugin)
	}

	desiredTrafficRouterPluginString, err := yaml.Marshal(trafficRouterPlugins)
	if err != nil {
		return fmt.Errorf("error marshalling trafficRouterPlugin to string %s", err)
	}

	desiredMetricPluginString, err := yaml.Marshal(metricPlugins)
	if err != nil {
		return fmt.Errorf("error marshalling metricPlugins to string %s", err)
	}

	desiredConfigMap.Data = map[string]string{
		TrafficRouterPluginConfigMapKey: string(desiredTrafficRouterPluginString),
		MetricPluginConfigMapKey:        string(desiredMetricPluginString),
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

	// Unmarshal the existing plugin data from the actual ConfigMap
	var actualTrafficRouterPlugins, actualMetricPlugins []pluginItem
	if err = yaml.Unmarshal([]byte(actualConfigMap.Data[TrafficRouterPluginConfigMapKey]), &actualTrafficRouterPlugins); err != nil {
		return fmt.Errorf("failed to unmarshal traffic router plugins: %s", err)
	}
	if err = yaml.Unmarshal([]byte(actualConfigMap.Data[MetricPluginConfigMapKey]), &actualMetricPlugins); err != nil {
		return fmt.Errorf("failed to unmarshal metric plugins: %s", err)
	}

	// Check if an update is needed by comparing desired and actual plugin configurations
	updateNeeded := !reflect.DeepEqual(actualTrafficRouterPlugins, trafficRouterPlugins) || !reflect.DeepEqual(actualMetricPlugins, metricPlugins)

	if updateNeeded {
		// Update the ConfigMap's plugin data with the new values
		actualConfigMap.Data[TrafficRouterPluginConfigMapKey] = string(desiredTrafficRouterPluginString)
		actualConfigMap.Data[MetricPluginConfigMapKey] = string(desiredMetricPluginString)

		// Update the ConfigMap in the cluster
		if err := r.Client.Update(ctx, actualConfigMap); err != nil {
			return fmt.Errorf("failed to update ConfigMap: %v", err)
		}
		log.Info("ConfigMap updated successfully")

		// Restarting rollouts pod only if configMap is updated
		if err := r.restartRolloutsPod(ctx, cr.Namespace); err != nil {
			return err
		}
	}
	log.Info("No changes detected in ConfigMap, skipping update and pod restart")
	return nil
}

// restartRolloutsPod deletes the Rollouts Pod to trigger a restart
func (r *RolloutManagerReconciler) restartRolloutsPod(ctx context.Context, namespace string) error {
	deployment := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: DefaultArgoRolloutsResourceName, Namespace: namespace}, deployment); err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(deployment.Spec.Selector.MatchLabels),
	}
	if err := r.Client.List(ctx, podList, listOpts...); err != nil {
		return fmt.Errorf("failed to list Rollouts Pods: %w", err)
	}

	for i := range podList.Items {
		pod := podList.Items[i]
		log.Info("Deleting Rollouts Pod", "podName", pod.Name)
		if err := r.Client.Delete(ctx, &pod); err != nil {
			if errors.IsNotFound(err) {
				log.Info(fmt.Sprintf("Pod %s already deleted", pod.Name))
				continue
			}
			return fmt.Errorf("failed to delete Rollouts Pod %s: %w", pod.Name, err)
		}
		log.Info("Rollouts Pod deleted successfully", "podName", pod.Name)
	}

	return nil
}
