package rollouts

import (
	"context"
	"testing"

	rolloutsApi "github.com/iam-veeramalla/argo-rollouts-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace       = "rollouts"
	testArgoRolloutName = "rollouts"
)

type argoCDOpt func(*rolloutsApi.ArgoRollout)

func makeTestArgoRollout(opts ...argoCDOpt) *rolloutsApi.ArgoRollout {
	a := &rolloutsApi.ArgoRollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoRolloutName,
			Namespace: testNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func makeTestReconciler(t *testing.T, objs ...runtime.Object) *ArgoRolloutsReconciler {
	s := scheme.Scheme
	assert.NoError(t, rolloutsApi.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	return &ArgoRolloutsReconciler{
		Client: cl,
		Scheme: s,
	}
}

func createNamespace(r *ArgoRolloutsReconciler, n string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
	return r.Client.Create(context.TODO(), ns)
}
