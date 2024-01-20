package rollouts

const (
	// ArgoRolloutsImageEnvName is an environment variable that can be used to deploy a
	// Custom Image of rollouts controller.
	ArgoRolloutsImageEnvName = "ARGO_ROLLOUTS_IMAGE"
	// DefaultArgoRolloutsMetricsServiceName is the default name for rollouts metrics service.
	DefaultArgoRolloutsMetricsServiceName = "argo-rollouts-metrics"
	// ArgoRolloutsDefaultImage is the default image for rollouts controller.
	DefaultArgoRolloutsImage = "quay.io/argoproj/argo-rollouts"
	// ArgoRolloutsDefaultVersion is the default version for the rollouts controller.
	DefaultArgoRolloutsVersion = "quay.io/argoproj/argo-rollouts:v1.6.4" // v1.6.4
	// DefaultArgoRolloutsResourceName is the default name for rollout controller resources such as
	// deployment, service, role, rolebinding and serviceaccount.
	DefaultArgoRolloutsResourceName = "argo-rollouts"
	// DefaultRolloutsNotificationSecretName is the default name for rollout controller secret resource.
	DefaultRolloutsNotificationSecretName = "argo-rollouts-notification-secret" // #nosec G101
	// DefaultRolloutsServiceSelectorKey is key used by selector
	DefaultRolloutsSelectorKey = "app.kubernetes.io/name"

	// OpenShiftRolloutPluginName is the plugin name for Openshift Route Plugin
	OpenShiftRolloutPluginName = "openshift-route-plugin"
	// OpenShiftRolloutPluginPath is the path on the rollout controller pod where the plugin will be mounted
	OpenShiftRolloutPluginPath = "/plugin/rollouts-plugin-trafficrouter-openshift"
	// DefaultRolloutsConfigMapName is the default name of the ConfigMap that contains the Rollouts controller configuration
	DefaultRolloutsConfigMapName = "argo-rollouts-config"
)

