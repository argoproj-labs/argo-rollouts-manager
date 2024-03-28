package rollouts

import (
	"context"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *RolloutManagerReconciler) reconcileRolloutsManager(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) (metav1.Condition, error) {

	log.Info("validating RolloutManager's scope")
	if err := validateRolloutsScope(ctx, r.Client, cr, r.NamespaceScopedArgoRolloutsController); err != nil {
		if invalidRolloutScope(err) {
			return createCondition(err.Error(), rolloutsmanagerv1alpha1.RolloutManagerReasonInvalidScoped), nil
		}
		log.Error(err, "failed to validate RolloutManager's scope.")
		return createCondition(err.Error()), err
	}

	log.Info("searching for existing RolloutManagers")
	if err := checkForExistingRolloutManager(ctx, r.Client, cr); err != nil {
		if multipleRolloutManagersExist(err) {
			return createCondition(err.Error(), rolloutsmanagerv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager), nil
		}
		log.Error(err, "failed to validate multiple RolloutManagers.")
		return createCondition(err.Error()), err
	}

	log.Info("reconciling Rollouts ServiceAccount")
	sa, err := r.reconcileRolloutsServiceAccount(ctx, cr)
	if err != nil {
		log.Error(err, "failed to reconcile Rollout's ServiceAccount.")
		return createCondition(err.Error()), err
	}

	var role *rbacv1.Role
	var clusterRole *rbacv1.ClusterRole

	if cr.Spec.NamespaceScoped {
		log.Info("reconciling Rollouts Roles")
		role, err = r.reconcileRolloutsRole(ctx, cr)
		if err != nil {
			log.Error(err, "failed to reconcile Rollout's Role.")
			return createCondition(err.Error()), err
		}
	} else {
		log.Info("reconciling Rollouts ClusterRoles")
		clusterRole, err = r.reconcileRolloutsClusterRole(ctx)
		if err != nil {
			log.Error(err, "failed to reconcile Rollout's ClusterRoles.")
			return createCondition(err.Error()), err
		}
	}

	log.Info("reconciling aggregate-to-admin ClusterRole")
	if err := r.reconcileRolloutsAggregateToAdminClusterRole(ctx); err != nil {
		log.Error(err, "failed to reconcile Rollout's aggregate-to-admin ClusterRoles.")
		return createCondition(err.Error()), err
	}

	log.Info("reconciling aggregate-to-edit ClusterRole")
	if err := r.reconcileRolloutsAggregateToEditClusterRole(ctx); err != nil {
		log.Error(err, "failed to reconcile Rollout's aggregate-to-edit ClusterRoles.")
		return createCondition(err.Error()), err
	}

	log.Info("reconciling aggregate-to-view ClusterRole")
	if err := r.reconcileRolloutsAggregateToViewClusterRole(ctx); err != nil {
		log.Error(err, "failed to reconcile Rollout's aggregate-to-view ClusterRoles.")
		return createCondition(err.Error()), err
	}

	if cr.Spec.NamespaceScoped {
		log.Info("reconciling Rollouts RoleBindings")
		if err := r.reconcileRolloutsRoleBinding(ctx, cr, role, sa); err != nil {
			log.Error(err, "failed to reconcile Rollout's RoleBindings.")
			return createCondition(err.Error()), err
		}
	} else {
		log.Info("reconciling Rollouts ClusterRoleBinding")
		if err := r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, sa); err != nil {
			log.Error(err, "failed to reconcile Rollout's ClusterRoleBinding.")
			return createCondition(err.Error()), err
		}
	}

	log.Info("reconciling Rollouts Secret")
	if err := r.reconcileRolloutsSecrets(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's Secret.")
		return createCondition(err.Error()), err
	}

	log.Info("reconciling ConfigMap for plugins")
	if err := r.reconcileConfigMap(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's ConfigMap.")
		return createCondition(err.Error()), err
	}

	log.Info("reconciling Rollouts Deployment")
	if err := r.reconcileRolloutsDeployment(ctx, cr, *sa); err != nil {
		log.Error(err, "failed to reconcile Rollout's Deployment.")
		return createCondition(err.Error()), err
	}

	log.Info("reconciling Rollouts Metrics Service")
	if err := r.reconcileRolloutsMetricsService(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's Metrics Service.")
		return createCondition(err.Error()), err
	}

	log.Info("reconciling status of workloads")
	if err := r.reconcileStatus(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's Metrics Service.")
		return createCondition(err.Error()), err
	}

	return createCondition(""), nil
}
