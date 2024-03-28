package rollouts

import (
	"context"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// reconcileStatus will ensure that all of the Status properties are updated for the given RolloutManager.
func (r *RolloutManagerReconciler) reconcileStatus(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	if err := r.reconcileRolloutControllerStatus(ctx, cr); err != nil {
		return err
	}

	if err := r.reconcileStatusPhase(ctx, cr); err != nil {
		return err
	}

	return nil
}

func (r *RolloutManagerReconciler) reconcileRolloutControllerStatus(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {
	status := rolloutsmanagerv1alpha1.PhaseUnknown

	deploy := &appsv1.Deployment{}
	if err := fetchObject(ctx, r.Client, cr.Namespace, DefaultArgoRolloutsResourceName, deploy); err != nil {
		if apierrors.IsNotFound(err) {
			status = rolloutsmanagerv1alpha1.PhaseFailure
		}
		log.Error(err, "error getting deployment")
	}

	if deploy.Spec.Replicas != nil {
		status = rolloutsmanagerv1alpha1.PhasePending
		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
			status = rolloutsmanagerv1alpha1.PhaseAvailable
		}
	}

	if cr.Status.RolloutController != status {
		if err := UpdateDeploymentStatusWithFunction(ctx, r.Client, cr, func(cr *rolloutsmanagerv1alpha1.RolloutManager) {
			cr.Status.RolloutController = status
		}); err != nil {
			log.Error(err, "error updating the Argo Rollout CR status")
		}
	}

	return nil
}

// Reconciles the status phase of the RolloutManager
func (r *RolloutManagerReconciler) reconcileStatusPhase(ctx context.Context, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	// For now, there is only one controller that is created by RolloutManager CR
	// So the status of Argo Rollout will be same as the status of the Rollout Controller
	// In future this condition may change
	if cr.Status.Phase != cr.Status.RolloutController {
		cr.Status.Phase = cr.Status.RolloutController

		return UpdateDeploymentStatusWithFunction(ctx, r.Client, cr, func(cr *rolloutsmanagerv1alpha1.RolloutManager) {
			cr.Status.Phase = cr.Status.RolloutController
		})
	}
	return nil
}
