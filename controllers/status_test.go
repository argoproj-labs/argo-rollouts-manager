package rollouts

import (
	"context"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("RolloutManager Test", func() {
	It("RolloutManagerStatus Test", func() {
		ctx := context.Background()
		a := makeTestRolloutManager()

		r := makeTestReconciler(a)
		Expect(createNamespace(r, a.Namespace)).To(Succeed())
		Expect(r.reconcileStatus(ctx, a)).To(Succeed())

		By("When deployment for rollout controller does not exist")
		Expect(a.Status.RolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
		Expect(a.Status.Phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))

		By("When deployment exists but with an unknown status")
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: a.Namespace,
			},
		}

		Expect(r.Client.Create(ctx, deploy)).To(Succeed())
		Expect(r.reconcileStatus(ctx, a)).To(Succeed())

		Expect(a.Status.RolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseUnknown))
		Expect(a.Status.Phase).To(Equal(rolloutsmanagerv1alpha1.PhaseUnknown))

		By("When deployment exists and replicas are in pending state.")
		var requiredReplicas int32 = 1
		deploy.Status.ReadyReplicas = 0
		deploy.Spec.Replicas = &requiredReplicas

		Expect(r.Client.Update(ctx, deploy)).To(Succeed())

		Expect(r.reconcileStatus(ctx, a)).To(Succeed())

		Expect(a.Status.RolloutController).To(Equal(rolloutsmanagerv1alpha1.PhasePending))
		Expect(a.Status.Phase).To(Equal(rolloutsmanagerv1alpha1.PhasePending))

		By("When deployment exists and required number of replicas are up and running.")
		deploy.Status.ReadyReplicas = 1
		deploy.Spec.Replicas = &requiredReplicas

		Expect(r.Client.Status().Update(ctx, deploy)).To(Succeed())
		Expect(r.reconcileStatus(ctx, a)).To(Succeed())

		Expect(a.Status.RolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseAvailable))
		Expect(a.Status.Phase).To(Equal(rolloutsmanagerv1alpha1.PhaseAvailable))

	})
})
