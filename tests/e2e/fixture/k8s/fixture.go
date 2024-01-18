package k8s

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ExistByName checks if the given resource exists, when retrieving it by name/namespace.
// Does NOT check if the resource content matches.
func ExistByName(k8sClient client.Client) matcher.GomegaMatcher {

	return WithTransform(func(k8sObject client.Object) bool {

		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(k8sObject), k8sObject)
		if err != nil {
			fmt.Println("Object does not exists in ExistByName:", k8sObject.GetName(), err)
		} else {
			fmt.Println("Object exists in ExistByName:", k8sObject.GetName())
		}
		return err == nil
	}, BeTrue())
}
