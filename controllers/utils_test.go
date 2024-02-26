package rollouts

import (
	"context"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/go-logr/logr"
	logger "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("checkForExistingRolloutManager tests", func() {

	var (
		ctx             context.Context
		log             logr.Logger
		k8sClient       client.WithWatch
		rolloutsManager rolloutsmanagerv1alpha1.RolloutManager
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(rolloutsmanagerv1alpha1.AddToScheme(s)).To(Succeed())

		ctx = context.Background()
		log = logger.FromContext(ctx)
		k8sClient = fake.NewClientBuilder().WithScheme(s).Build()

		rolloutsManager = rolloutsmanagerv1alpha1.RolloutManager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rm-1",
				Namespace: "test-ns-1",
			},
			Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
				NamespaceScoped: false,
			},
		}
	})

	When("A single cluster scoped rolloutsManager is created.", func() {
		It("should not return any error, as it is a valid use case.", func() {

			By("Create only one RolloutManager.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify there is no error returned.")
			Expect(checkForExistingRolloutManager(ctx, k8sClient, &rolloutsManager, log)).To(Succeed())
		})
	})

	When("Multiple rolloutsManager are created and all are namespace scoped RMs.", func() {
		It("should not return error for any of them, because only one cluster scoped RM or all namespace scoped RMs are allowed.", func() {

			By("1st RM: Create namespace scoped RM.")
			rolloutsManager.Spec.NamespaceScoped = true
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("1st RM: Verify there is no error returned, as only one RM is created yet.")
			Expect(checkForExistingRolloutManager(ctx, k8sClient, &rolloutsManager, log)).To(Succeed())

			By("2nd RM: Create namespace scoped RM.")
			rolloutsManager2 := rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rm-2",
					Namespace: "test-ns-2",
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutsManager2)).To(Succeed())

			By("2nd RM: Verify there is no error returned, as all namespace scoped RMs are created.")
			Expect(checkForExistingRolloutManager(ctx, k8sClient, &rolloutsManager2, log)).To(Succeed())

			By("1st RM: Recheck and it should still work, as all namespace scoped RMs are created.")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rolloutsManager.Name, Namespace: rolloutsManager.Namespace}, &rolloutsManager)).To(Succeed())
			Expect(checkForExistingRolloutManager(ctx, k8sClient, &rolloutsManager, log)).To(Succeed())
		})
	})

	When("Multiple rolloutsManager are created, including cluster scoped RMs.", func() {
		It("should return error for all of them, because only one cluster scoped RM or all namespace scoped RMs are allowed.", func() {

			By("1st RM: Create cluster scoped RM.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("1st RM: Verify there is no error returned, as only one RM is created yet.")
			Expect(checkForExistingRolloutManager(ctx, k8sClient, &rolloutsManager, log)).To(Succeed())

			By("2nd RM: Create namespace scoped RM.")
			rolloutsManager2 := rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rm-2",
					Namespace: "test-ns-2",
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
					NamespaceScoped: true,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutsManager2)).To(Succeed())

			By("2nd RM: It should return error.")
			err := checkForExistingRolloutManager(ctx, k8sClient, &rolloutsManager2, log)
			Expect(err).To(HaveOccurred())
			Expect(multipleRolloutManagersExist(err)).To(BeTrue())
			Expect(rolloutsManager2.Status.Phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(rolloutsManager2.Status.RolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))

			By("1st RM: Recheck 1st RM and it should also have error now. since multiple RMs are created.")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rolloutsManager2.Name, Namespace: rolloutsManager2.Namespace}, &rolloutsManager2)).To(Succeed())

			err = checkForExistingRolloutManager(ctx, k8sClient, &rolloutsManager, log)
			Expect(err).To(HaveOccurred())
			Expect(multipleRolloutManagersExist(err)).To(BeTrue())
			Expect(rolloutsManager.Status.Phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(rolloutsManager.Status.RolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
		})
	})
})

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

func makeTestReconciler(objs ...runtime.Object) *RolloutManagerReconciler {
	s := scheme.Scheme

	err := rolloutsmanagerv1alpha1.AddToScheme(s)
	Expect(err).ToNot(HaveOccurred())

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	return &RolloutManagerReconciler{
		Client:                       cl,
		Scheme:                       s,
		OpenShiftRoutePluginLocation: "file://non-empty-test-url", // Set a non-real, non-empty value for unit tests: override this to test a specific value
	}
}

func createNamespace(r *RolloutManagerReconciler, n string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
	return r.Client.Create(context.Background(), ns)
}
