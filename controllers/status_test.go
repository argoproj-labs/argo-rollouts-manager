package rollouts

import (
	"context"
	"testing"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRolloutManager_Status(t *testing.T) {

	ctx := context.Background()

	a := makeTestRolloutManager()

	r := makeTestReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace))

	err := r.reconcileStatus(ctx, a)
	assert.NoError(t, err)

	// When deployment for rollout controller does not exist
	assert.Equal(t, a.Status.RolloutController, rolloutsmanagerv1alpha1.PhaseFailure)
	assert.Equal(t, a.Status.Phase, rolloutsmanagerv1alpha1.PhaseFailure)

	// When deployment exists but with an unknown status
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: a.Namespace,
		},
	}
	assert.NoError(t, r.Client.Create(ctx, deploy))

	assert.NoError(t, r.reconcileStatus(ctx, a))
	assert.Equal(t, a.Status.RolloutController, rolloutsmanagerv1alpha1.PhaseUnknown)
	assert.Equal(t, a.Status.Phase, rolloutsmanagerv1alpha1.PhaseUnknown)

	// When deployment exists and replicas are in pending state.
	var requiredReplicas int32 = 1
	deploy.Status.ReadyReplicas = 0
	deploy.Spec.Replicas = &requiredReplicas

	assert.NoError(t, r.Client.Update(ctx, deploy))

	assert.NoError(t, r.reconcileStatus(ctx, a))
	assert.Equal(t, a.Status.RolloutController, rolloutsmanagerv1alpha1.PhasePending)
	assert.Equal(t, a.Status.Phase, rolloutsmanagerv1alpha1.PhasePending)

	// When deployment exists and required number of replicas are up and running.
	deploy.Status.ReadyReplicas = 1
	deploy.Spec.Replicas = &requiredReplicas

	assert.NoError(t, r.Client.Update(ctx, deploy))

	assert.NoError(t, r.reconcileStatus(ctx, a))
	assert.Equal(t, a.Status.RolloutController, rolloutsmanagerv1alpha1.PhaseAvailable)
	assert.Equal(t, a.Status.Phase, rolloutsmanagerv1alpha1.PhaseAvailable)
}
