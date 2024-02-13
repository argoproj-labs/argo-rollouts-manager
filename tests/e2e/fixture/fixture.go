package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
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
	TestE2ENamespace     = "argo-rollouts"
	NameSpaceLabelsKey   = "app"
	NameSpaceLabelsValue = "rolloutsmanager-e2e-test"
)

type Cleaner struct {
	cxt       context.Context
	k8sClient client.Client
}

func NewCleaner() (*Cleaner, error) {
	k8sClient, _, err := GetE2ETestKubeClient()
	if err != nil {
		return nil, err
	}

	return &Cleaner{
		cxt:       context.Background(),
		k8sClient: k8sClient,
	}, nil
}

func EnsureCleanSlate() error {
	cleaner, err := NewCleaner()
	if err != nil {
		return err
	}

	err = cleaner.EnsureTestNamespaceDeleted()
	if err != nil {
		return err
	}

	err = cleaner.EnsureDestinationNamespaceExists(TestE2ENamespace)
	if err != nil {
		return err
	}

	err = cleaner.DeleteRolloutsClusterRoles()
	if err != nil {
		return err
	}

	return nil
}

func (cleaner *Cleaner) EnsureDestinationNamespaceExists(namespaceParam string) error {
	if err := cleaner.DeleteNamespace(namespaceParam); err != nil {
		return fmt.Errorf("unable to delete namespace '%s': %w", namespaceParam, err)
	}

	namespaceToCreate := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: namespaceParam,
	}}

	if err := cleaner.k8sClient.Create(cleaner.cxt, &namespaceToCreate); err != nil {
		return fmt.Errorf("unable to create namespace '%s': %w", namespaceParam, err)
	}

	return nil
}

func (cleaner *Cleaner) DeleteRolloutsClusterRoles() error {
	crList := rbacv1.ClusterRoleList{}
	if err := cleaner.k8sClient.List(cleaner.cxt, &crList, &client.ListOptions{}); err != nil {
		return err
	}
	for idx := range crList.Items {
		sa := crList.Items[idx]
		// Skip any CRs that DON'T contain argo-rollouts
		if !strings.Contains(sa.Name, "argo-rollouts") {
			continue
		}
		if err := cleaner.k8sClient.Delete(cleaner.cxt, &sa); err != nil {
			return err
		}
	}

	return nil
}

// DeleteNamespace deletes a namespace, and waits for it to be reported as deleted.
func (cleaner *Cleaner) DeleteNamespace(namespaceParam string) error {

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
		if err := cleaner.k8sClient.Delete(cleaner.cxt, &namespace); err != nil {
			if !apierr.IsNotFound(err) {
				GinkgoWriter.Printf("unable to delete namespace '%s': %v\n", namespaceParam, err)
				return false, nil
			}
		}

		if err := cleaner.k8sClient.Get(cleaner.cxt, client.ObjectKeyFromObject(&namespace), &namespace); err != nil {
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

func GetE2ETestKubeClient() (client.Client, *runtime.Scheme, error) {
	config, err := GetSystemKubeConfig()
	if err != nil {
		return nil, nil, err
	}

	k8sClient, scheme, err := GetKubeClient(config)
	if err != nil {
		return nil, nil, err
	}

	return k8sClient, scheme, nil
}

// GetKubeClient returns a controller-runtime Client for accessing K8s API resources used by the controller.
func GetKubeClient(config *rest.Config) (client.Client, *runtime.Scheme, error) {

	scheme := runtime.NewScheme()

	if err := rolloutsmanagerv1alpha1.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}

	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}

	if err := apps.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}

	if err := admissionv1.AddToScheme(scheme); err != nil {
		return nil, nil, err
	}

	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, nil, err
	}

	return k8sClient, scheme, nil

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

func (cleaner *Cleaner) EnsureTestNamespaceDeleted() error {
	nsList, err := ListNameSpaces(cleaner.cxt, cleaner.k8sClient)
	if err != nil {
		return fmt.Errorf("unable to delete test namespace: %w", err)
	}

	for _, namespace := range nsList.Items {
		if err := cleaner.DeleteNamespace(namespace.Name); err != nil {
			return fmt.Errorf("unable to delete namespace '%s': %w", namespace.Name, err)
		}
	}
	return nil
}

func ListNameSpaces(ctx context.Context, k8sClient client.Client) (corev1.NamespaceList, error) {
	nsList := corev1.NamespaceList{}
	req, err := labels.NewRequirement(NameSpaceLabelsKey, selection.Equals, []string{NameSpaceLabelsValue})
	if err != nil {
		return nsList, fmt.Errorf("unable to set labels while fetching list of test namespace: %w", err)
	}

	err = k8sClient.List(ctx, &nsList, &client.ListOptions{LabelSelector: labels.NewSelector().Add(*req)})
	if err != nil {
		return nsList, fmt.Errorf("unable to fetch list of test namespace: %w", err)
	}
	return nsList, nil
}
