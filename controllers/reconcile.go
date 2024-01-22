package rollouts

import (
	"context"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
)

func (r *RolloutManagerReconciler) reconcileRolloutsManager(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	log.Info("reconciling rollouts serviceaccount")
	sa, err := r.reconcileRolloutsServiceAccount(ctx, cr)
	if err != nil {
		return err
	}

	log.Info("reconciling rollouts roles")
	role, err := r.reconcileRolloutsRole(ctx, cr)
	if err != nil {
		return err
	}

	log.Info("reconciling aggregate-to-admin ClusterRole")
	if err := r.reconcileRolloutsAggregateToAdminClusterRole(ctx, cr); err != nil {
		return err
	}

	log.Info("reconciling aggregate-to-edit ClusterRole")
	if err := r.reconcileRolloutsAggregateToEditClusterRole(ctx, cr); err != nil {
		return err
	}

	log.Info("reconciling aggregate-to-view ClusterRole")
	if err := r.reconcileRolloutsAggregateToViewClusterRole(ctx, cr); err != nil {
		return err
	}

	log.Info("reconciling rollouts role bindings")
	if err := r.reconcileRolloutsRoleBinding(ctx, cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling rollouts secret")
	if err := r.reconcileRolloutsSecrets(ctx, cr); err != nil {
		return err
	}

	// TODO: #22 - Re-enable this once ConfigMap reconciliation is fixed:

	// // reconcile configMap for plugins
	// log.Info("reconciling configMap for plugins")
	// if err := r.reconcileConfigMap(ctx, cr); err != nil {
	// 	return err
	// }

	log.Info("reconciling rollouts deployment")
	if err := r.reconcileRolloutsDeployment(ctx, cr, sa); err != nil {
		return err
	}

	log.Info("reconciling rollouts metrics service")
	if err := r.reconcileRolloutsMetricsService(ctx, cr); err != nil {
		return err
	}

	log.Info("reconciling status of workloads")
	if err := r.reconcileStatus(ctx, cr); err != nil {
		return err
	}

	return nil
}
