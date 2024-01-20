package rolloutmanager

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
)

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func expectedCondition(f func(app rolloutsmanagerv1alpha1.RolloutManager) bool) matcher.GomegaMatcher {

	return WithTransform(func(app rolloutsmanagerv1alpha1.RolloutManager) bool {

		k8sClient, err := fixture.GetE2ETestKubeClient()
		if err != nil {
			fmt.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(&app), &app)
		if err != nil {
			fmt.Println(err)
			return false
		}

		return f(app)

	}, BeTrue())

}

func HavePhase(phase rolloutsmanagerv1alpha1.RolloutControllerPhase) matcher.GomegaMatcher {
	return expectedCondition(func(app rolloutsmanagerv1alpha1.RolloutManager) bool {
		return app.Status.Phase == phase
	})
}
