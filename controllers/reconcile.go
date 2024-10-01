package rollouts

import (
	"context"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reconcileStatusResult is returned by 'reconcileRolloutsManager', and related functions, to control what values to set on the .status field of RolloutManager, after reconciliation. Values set in reconcileStatusResult will be set on RolloutManager's .status field.
type reconcileStatusResult struct {

	// condition to be set on RolloutManager's .status.condition
	condition metav1.Condition

	// rolloutController: if non-nil, .status.rolloutController will be set to this value, after call to reconcileRolloutsManager
	rolloutController *rolloutsmanagerv1alpha1.RolloutControllerPhase

	// phase: if non-nil, .status.phase will be set to this value, after call to reconcileRolloutsManager
	phase *rolloutsmanagerv1alpha1.RolloutControllerPhase
}

func (r *RolloutManagerReconciler) reconcileRolloutsManager(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) (reconcileStatusResult, error) {

	log.Info("validating RolloutManager's scope")
	if rr, err := validateRolloutsScope(cr, r.NamespaceScopedArgoRolloutsController); err != nil {
		if invalidRolloutScope(err) {
			rr.condition = createCondition(err.Error(), rolloutsmanagerv1alpha1.RolloutManagerReasonInvalidScoped)
			return *rr, nil
		}

		if invalidRolloutNamespace(err) {
			rr.condition = createCondition(err.Error(), rolloutsmanagerv1alpha1.RolloutManagerReasonInvalidNamespace)
			return *rr, nil
		}

		log.Error(err, "failed to validate RolloutManager's scope.")
		return wrapCondition(createCondition(err.Error())), err
	}

	log.Info("searching for existing RolloutManagers")
	if res, err := checkForExistingRolloutManager(ctx, r.Client, cr); err != nil {
		if multipleRolloutManagersExist(err) {

			res.condition = createCondition(err.Error(), rolloutsmanagerv1alpha1.RolloutManagerReasonMultipleClusterScopedRolloutManager)

			return *res, nil
		}
		log.Error(err, "failed to validate multiple RolloutManagers.")
		return wrapCondition(createCondition(err.Error())), err
	}

	log.Info("reconciling Rollouts ServiceAccount")
	sa, err := r.reconcileRolloutsServiceAccount(ctx, cr)
	if err != nil {
		log.Error(err, "failed to reconcile Rollout's ServiceAccount.")
		return wrapCondition(createCondition(err.Error())), err
	}

	var role *rbacv1.Role
	var clusterRole *rbacv1.ClusterRole

	if cr.Spec.NamespaceScoped {
		log.Info("reconciling Rollouts Roles")
		role, err = r.reconcileRolloutsRole(ctx, cr)
		if err != nil {
			log.Error(err, "failed to reconcile Rollout's Role.")
			return wrapCondition(createCondition(err.Error())), err
		}
	} else {
		log.Info("reconciling Rollouts ClusterRoles")
		clusterRole, err = r.reconcileRolloutsClusterRole(ctx, cr)
		if err != nil {
			log.Error(err, "failed to reconcile Rollout's ClusterRoles.")
			return wrapCondition(createCondition(err.Error())), err
		}
	}

	log.Info("reconciling aggregate-to-admin ClusterRole")
	if err := r.reconcileRolloutsAggregateToAdminClusterRole(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's aggregate-to-admin ClusterRoles.")
		return wrapCondition(createCondition(err.Error())), err
	}

	log.Info("reconciling aggregate-to-edit ClusterRole")
	if err := r.reconcileRolloutsAggregateToEditClusterRole(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's aggregate-to-edit ClusterRoles.")
		return wrapCondition(createCondition(err.Error())), err
	}

	log.Info("reconciling aggregate-to-view ClusterRole")
	if err := r.reconcileRolloutsAggregateToViewClusterRole(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's aggregate-to-view ClusterRoles.")
		return wrapCondition(createCondition(err.Error())), err
	}

	if cr.Spec.NamespaceScoped {
		log.Info("reconciling Rollouts RoleBindings")
		if err := r.reconcileRolloutsRoleBinding(ctx, cr, role, sa); err != nil {
			log.Error(err, "failed to reconcile Rollout's RoleBindings.")
			return wrapCondition(createCondition(err.Error())), err
		}
	} else {
		log.Info("reconciling Rollouts ClusterRoleBinding")
		if err := r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, sa, cr); err != nil {
			log.Error(err, "failed to reconcile Rollout's ClusterRoleBinding.")
			return wrapCondition(createCondition(err.Error())), err
		}
	}

	log.Info("reconciling Rollouts Secret")
	if err := r.reconcileRolloutsSecrets(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's Secret.")
		return wrapCondition(createCondition(err.Error())), err
	}

	log.Info("reconciling ConfigMap for plugins")
	if err := r.reconcileConfigMap(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's ConfigMap.")
		return wrapCondition(createCondition(err.Error())), err
	}

	log.Info("reconciling Rollouts Deployment")
	if err := r.reconcileRolloutsDeployment(ctx, cr, *sa); err != nil {
		log.Error(err, "failed to reconcile Rollout's Deployment.")
		return wrapCondition(createCondition(err.Error())), err
	}

	log.Info("reconciling Rollouts Metrics Service")
	if err := r.reconcileRolloutsMetricsServiceAndMonitor(ctx, cr); err != nil {
		log.Error(err, "failed to reconcile Rollout's Metrics Service.")
		return wrapCondition(createCondition(err.Error())), err
	}

	log.Info("reconciling status of workloads")
	rr, err := r.determineStatusPhase(ctx, cr)
	if err != nil {
		log.Error(err, "failed to reconcile status of workloads.")
		return wrapCondition(createCondition(err.Error())), err
	}

	rr.condition = createCondition("") // success

	return rr, nil
}
