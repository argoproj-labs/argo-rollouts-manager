package rollouts

import (
	"context"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// determineStatusPhase calculates and returns RolloutManager's current .status.phase and .status.rolloutcontroller, both based on Deployment status.
func (r *RolloutManagerReconciler) determineStatusPhase(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager) (reconcileStatusResult, error) {

	status := rolloutsmanagerv1alpha1.PhaseUnknown

	deploy := &appsv1.Deployment{}
	if err := fetchObject(ctx, r.Client, cr.Namespace, DefaultArgoRolloutsResourceName, deploy); err != nil {
		if apierrors.IsNotFound(err) {
			status = rolloutsmanagerv1alpha1.PhaseFailure
		} else {
			log.Error(err, "error retrieving Deployment")
			return reconcileStatusResult{}, err
		}
	} else {

		// Deployment exists

		if deploy.Spec.Replicas != nil {
			status = rolloutsmanagerv1alpha1.PhasePending
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = rolloutsmanagerv1alpha1.PhaseAvailable
			}
		}
	}

	var res reconcileStatusResult

	if cr.Status.RolloutController != status {
		res.rolloutController = &status
	}

	if cr.Status.Phase != status {
		res.phase = &status
	}

	return res, nil
}
