package rollouts

import (
	"context"
	"os"

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
		DescribeTable("ensure that status conditions are set accordingly", func(reason ...string) {
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			newCondition := createCondition("my condition", reason...)

			rsr := reconcileStatusResult{
				condition: newCondition,
			}
			Expect(updateStatusConditionOfRolloutManager(ctx, rsr, &rolloutsManager, k8sClient, logger.FromContext(ctx))).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&rolloutsManager), &rolloutsManager)).To(Succeed())
			Expect(rolloutsManager.Status.Conditions).To(HaveLen(1))
			Expect(rolloutsManager.Status.Conditions[0].Message).To(Equal(newCondition.Message))
			Expect(rolloutsManager.Status.Conditions[0].Reason).To(Equal(newCondition.Reason))

			// Verify whether an error condition is set when len(reason) > 1.
			if len(reason) > 1 {
				Expect(newCondition.Reason).To(Equal(rolloutsmanagerv1alpha1.RolloutManagerReasonErrorOccurred))
				Expect(newCondition.Message).To(Equal("An internal error occurred"))
				Expect(newCondition.Status).To(Equal(metav1.ConditionTrue))
			}
		},
			Entry("should set condition on status"),
			Entry("should return error when len(reason) > 1", "my reason 1", "my reason 2"))
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

var _ = Describe("combineStringMaps tests", func() {

	DescribeTable("test combineStringMaps", func(maps []map[string]string, expectedResult map[string]string) {
		res := combineStringMaps(maps...)
		Expect(res).To(Equal(expectedResult))
	},
		Entry("single element", append([]map[string]string{},
			map[string]string{"a": "b", "1": "2"}),
			map[string]string{"a": "b", "1": "2"}),
		Entry("multiple elements, no overlap", append([]map[string]string{},
			map[string]string{"a": "b", "1": "2"}, map[string]string{"c": "d", "3": "4"}),
			map[string]string{"a": "b", "1": "2", "c": "d", "3": "4"}),
		Entry("multiple elements with overlap, final element should take precedence", append([]map[string]string{},
			map[string]string{"a": "b", "1": "2", "overlap1": "Z"}, map[string]string{"c": "d", "3": "4", "overlap1": "X"}),
			map[string]string{"a": "b", "1": "2", "c": "d", "3": "4", "overlap1": "X"}),
		Entry("nil param", nil, nil),
		Entry("nil one param", append([]map[string]string{},
			map[string]string{"a": "b", "1": "2"}, nil),
			map[string]string{"a": "b", "1": "2"}),
	)
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

		BeforeEach(func() {
			By("Set Env variable.")
			os.Setenv(ClusterScopedArgoRolloutsNamespaces, rolloutsManager.Namespace)
		})

		AfterEach(func() {
			By("Unset Env variable.")
			os.Unsetenv(ClusterScopedArgoRolloutsNamespaces)
		})

		namespaceScopedArgoRolloutsController := false

		It("should not return error, if cluster-scoped RolloutManager is created in a namespace specified in env variable.", func() {

			By("Create cluster-scoped RolloutManager.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify there is no error returned.")
			rr, err := validateRolloutsScope(rolloutsManager, namespaceScopedArgoRolloutsController)
			Expect(err).ToNot(HaveOccurred())
			Expect(rr).To(BeNil())
		})

		It("should return error, if cluster-scoped RolloutManager is created in a namespace which is not specified in env variable.", func() {

			By("Unset Env variable.")
			os.Unsetenv(ClusterScopedArgoRolloutsNamespaces)

			By("Create cluster-scoped RolloutManager.")
			Expect(k8sClient.Create(ctx, &rolloutsManager)).To(Succeed())

			By("Verify there is no error returned.")
			rr, err := validateRolloutsScope(rolloutsManager, namespaceScopedArgoRolloutsController)
			Expect(err).To(HaveOccurred())
			Expect(invalidRolloutNamespace(err)).To(BeTrue())
			Expect(*rr.phase).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
			Expect(*rr.rolloutController).To(Equal(rolloutsmanagerv1alpha1.PhaseFailure))
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

var _ = Describe("removeUserLabelsAndAnnotations tests", func() {
	var (
		obj       metav1.ObjectMeta
		cr        rolloutsmanagerv1alpha1.RolloutManager
		ctx       context.Context
		k8sClient client.WithWatch
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(rolloutsmanagerv1alpha1.AddToScheme(s)).To(Succeed())

		ctx = context.Background()
		log = logger.FromContext(ctx)
		k8sClient = fake.NewClientBuilder().WithStatusSubresource(&rolloutsmanagerv1alpha1.RolloutManager{}).WithScheme(s).Build()

		cr = rolloutsmanagerv1alpha1.RolloutManager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rm-1",
				Namespace: "test-ns-1",
			},
			Spec: rolloutsmanagerv1alpha1.RolloutManagerSpec{
				NamespaceScoped: false,
			},
		}
	})

	DescribeTable("should correctly remove user labels and annotations",
		func(initialLabels, initialAnnotations, expectedLabels, expectedAnnotations map[string]string) {
			obj = metav1.ObjectMeta{
				Labels:      initialLabels,
				Annotations: initialAnnotations,
			}

			Expect(k8sClient.Create(ctx, &cr)).To(Succeed())
			setRolloutsLabelsAndAnnotations(&obj)

			removeUserLabelsAndAnnotations(&obj, cr)

			Expect(obj.Labels).To(Equal(expectedLabels))
			Expect(obj.Annotations).To(Equal(expectedAnnotations))
		},
		Entry("when no user-defined labels or annotations exist",
			map[string]string{}, map[string]string{},
			map[string]string{"app.kubernetes.io/name": DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/part-of":   DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/component": DefaultArgoRolloutsResourceName},
			map[string]string{},
		),
		Entry("when user-defined labels and annotations are present",
			map[string]string{"user-label": "value"}, map[string]string{"user-annotation": "value"},
			map[string]string{"app.kubernetes.io/name": DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/part-of":   DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/component": DefaultArgoRolloutsResourceName},
			map[string]string{},
		),
		Entry("when user-defined labels are present and annotations are empty",
			map[string]string{"user-label": "value"}, map[string]string{},
			map[string]string{"app.kubernetes.io/name": DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/part-of":   DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/component": DefaultArgoRolloutsResourceName},
			map[string]string{},
		),
		Entry("when user-defined labels are empty and annotations are present",
			map[string]string{}, map[string]string{"user-annotation": "value"},
			map[string]string{"app.kubernetes.io/name": DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/part-of":   DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/component": DefaultArgoRolloutsResourceName},
			map[string]string{},
		),

		Entry("when both user and non-user-defined labels are present, and annotations are empty",
			map[string]string{"user-label": "value",
				"app.kubernetes.io/name":      DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/part-of":   DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/component": DefaultArgoRolloutsResourceName},
			map[string]string{},
			map[string]string{"app.kubernetes.io/name": DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/part-of":   DefaultArgoRolloutsResourceName,
				"app.kubernetes.io/component": DefaultArgoRolloutsResourceName},
			map[string]string{},
		),
	)
})

var _ = Describe("insertOrUpdateConditionsInSlice tests", func() {
	var (
		existingConditions []metav1.Condition
		newCondition       metav1.Condition
	)

	Context("when the condition does not exist", func() {
		It("should add the new condition and return true", func() {
			existingConditions = []metav1.Condition{}
			newCondition = metav1.Condition{
				Type:    rolloutsmanagerv1alpha1.RolloutManagerConditionType,
				Status:  metav1.ConditionTrue,
				Reason:  "test reason",
				Message: "test message",
			}
			changed, conditions := insertOrUpdateConditionsInSlice(newCondition, existingConditions)
			Expect(changed).To(BeTrue())
			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0].Type).To(Equal(newCondition.Type))
			Expect(conditions[0].Status).To(Equal(newCondition.Status))
			Expect(conditions[0].Reason).To(Equal(newCondition.Reason))
			Expect(conditions[0].Message).To(Equal(newCondition.Message))
		})
	})

	Context("when the condition exists but is changed", func() {
		It("should update the condition and return true", func() {
			existingConditions = []metav1.Condition{
				{
					Type:    rolloutsmanagerv1alpha1.RolloutManagerConditionType,
					Status:  metav1.ConditionTrue,
					Reason:  "test reason",
					Message: "test message",
				},
			}
			newCondition = metav1.Condition{
				Type:    rolloutsmanagerv1alpha1.RolloutManagerConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  "Updated test reason",
				Message: "Updated test message",
			}
			changed, conditions := insertOrUpdateConditionsInSlice(newCondition, existingConditions)
			Expect(changed).To(BeTrue())
			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0].Type).To(Equal(newCondition.Type))
			Expect(conditions[0].Status).To(Equal(newCondition.Status))
			Expect(conditions[0].Reason).To(Equal(newCondition.Reason))
			Expect(conditions[0].Message).To(Equal(newCondition.Message))
		})
	})

	Context("when there is another unrelated condition in the list", func() {
		It("should not remove the unrelated condition and return true", func() {
			newCondition := metav1.Condition{
				Type:    rolloutsmanagerv1alpha1.RolloutManagerConditionType,
				Status:  metav1.ConditionTrue,
				Reason:  "test reason",
				Message: "test message",
			}
			unrelatedCondition := metav1.Condition{
				Type:    "UnrelatedCondition",
				Status:  metav1.ConditionFalse,
				Reason:  "some reason",
				Message: "some message",
			}
			existingConditions := []metav1.Condition{
				unrelatedCondition,
			}

			changed, conditions := insertOrUpdateConditionsInSlice(newCondition, existingConditions)
			Expect(changed).To(BeTrue())
			Expect(conditions).To(HaveLen(2))

			By("Check that the unrelated condition is still present")
			Expect(conditions).To(ContainElement(unrelatedCondition))

			By("Check that the new condition was added")
			Expect(conditions[1].Type).To(Equal(newCondition.Type))
			Expect(conditions[1].Status).To(Equal(newCondition.Status))
			Expect(conditions[1].Reason).To(Equal(newCondition.Reason))
			Expect(conditions[1].Message).To(Equal(newCondition.Message))
		})
	})

})

var _ = Describe("isMergable tests", func() {
	DescribeTable("checking for duplicate arguments", func(extraArgs, cmd []string, expectedErr bool) {
		err := isMergable(extraArgs, cmd)
		if expectedErr {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).NotTo(HaveOccurred())
		}
	},
		Entry("no extraArgs", []string{}, []string{"--cmd1", "--cmd2"}, false),
		Entry("extraArgs with no `--` args", []string{"arg1", "arg2"}, []string{"--cmd1", "--cmd2"}, false),
		Entry("extraArgs with `--` args but no duplicates", []string{"--arg1", "--arg2"}, []string{"--cmd1", "--cmd2"}, false),
		Entry("extraArgs with duplicate `--` args", []string{"--arg1", "--cmd1"}, []string{"--cmd1", "--cmd2"}, true),
	)
})

var _ = Describe("combineImageTag tests", func() {
	DescribeTable("checking for combined image and tag", func(img, tag, expected string) {
		Expect(combineImageTag(img, tag)).To(Equal(expected))
	},
		Entry("verify when tag contains `:`", DefaultArgoRolloutsImage, DefaultArgoRolloutsImage+":"+DefaultArgoRolloutsVersion, DefaultArgoRolloutsImage+"@"+DefaultArgoRolloutsImage+":"+DefaultArgoRolloutsVersion),
		Entry("verify when tag length is > 0", DefaultArgoRolloutsImage, DefaultArgoRolloutsVersion, DefaultArgoRolloutsImage+":"+DefaultArgoRolloutsVersion),
		Entry("verify when no tag is passed", DefaultArgoRolloutsImage, "", DefaultArgoRolloutsImage),
	)
})

var _ = Describe("setAdditionalRolloutsLabelsAndAnnotationsToObject tests", func() {
	var (
		obj *metav1.ObjectMeta
		cr  rolloutsmanagerv1alpha1.RolloutManager
	)

	BeforeEach(func() {
		obj = &metav1.ObjectMeta{}
		cr = rolloutsmanagerv1alpha1.RolloutManager{}
	})

	Context("when AdditionalMetadata is nil", func() {
		It("should not modify labels and annotations", func() {
			setAdditionalRolloutsLabelsAndAnnotationsToObject(obj, cr)
			Expect(obj.Labels).To(BeNil())
			Expect(obj.Annotations).To(BeNil())
		})
	})

	Context("when AdditionalMetadata is not nil", func() {
		BeforeEach(func() {
			cr.Spec.AdditionalMetadata = &rolloutsmanagerv1alpha1.ResourceMetadata{
				Labels:      map[string]string{"key1": "value1"},
				Annotations: map[string]string{"annotation1": "value1"},
			}
		})

		Context("and obj.Labels and obj.Annotations are nil", func() {
			It("should initialize and set labels and annotations", func() {
				setAdditionalRolloutsLabelsAndAnnotationsToObject(obj, cr)
				Expect(obj.Labels).To(HaveKeyWithValue("key1", "value1"))
				Expect(obj.Annotations).To(HaveKeyWithValue("annotation1", "value1"))
			})
		})

		Context("and obj.Labels and obj.Annotations are already set", func() {

			It("should merge labels and annotations", func() {
				obj.Labels = map[string]string{"existingKey": "existingValue"}
				obj.Annotations = map[string]string{"existingAnnotation": "existingValue"}

				setAdditionalRolloutsLabelsAndAnnotationsToObject(obj, cr)
				Expect(obj.Labels).To(HaveKeyWithValue("existingKey", "existingValue"))
				Expect(obj.Labels).To(HaveKeyWithValue("key1", "value1"))
				Expect(obj.Annotations).To(HaveKeyWithValue("existingAnnotation", "existingValue"))
				Expect(obj.Annotations).To(HaveKeyWithValue("annotation1", "value1"))
			})
		})

		Context("and obj.Labels and obj.Annotations are are already set with different values", func() {

			It("should replace the existing values with the new values", func() {
				obj.Labels = map[string]string{"key1": "oldValue"}
				obj.Annotations = map[string]string{"annotation1": "oldValue"}

				cr.Spec.AdditionalMetadata = &rolloutsmanagerv1alpha1.ResourceMetadata{
					Labels:      map[string]string{"key1": "newValue"},
					Annotations: map[string]string{"annotation1": "newValue"},
				}

				setAdditionalRolloutsLabelsAndAnnotationsToObject(obj, cr)
				Expect(obj.Labels).To(HaveKeyWithValue("key1", "newValue"))
				Expect(obj.Annotations).To(HaveKeyWithValue("annotation1", "newValue"))
			})
		})

	})
})

var _ = Describe("envMerge tests", func() {
	DescribeTable("merges two slices of EnvVar entries",
		func(existing, merge, expected []corev1.EnvVar, override bool) {
			result := envMerge(existing, merge, override)
			Expect(result).To(Equal(expected))
		},
		Entry("when both slices are empty",
			[]corev1.EnvVar{},
			[]corev1.EnvVar{},
			[]corev1.EnvVar{},
			false),
		Entry("when existing is empty and merge has one element",
			[]corev1.EnvVar{},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}},
			false),
		Entry("when existing has one element and merge is empty",
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}},
			[]corev1.EnvVar{},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}},
			false),
		Entry("when merging with no override and no conflicts",
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}},
			[]corev1.EnvVar{{Name: "test-name-2", Value: "test-value-2"}},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}, {Name: "test-name-2", Value: "test-value-2"}},
			false),
		Entry("when merging with no override and with conflicts",
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-2"}},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}},
			false),
		Entry("when merging with override and with conflicts",
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-2"}},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-2"}},
			true),
		Entry("when merging with multiple elements and override",
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}, {Name: "test-name-3", Value: "test-value-3"}},
			[]corev1.EnvVar{{Name: "test-name-2", Value: "test-value-2"}, {Name: "test-name-3", Value: "new-value-3"}},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}, {Name: "test-name-2", Value: "test-value-2"}, {Name: "test-name-3", Value: "new-value-3"}},
			true),
		Entry("when merging with multiple elements and no override",
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}, {Name: "test-name-3", Value: "test-value-3"}},
			[]corev1.EnvVar{{Name: "test-name-2", Value: "test-value-2"}, {Name: "test-name-3", Value: "new-value-3"}},
			[]corev1.EnvVar{{Name: "test-name-1", Value: "test-value-1"}, {Name: "test-name-2", Value: "test-value-2"}, {Name: "test-name-3", Value: "test-value-3"}},
			false),
	)
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
