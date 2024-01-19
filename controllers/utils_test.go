package rollouts

import (
	"context"
	"testing"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace          = "rollouts"
	testRolloutManagerName = "rollouts"
)

type rolloutManagerOpt func(*rolloutsmanagerv1alpha1.RolloutManager)

func makeTestRolloutManager(opts ...rolloutManagerOpt) *rolloutsmanagerv1alpha1.RolloutManager {
	a := &rolloutsmanagerv1alpha1.RolloutManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRolloutManagerName,
			Namespace: testNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func makeTestReconciler(t *testing.T, objs ...runtime.Object) *RolloutManagerReconciler {
	s := scheme.Scheme
	assert.NoError(t, rolloutsmanagerv1alpha1.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	return &RolloutManagerReconciler{
		Client: cl,
		Scheme: s,
	}
}

func createNamespace(r *RolloutManagerReconciler, n string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
	return r.Client.Create(context.Background(), ns)
}
