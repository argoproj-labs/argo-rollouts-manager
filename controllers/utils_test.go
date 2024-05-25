package rollouts

import (
	"context"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	logger "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("updateStatusConditionOfRolloutManager tests", func() {

	var (
		ctx             context.Context
		k8sClient       client.WithWatch
		rolloutsManager rolloutsmanagerv1alpha1.RolloutManager
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(rolloutsmanagerv1alpha1.AddToScheme(s)).To(Succeed())

		ctx = context.Background()
		log = logger.FromContext(ctx)
		k8sClient = fake.NewClientBuilder().WithStatusSubresource(&rolloutsmanagerv1alpha1.RolloutManager{}).WithScheme(s).Build()

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

	When("reconcileStatusResult has a non-nil phase", func() {
		It("should set the phase on the RolloutManager status", func() {

			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			failed := rolloutsmanagerv1alpha1.PhaseFailure
			rsr := reconcileStatusResult{
				phase: &failed,
			}
			Expect(updateStatusConditionOfRolloutManager(ctx, rsr, &rolloutsManager, k8sClient, logger.FromContext(ctx))).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManager), &rolloutsManager)).To(Succeed())

			Expect(rolloutsManager.Status.Phase).To(Equal(failed))

		})
	})

	When("reconcileStatusResult has a non-nil rolloutController", func() {
		It("should set the phase on the RolloutManager status", func() {

			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			failed := rolloutsmanagerv1alpha1.PhaseFailure
			rsr := reconcileStatusResult{
				rolloutController: &failed,
			}
			Expect(updateStatusConditionOfRolloutManager(ctx, rsr, &rolloutsManager, k8sClient, logger.FromContext(ctx))).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManager), &rolloutsManager)).To(Succeed())

			Expect(rolloutsManager.Status.RolloutController).To(Equal(failed))

		})
	})

	When("reconcileStatusResult contains a new condition to set on RolloutManger Status", func() {
		It("should set condition on status", func() {
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			newCondition := createCondition("my condition")

			rsr := reconcileStatusResult{
				condition: newCondition,
			}
			Expect(updateStatusConditionOfRolloutManager(ctx, rsr, &rolloutsManager, k8sClient, logger.FromContext(ctx))).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManager), &rolloutsManager)).To(Succeed())

			Expect(rolloutsManager.Status.Conditions).To(HaveLen(1))
			Expect(rolloutsManager.Status.Conditions[0].Message).To(Equal(newCondition.Message))

		})
	})

})

var _ = Describe("checkForExistingRolloutManager tests", func() {

	var (
		ctx             context.Context
		k8sClient       client.WithWatch
		rolloutsManager rolloutsmanagerv1alpha1.RolloutManager
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(rolloutsmanagerv1alpha1.AddToScheme(s)).To(Succeed())

		ctx = context.Background()
		log = logger.FromContext(ctx)

		rolloutsManager = rolloutsmanagerv1alpha1.RolloutManager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rm-1",
				Namespace: "test-ns-1",
			},
			Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
				NamespaceScoped: false,
			},
		}
		k8sClient = fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&rolloutsManager).Build()
	})

	When("A single cluster-scoped RolloutsManager is created.", func() {

		It("should not return any error, as it is a valid use case.", func() {

			By("Create only one RolloutManager.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify there is no error returned.")
			rr, err := checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())
		})
	})

	When("Multiple namespace-scoped RolloutsManagers are created.", func() {

		It("should not return error for any of them, because only one cluster-scoped or all namespace-scoped RolloutsManagers are allowed.", func() {

			By("1st RM: Create namespace-scoped RolloutsManager.")
			rolloutsManager.Spec.NamespaceScoped = true
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("1st RM: Verify there is no error returned, as only one RolloutsManager is created yet.")
			rr, err := checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())

			By("2nd RM: Create namespace-scoped RolloutsManager.")
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

			By("2nd RM: Verify there is no error returned, as all namespace-scoped RolloutsManagers are created.")
			rr, err = checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager2)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())

			By("1st RM: Recheck and it should still work, as all namespace-scoped RolloutsManagers are created.")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rolloutsManager.Name, Namespace: rolloutsManager.Namespace}, &rolloutsManager)).To(Succeed())
			rr, err = checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())
		})
	})

	When("Multiple cluster-scoped RolloutsManagers are created.", func() {

		It("should return error for all of them, because only one cluster-scoped RolloutsManagers is allowed.", func() {

			By("1st RM: Create cluster-scoped RolloutsManager.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("1st RM: Verify there is no error returned, as only one RolloutsManager is created yet.")
			rr, err := checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())

			By("2nd RM: Create another cluster-scoped RolloutsManager.")
			rolloutsManager2 := rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rm-2",
					Namespace: "test-ns-2",
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
					NamespaceScoped: false,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutsManager2)).To(Succeed())

			By("2nd RM: It should return error.")
			rr, err = checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager2)
			Expect(err).To(HaveOccurred())
			Expect(multipleRolloutManagersExist(err)).To(BeTrue())
			Expect(*rr.phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(*rr.rolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))

			By("1st RM: Recheck 1st RolloutsManager and it should also have error now. since multiple RolloutsManagers are created.")
			rr, err = checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager)
			Expect(err).To(HaveOccurred())
			Expect(multipleRolloutManagersExist(err)).To(BeTrue())
			Expect(*rr.phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(*rr.rolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
		})

		It("should return error when multiple cluster-scoped RolloutsManagers are created, and when one of them is deleted other one should start working.", func() {

			By("1st RM: Create cluster-scoped RolloutsManager.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("1st RM: Verify there is no error returned, as only one RolloutsManager is created yet.")
			rr, err := checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())

			By("2nd RM: Create another cluster-scoped RolloutsManager.")
			rolloutsManager2 := rolloutsmanagerv1alpha1.RolloutManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rm-2",
					Namespace: "test-ns-2",
				},
				Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
					NamespaceScoped: false,
				},
			}
			Expect(k8sClient.Create(ctx, &rolloutsManager2)).To(Succeed())

			By("2nd RM: It should return error.")
			rr, err = checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager2)
			Expect(err).To(HaveOccurred())
			Expect(multipleRolloutManagersExist(err)).To(BeTrue())
			Expect(*rr.phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(*rr.rolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))

			By("1st RM: Recheck 1st RolloutsManager and it should also have error now. since multiple RolloutsManagers are created.")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rolloutsManager2.Name, Namespace: rolloutsManager2.Namespace}, &rolloutsManager2)).To(Succeed())

			rr, err = checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager)
			Expect(err).To(HaveOccurred())
			Expect(multipleRolloutManagersExist(err)).To(BeTrue())
			Expect(*rr.phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(*rr.rolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))

			By("2nd RM: Delete one RolloutsManager.")
			Expect(k8sClient.Delete(ctx, &rolloutsManager2)).To(Succeed())

			By("1st RM: Verify it works now, as only one RolloutsManager is available.")
			rr, err = checkForExistingRolloutManager(ctx, k8sClient, rolloutsManager)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())
		})
	})
})

var _ = Describe("validateRolloutsScope tests", func() {

	var (
		ctx             context.Context
		k8sClient       client.WithWatch
		rolloutsManager rolloutsmanagerv1alpha1.RolloutManager
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(rolloutsmanagerv1alpha1.AddToScheme(s)).To(Succeed())

		ctx = context.Background()
		log = logger.FromContext(ctx)

		rolloutsManager = rolloutsmanagerv1alpha1.RolloutManager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rm-1",
				Namespace: "test-ns-1",
			},
			Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
				NamespaceScoped: false,
			},
		}
		k8sClient = fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&rolloutsManager).Build()
	})

	When("NAMESPACE_SCOPED_ARGO_ROLLOUTS environment variable is set to True.", func() {

		namespaceScopedArgoRolloutsController := true

		It("should return error, if cluster-scoped RolloutManager is created.", func() {

			By("Create cluster-scoped RolloutManager.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify an error is returned.")
			rr, err := validateRolloutsScope(rolloutsManager, namespaceScopedArgoRolloutsController)

			Expect(err).To(HaveOccurred())
			Expect(invalidRolloutScope(err)).To(BeTrue())
			Expect(*rr.phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(*rr.rolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
		})

		It("should not return any error, if namespace-scoped RolloutManager is created.", func() {

			By("Create namespace-scoped RolloutManager.")
			rolloutsManager.Spec.NamespaceScoped = true
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify there is no error returned.")
			rr, err := validateRolloutsScope(rolloutsManager, namespaceScopedArgoRolloutsController)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())
		})
	})

	When("NAMESPACE_SCOPED_ARGO_ROLLOUTS environment variable is set to False.", func() {

		namespaceScopedArgoRolloutsController := false

		It("should not return error, if cluster-scoped RolloutManager is created.", func() {

			By("Create cluster-scoped RolloutManager.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify there is no error returned.")
			rr, err := validateRolloutsScope(rolloutsManager, namespaceScopedArgoRolloutsController)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())
		})

		It("should return error, if namespace-scoped RolloutManager is created.", func() {

			By("Create namespace-scoped RolloutManager.")
			rolloutsManager.Spec.NamespaceScoped = true
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify an error is returned.")
			rr, err := validateRolloutsScope(rolloutsManager, namespaceScopedArgoRolloutsController)

			Expect(err).To(HaveOccurred())
			Expect(invalidRolloutScope(err)).To(BeTrue())
			Expect(*rr.phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(*rr.rolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
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

func makeTestReconciler(obj ...client.Object) *RolloutManagerReconciler {
	s := scheme.Scheme

	err := rolloutsmanagerv1alpha1.AddToScheme(s)
	Expect(err).ToNot(HaveOccurred())

	err = monitoringv1.AddToScheme(s)
	Expect(err).ToNot(HaveOccurred())

	err = crdv1.AddToScheme(s)
	Expect(err).ToNot(HaveOccurred())

	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(obj...).WithObjects(obj...).Build()

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
