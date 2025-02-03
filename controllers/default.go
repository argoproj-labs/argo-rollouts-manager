package rollouts

const (
	// ArgoRolloutsImageEnvName is an environment variable that can be used to deploy a
	// Custom Image of rollouts controller.
	ArgoRolloutsImageEnvName = "ARGO_ROLLOUTS_IMAGE"

	// DefaultArgoRolloutsMetricsServiceName is the default name for rollouts metrics Service.
	DefaultArgoRolloutsMetricsServiceName = "argo-rollouts-metrics"

	// ArgoRolloutsDefaultImage is the default image for rollouts controller.
	DefaultArgoRolloutsImage = "quay.io/argoproj/argo-rollouts"

	// ArgoRolloutsDefaultVersion is the default version for the Rollouts controller.
	DefaultArgoRolloutsVersion = "v1.8.0" // v1.8.0

	// DefaultArgoRolloutsResourceName is the default name for Rollouts controller resources such as
	// deployment, service, role, rolebinding and serviceaccount.
	DefaultArgoRolloutsResourceName = "argo-rollouts"

	// DefaultRolloutsNotificationSecretName is the default name for rollout controller secret resource.
	DefaultRolloutsNotificationSecretName = "argo-rollouts-notification-secret" // #nosec G101

	// DefaultRolloutsServiceSelectorKey is key used by selector
	DefaultRolloutsSelectorKey = "app.kubernetes.io/name"

	// OpenShiftRolloutPluginName is the plugin name for Openshift Route Plugin
	OpenShiftRolloutPluginName = "argoproj-labs/openshift"

	// DefaultRolloutsConfigMapName is the default name of the ConfigMap that contains the Rollouts controller configuration
	DefaultRolloutsConfigMapName = "argo-rollouts-config"

	DefaultOpenShiftRoutePluginURL = "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-openshift/releases/download/commit-8d0b3c6c5c18341f9f019cf1015b56b0d0c6085b/rollouts-plugin-trafficrouter-openshift-linux-amd64"

	// NamespaceScopedArgoRolloutsController is an environment variable that can be used to configure scope of Argo Rollouts controller
	// Set true to allow only namespace-scoped Argo Rollouts controller deployment and false for cluster-scoped
	NamespaceScopedArgoRolloutsController = "NAMESPACE_SCOPED_ARGO_ROLLOUTS"

	// ClusterScopedArgoRolloutsNamespaces is an environment variable that can be used to configure namespaces that are allowed to host cluster-scoped Argo Rollouts
	ClusterScopedArgoRolloutsNamespaces = "CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES"

	KubernetesHostnameLabel = "kubernetes.io/hostname"

	TopologyKubernetesZoneLabel = "topology.kubernetes.io/zone"
)
