package rolloutmanager

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argo-rollouts-manager/tests/e2e/fixture"
)

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchRolloutManager(f func(app rolloutsmanagerv1alpha1.RolloutManager) bool) matcher.GomegaMatcher {

	return WithTransform(func(app rolloutsmanagerv1alpha1.RolloutManager) bool {

		k8sClient, _, err := fixture.GetE2ETestKubeClient()
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
	return fetchRolloutManager(func(app rolloutsmanagerv1alpha1.RolloutManager) bool {
		fmt.Println("HavePhase:", "expected: ", phase, "actual: ", app.Status.Phase)
		return app.Status.Phase == phase
	})
}

func HaveConditions(expected metav1.Condition) matcher.GomegaMatcher {
	return fetchRolloutManager(func(app rolloutsmanagerv1alpha1.RolloutManager) bool {

		if len(app.Status.Conditions) == 0 {
			fmt.Println("HaveConditions: Conditions is nil")
			return false
		}

		for _, condition := range app.Status.Conditions {
			if condition.Type == expected.Type {
				fmt.Println("HaveConditions:", "expected: ", expected, "actual: ", condition)
				return condition.Type == expected.Type &&
					condition.Status == expected.Status &&
					condition.Reason == expected.Reason &&
					condition.Message == expected.Message
			}
		}
		return false
	})
}
