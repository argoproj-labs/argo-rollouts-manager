package rollouts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRolloutManager_Status(t *testing.T) {
	a := makeTestRolloutManager()

	r := makeTestReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace))

	err := r.reconcileStatus(a)
	assert.NoError(t, err)

	// When deployment for rollout controller does not exist
	assert.Equal(t, a.Status.RolloutController, "Failure")
	assert.Equal(t, a.Status.Phase, "Failure")

	// When deployment exists but with an unknown status
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: a.Namespace,
		},
	}
	assert.NoError(t, r.Client.Create(context.TODO(), deploy))

	assert.NoError(t, r.reconcileStatus(a))
	assert.Equal(t, a.Status.RolloutController, "Unknown")
	assert.Equal(t, a.Status.Phase, "Unknown")

	// When deployment exists and replicas are in pending state.
	var requiredReplicas int32 = 1
	deploy.Status.ReadyReplicas = 0
	deploy.Spec.Replicas = &requiredReplicas

	assert.NoError(t, r.Client.Update(context.TODO(), deploy))

	assert.NoError(t, r.reconcileStatus(a))
	assert.Equal(t, a.Status.RolloutController, "Pending")
	assert.Equal(t, a.Status.Phase, "Pending")

	// When deployment exists and required number of replicas are up and running.
	deploy.Status.ReadyReplicas = 1
	deploy.Spec.Replicas = &requiredReplicas

	assert.NoError(t, r.Client.Update(context.TODO(), deploy))

	assert.NoError(t, r.reconcileStatus(a))
	assert.Equal(t, a.Status.RolloutController, "Available")
	assert.Equal(t, a.Status.Phase, "Available")
}
