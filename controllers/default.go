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
	DefaultArgoRolloutsVersion = "sha256:995450a0a7f7843d68e96d1a7f63422fa29b245c58f7b57dd0cf9cad72b8308f" //v1.4.1
	// DefaultArgoRolloutsResourceName is the default name for rollout controller resources such as
	// deployment, service, role, rolebinding and serviceaccount.
	DefaultArgoRolloutsResourceName = "argo-rollouts"
	// DefaultRolloutsNotificationSecretName is the default name for rollout controller secret resource.
	DefaultRolloutsNotificationSecretName = "argo-rollouts-notification-secret"
	// DefaultRolloutsServiceSelectorKey is key used by selector
	DefaultRolloutsSelectorKey = "app.kubernetes.io/name"
)
