package rollouts

import (
	rolloutsApi "github.com/iam-veeramalla/argo-rollouts-manager/api/v1alpha1"
)

func (r *RolloutManagerReconciler) reconcileRolloutsController(cr *rolloutsApi.RolloutManager) error {

	log.Info("reconciling rollouts serviceaccount")
	sa, err := r.reconcileRolloutsServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling rollouts roles")
	role, err := r.reconcileRolloutsRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling aggregate-to-admin ClusterRole")
	if err := r.reconcileRolloutsAggregateToAdminClusterRole(cr); err != nil {
		return err
	}

	log.Info("reconciling aggregate-to-edit ClusterRole")
	if err := r.reconcileRolloutsAggregateToEditClusterRole(cr); err != nil {
		return err
	}

	log.Info("reconciling aggregate-to-view ClusterRole")
	if err := r.reconcileRolloutsAggregateToViewClusterRole(cr); err != nil {
		return err
	}

	log.Info("reconciling rollouts role bindings")
	if err := r.reconcileRolloutsRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling rollouts secret")
	if err := r.reconcileRolloutsSecrets(cr); err != nil {
		return err
	}

	log.Info("reconciling rollouts deployment")
	if err := r.reconcileRolloutsDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("reconciling rollouts metrics service")
	if err := r.reconcileRolloutsMetricsService(cr); err != nil {
		return err
	}

	log.Info("reconciling status of workloads")
	if err := r.reconcileStatus(cr); err != nil {
		return err
	}

	return nil
}
