package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	// . "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TestE2ENamespace = "argo-rollouts"
)

func EnsureCleanSlate() error {
	return EnsureDestinationNamespaceExists(context.Background(), TestE2ENamespace)
}

func EnsureDestinationNamespaceExists(ctx context.Context, namespaceParam string) error {

	config, err := GetSystemKubeConfig()
	if err != nil {
		return fmt.Errorf("unable to retrieve valid Kubernetes context: %w", err)
	}

	k8sClient, err := GetKubeClient(config)
	if err != nil {
		return err
	}

	if err := DeleteNamespace(ctx, namespaceParam, k8sClient); err != nil {
		return fmt.Errorf("unable to delete namespace '%s': %v", namespaceParam, err)
	}

	namespaceToCreate := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: namespaceParam,
	}}

	if err := k8sClient.Create(context.Background(), &namespaceToCreate); err != nil {
		return fmt.Errorf("unable to create namespace '%s': %v", namespaceParam, err)
	}

	return nil
}

// DeleteNamespace deletes a namespace, and waits for it to be reported as deleted.
func DeleteNamespace(ctx context.Context, namespaceParam string, k8sClient client.Client) error {

	// Delete the namespace:
	// - Issue a request to Delete the namespace
	// - Finally, we check if it has been deleted.
	if err := wait.PollImmediate(time.Second*5, time.Minute*6, func() (done bool, err error) {

		// Delete the namespace, if it exists
		namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceParam,
			},
		}
		if err := k8sClient.Delete(ctx, &namespace); err != nil {
			if !apierr.IsNotFound(err) {
				GinkgoWriter.Printf("unable to delete namespace '%s': %v\n", namespaceParam, err)
				return false, nil
			}
		}

		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&namespace), &namespace); err != nil {
			if apierr.IsNotFound(err) {
				return true, nil
			} else {
				GinkgoWriter.Printf("unable to Get namespace '%s': %v\n", namespaceParam, err)
				return false, nil
			}
		}

		return false, nil
	}); err != nil {
		return fmt.Errorf("namespace was never deleted, after delete was issued. '%s':%v", namespaceParam, err)
	}

	return nil
}

func GetE2ETestKubeClient() (client.Client, error) {
	config, err := GetSystemKubeConfig()
	if err != nil {
		return nil, err
	}

	k8sClient, err := GetKubeClient(config)
	if err != nil {
		return nil, err
	}

	return k8sClient, nil
}

// GetKubeClient returns a controller-runtime Client for accessing K8s API resources used by the controller.
func GetKubeClient(config *rest.Config) (client.Client, error) {

	scheme := runtime.NewScheme()

	if err := rolloutsmanagerv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := apps.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := admissionv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return k8sClient, nil

}

// Retrieve the system-level Kubernetes config (e.g. ~/.kube/config or service account config from volume)
func GetSystemKubeConfig() (*rest.Config, error) {

	overrides := clientcmd.ConfigOverrides{}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return restConfig, nil
}
