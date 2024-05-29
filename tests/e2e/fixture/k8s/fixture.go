package k8s

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"
	"k8s.io/client-go/util/retry"

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

// UpdateWithoutConflict will keep trying to update object until it succeeds, or times out.
func UpdateWithoutConflict(ctx context.Context, obj client.Object, k8sClient client.Client, modify func(client.Object)) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of the object
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return err
		}
		modify(obj)

		// Attempt to update the object
		return k8sClient.Update(ctx, obj)
	})

	return err
}

func HaveLabel(keyParam, valueParam string, k8sClient client.Client) matcher.GomegaMatcher {

	return WithTransform(func(k8sObject client.Object) bool {

		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(k8sObject), k8sObject)
		Expect(err).ToNot(HaveOccurred())

		for key, value := range k8sObject.GetLabels() {
			if key == keyParam && value == valueParam {
				return true
			}
		}

		return false

	}, BeTrue())
}

func HaveAnnotation(keyParam, valueParam string, k8sClient client.Client) matcher.GomegaMatcher {

	return WithTransform(func(k8sObject client.Object) bool {

		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(k8sObject), k8sObject)
		Expect(err).ToNot(HaveOccurred())

		for key, value := range k8sObject.GetAnnotations() {
			if key == keyParam && value == valueParam {
				return true
			}
		}

		return false

	}, BeTrue())
}
