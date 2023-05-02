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

	log.Info("reconciling rollouts service")
	if err := r.reconcileRolloutsService(cr); err != nil {
		return err
	}

	log.Info("reconciling status of workloads")
	if err := r.reconcileStatus(cr); err != nil {
		return err
	}

	return nil
}
