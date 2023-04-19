package rollouts

import (
	rolloutsApi "github.com/iam-veeramalla/argo-rollouts-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
)

// setResourceWatches will register Watches for each of the supported Resources.
func setResourceWatches(bld *builder.Builder) *builder.Builder {
	// Watch for changes to primary resource ArgoRollouts
	bld.For(&rolloutsApi.ArgoRollouts{})

	// Watch for changes to ConfigMap sub-resources owned by ArgoRollouts.
	bld.Owns(&corev1.ConfigMap{})

	// Watch for changes to Secret sub-resources owned by ArgoRollouts.
	bld.Owns(&corev1.Secret{})

	// Watch for changes to Service sub-resources owned by ArgoRollouts.
	bld.Owns(&corev1.Service{})

	// Watch for changes to Deployment sub-resources owned by ArgoRollouts.
	bld.Owns(&appsv1.Deployment{})

	// Watch for changes to Role sub-resources owned by ArgoRollouts.
	bld.Owns(&v1.Role{})

	// Watch for changes to RoleBinding sub-resources owned by ArgoRollouts.
	bld.Owns(&v1.RoleBinding{})

	return bld
}
