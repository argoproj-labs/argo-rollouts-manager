package rollouts

import (
	"context"

	rolloutsApi "github.com/iam-veeramalla/argo-rollouts-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// reconcileStatus will ensure that all of the Status properties are updated for the given ArgoCD.
func (r *ArgoRolloutsReconciler) reconcileStatus(cr *rolloutsApi.ArgoRollout) error {

	if err := r.reconcileRolloutControllerStatus(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusPhase(cr); err != nil {
		return err
	}

	return nil
}

func (r *ArgoRolloutsReconciler) reconcileRolloutControllerStatus(cr *rolloutsApi.ArgoRollout) error {
	status := "Unknown"

	deploy := &appsv1.Deployment{}
	if err := fetchObject(r.Client, cr.Namespace, DefaultArgoRolloutsResourceName, deploy); err != nil {
		if errors.IsNotFound(err) {
			status = "Failure"
		}
		log.Error(err, "error getting deployment")
	}

	if deploy.Spec.Replicas != nil {
		status = "Pending"
		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
			status = "Available"
		}
	}

	if cr.Status.RolloutController != status {
		cr.Status.RolloutController = status
		if err := r.Client.Status().Update(context.TODO(), cr); err != nil {
			log.Error(err, "error updating the Argo Rollout CR status")
		}
	}

	return nil
}

// Reconciles the status phase of the ArgoRollout
func (r *ArgoRolloutsReconciler) reconcileStatusPhase(cr *rolloutsApi.ArgoRollout) error {

	// For now, there is only one controller that is created by ArgoRollout CR
	// So the status of Argo ROllout will be same as the status of the Rollout Controller
	// In future this condition may change
	if cr.Status.Phase != cr.Status.RolloutController {
		cr.Status.Phase = cr.Status.RolloutController
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
