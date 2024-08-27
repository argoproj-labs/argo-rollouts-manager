package rollouts

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Resource creation and cleanup tests", func() {

	Context("Verify resource creation when RolloutManger does not contain a user-defined label/annotation", func() {
		var (
			ctx context.Context
			a   v1alpha1.RolloutManager
			r   *RolloutManagerReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			a = *makeTestRolloutManager()
			r = makeTestReconciler(&a)
			err := createNamespace(r, a.Namespace)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Test for reconcileRolloutsServiceAccount function", func() {
			_, err := r.reconcileRolloutsServiceAccount(ctx, a)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Test for reconcileRolloutsRole function", func() {
			role, err := r.reconcileRolloutsRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())

			By("Modify Rules of Role.")
			role.Rules[0].Verbs = append(role.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, role)).To(Succeed())

			By("Reconciler should revert modifications.")
			role, err = r.reconcileRolloutsRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			Expect(role.Rules).To(Equal(GetPolicyRules()))
		})

		It("Test for reconcileRolloutsClusterRole function", func() {
			clusterRole, err := r.reconcileRolloutsClusterRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())

			By("Modify Rules of Role.")
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Reconciler should revert modifications.")
			clusterRole, err = r.reconcileRolloutsClusterRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterRole.Rules).To(Equal(GetPolicyRules()))
		})

		It("Test for reconcileRolloutsRoleBinding function", func() {
			sa, err := r.reconcileRolloutsServiceAccount(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			role, err := r.reconcileRolloutsRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.reconcileRolloutsRoleBinding(ctx, a, role, sa)).To(Succeed())

			By("Modify Subject of RoleBinding.")
			rb := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultArgoRolloutsResourceName,
					Namespace: a.Namespace,
				},
			}
			Expect(fetchObject(ctx, r.Client, a.Namespace, rb.Name, rb)).To(Succeed())

			subTemp := rb.Subjects
			rb.Subjects = append(rb.Subjects, rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "test", Namespace: "test"})
			Expect(r.Client.Update(ctx, rb)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsRoleBinding(ctx, a, role, sa)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, a.Namespace, rb.Name, rb)).To(Succeed())
			Expect(rb.Subjects).To(Equal(subTemp))
		})

		It("Test for reconcileRolloutsClusterRoleBinding function", func() {
			sa, err := r.reconcileRolloutsServiceAccount(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			clusterRole, err := r.reconcileRolloutsClusterRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, sa, a)).To(Succeed())

			By("Modify Subject of ClusterRoleBinding.")
			crb := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: DefaultArgoRolloutsResourceName,
				},
			}
			Expect(fetchObject(ctx, r.Client, "", crb.Name, crb)).To(Succeed())

			subTemp := crb.Subjects
			crb.Subjects = append(crb.Subjects, rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "test", Namespace: "test"})
			Expect(r.Client.Update(ctx, crb)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, sa, a)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", crb.Name, crb)).To(Succeed())
			Expect(crb.Subjects).To(Equal(subTemp))
		})

		It("Test for reconcileRolloutsAggregateToAdminClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToAdminClusterRole(ctx, a)).To(Succeed())

			By("Modify Rules of ClusterRole.")
			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-admin",
				},
			}
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsAggregateToAdminClusterRole(ctx, a)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToAdminPolicyRules()))
		})

		It("Test for reconcileRolloutsAggregateToEditClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToEditClusterRole(ctx, a)).To(Succeed())

			By("Modify Rules of ClusterRole.")
			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-edit",
				},
			}
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsAggregateToEditClusterRole(ctx, a)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToEditPolicyRules()))
		})

		It("Test for reconcileRolloutsAggregateToViewClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToViewClusterRole(ctx, a)).To(Succeed())

			By("Modify Rules of ClusterRole.")
			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-view",
				},
			}
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsAggregateToViewClusterRole(ctx, a)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToViewPolicyRules()))
		})

		It("Test for reconcileRolloutsMetricsService function", func() {
			Expect(r.reconcileRolloutsMetricsServiceAndMonitor(ctx, a)).To(Succeed())
		})

		It("Test for reconcileRolloutsSecrets function", func() {
			Expect(r.reconcileRolloutsSecrets(ctx, a)).To(Succeed())
		})

		It("test for removeClusterScopedResourcesIfApplicable function", func() {

			By("creating default cluster-scoped ClusterRole/ClusterRoleBinding. These should be deleted by the call to removeClusterScopedResourcesIfApplicable")
			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: DefaultArgoRolloutsResourceName,
				},
			}
			Expect(r.Client.Create(ctx, clusterRole)).To(Succeed())

			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: DefaultArgoRolloutsResourceName,
				},
			}
			Expect(r.Client.Create(ctx, clusterRoleBinding)).To(Succeed())

			By("creating '*aggregate* clusterRoles")
			clusterRoleAdmin := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-admin",
				},
			}
			Expect(r.Client.Create(ctx, clusterRoleAdmin)).To(Succeed())

			clusterRoleEdit := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-edit",
				},
			}
			Expect(r.Client.Create(ctx, clusterRoleEdit)).To(Succeed())

			clusterRoleView := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-view",
				},
			}
			Expect(r.Client.Create(ctx, clusterRoleView)).To(Succeed())

			By("creating default cluster-scoped ClusterRole/ClusterRoleBinding with a different name. These should not be deleted")

			unrelatedRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unrelated-resource-should-not-be-deleted",
				},
			}
			Expect(r.Client.Create(ctx, unrelatedRole)).To(Succeed())

			unrelatedRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unrelated-resource-should-not-be-deleted",
				},
			}
			Expect(r.Client.Create(ctx, unrelatedRoleBinding)).To(Succeed())

			By("calling removeClusterScopedResourcesIfApplicable, which should delete the cluster scoped resources")
			Expect(r.removeClusterScopedResourcesIfApplicable(ctx)).To(Succeed())

			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRole), clusterRole)).ToNot(Succeed(),
				"ClusterRole should have been deleted")
			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)).ToNot(Succeed(), "ClusterRoleBinding should have been deleted")

			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(unrelatedRole), unrelatedRole)).To(Succeed(),
				"Unrelated ClusterRole should not have been deleted")
			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(unrelatedRoleBinding), unrelatedRoleBinding)).To(Succeed(), "Unrelated ClusterRoleBinding should not have been deleted")

			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRoleAdmin), clusterRoleAdmin)).ToNot(Succeed(),
				"ClusterRole should have been deleted")
			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRoleEdit), clusterRoleEdit)).ToNot(Succeed(),
				"ClusterRole should have been deleted")
			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRoleView), clusterRoleView)).ToNot(Succeed(),
				"ClusterRole should have been deleted")

			Expect(r.removeClusterScopedResourcesIfApplicable(ctx)).To(Succeed(), "calling the function again should not return an error")

		})
	})

	Context("Verify resource creation when RolloutManger contains a user-defined label/annotation", func() {
		var (
			ctx context.Context
			a   v1alpha1.RolloutManager
			r   *RolloutManagerReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			a = *makeTestRolloutManager()

			a.Spec = v1alpha1.RolloutManagerSpec{
				AdditionalMetadata: &v1alpha1.ResourceMetadata{
					Labels: map[string]string{
						"keylabel": "valuelabel",
					},
					Annotations: map[string]string{
						"keyannotation": "valueannotation",
					},
				},
			}

			r = makeTestReconciler(&a)
			err := createNamespace(r, a.Namespace)
			Expect(err).ToNot(HaveOccurred())

		})

		It("Test for reconcileRolloutsServiceAccount function", func() {
			sa, err := r.reconcileRolloutsServiceAccount(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			Expect(sa.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(sa.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsRole function", func() {
			role, err := r.reconcileRolloutsRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			Expect(role.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(role.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))

			By("Modify Rules of Role.")
			role.Rules[0].Verbs = append(role.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, role)).To(Succeed())

			By("Modify Labels of RM to verify whether label is updated in Role.")
			a.Spec.AdditionalMetadata.Labels["keylabel"] = "keylabel-update"
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("Reconciler should revert modifications.")
			role, err = r.reconcileRolloutsRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			Expect(role.Rules).To(Equal(GetPolicyRules()))
			Expect(role.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(role.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsClusterRole function", func() {
			clusterRole, err := r.reconcileRolloutsClusterRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))

			By("Modify Rules of ClusterRole.")
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Modify Labels of RM to verify whether label and annotation is updated in ClusterRole.")
			a.Spec.AdditionalMetadata.Labels["keylabel"] = "keylabel-update"
			a.Spec.AdditionalMetadata.Annotations["keyannotation"] = "keyannotation-update"
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("Reconciler should revert modifications.")
			clusterRole, err = r.reconcileRolloutsClusterRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			Expect(clusterRole.Rules).To(Equal(GetPolicyRules()))
			Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsRoleBinding function", func() {
			sa, err := r.reconcileRolloutsServiceAccount(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			role, err := r.reconcileRolloutsRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.reconcileRolloutsRoleBinding(ctx, a, role, sa)).To(Succeed())

			rb := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultArgoRolloutsResourceName,
					Namespace: a.Namespace,
				},
			}
			Expect(fetchObject(ctx, r.Client, a.Namespace, rb.Name, rb)).To(Succeed())

			By("Verify labels and annotations of RoleBinding are updated.")
			Expect(rb.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(rb.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))

			By("Modify Subject of RoleBinding.")
			subTemp := rb.Subjects
			rb.Subjects = append(rb.Subjects, rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "test", Namespace: "test"})
			Expect(r.Client.Update(ctx, rb)).To(Succeed())

			By("Modify Labels of RM to verify whether label and annotation is updated in RoleBinding(.")
			a.Spec.AdditionalMetadata.Labels["keylabel"] = "keylabel-update"
			a.Spec.AdditionalMetadata.Annotations["keyannotation"] = "keyannotation-update"
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsRoleBinding(ctx, a, role, sa)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, a.Namespace, rb.Name, rb)).To(Succeed())
			Expect(rb.Subjects).To(Equal(subTemp))
			Expect(rb.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(rb.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsClusterRoleBinding function", func() {
			sa, err := r.reconcileRolloutsServiceAccount(ctx, a)
			Expect(err).ToNot(HaveOccurred())
			clusterRole, err := r.reconcileRolloutsClusterRole(ctx, a)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, sa, a)).To(Succeed())

			crb := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: DefaultArgoRolloutsResourceName,
				},
			}
			Expect(fetchObject(ctx, r.Client, "", crb.Name, crb)).To(Succeed())

			By("Verify labels and annotations of ClusterRoleBinding are updated.")
			Expect(crb.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(crb.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))

			By("Modify Subject of ClusterRoleBinding.")
			subTemp := crb.Subjects
			crb.Subjects = append(crb.Subjects, rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "test", Namespace: "test"})
			Expect(r.Client.Update(ctx, crb)).To(Succeed())

			By("Modify Labels of RM to verify whether label and annotation is updated in ClusterRoleBinding.")
			a.Spec.AdditionalMetadata.Labels["keylabel"] = "keylabel-update"
			a.Spec.AdditionalMetadata.Annotations["keyannotation"] = "keyannotation-update"
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, sa, a)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", crb.Name, crb)).To(Succeed())
			Expect(crb.Subjects).To(Equal(subTemp))
			Expect(crb.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(crb.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsAggregateToAdminClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToAdminClusterRole(ctx, a)).To(Succeed())

			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-admin",
				},
			}
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())

			By("Verify labels and annotations of ClusterRole are updated.")
			Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))

			By("Modify Rules of ClusterRole.")
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Modify Labels of RM to verify whether label and annotation is updated in ClusterRole.")
			a.Spec.AdditionalMetadata.Labels["keylabel"] = "keylabel-update"
			a.Spec.AdditionalMetadata.Annotations["keyannotation"] = "keyannotation-update"
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsAggregateToAdminClusterRole(ctx, a)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToAdminPolicyRules()))
			Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsAggregateToEditClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToEditClusterRole(ctx, a)).To(Succeed())

			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-edit",
				},
			}
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())

			By("Verify labels and annotations of ClusterRole are updated.")
			Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))

			By("Modify Rules of ClusterRole.")
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Modify Labels of RM to verify whether label and annotation is updated in ClusterRole.")
			a.Spec.AdditionalMetadata.Labels["keylabel"] = "keylabel-update"
			a.Spec.AdditionalMetadata.Annotations["keyannotation"] = "keyannotation-update"
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsAggregateToEditClusterRole(ctx, a)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToEditPolicyRules()))
			Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsAggregateToViewClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToViewClusterRole(ctx, a)).To(Succeed())

			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argo-rollouts-aggregate-to-view",
				},
			}
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())

			By("Verify labels and annotations of ClusterRole are updated.")
			Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))

			By("Modify Rules of ClusterRole.")
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Modify Labels of RM to verify whether label and annotation is updated in ClusterRole.")
			a.Spec.AdditionalMetadata.Labels["keylabel"] = "keylabel-update"
			a.Spec.AdditionalMetadata.Annotations["keyannotation"] = "keyannotation-update"
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("Reconciler should revert modifications.")
			Expect(r.reconcileRolloutsAggregateToViewClusterRole(ctx, a)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToViewPolicyRules()))
			Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsMetricsService function", func() {
			Expect(r.reconcileRolloutsMetricsServiceAndMonitor(ctx, a)).To(Succeed())
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultArgoRolloutsMetricsServiceName,
					Namespace: a.Namespace,
				},
			}
			Expect(fetchObject(ctx, r.Client, a.Namespace, service.Name, service)).To(Succeed())
			Expect(service.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(service.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("Test for reconcileRolloutsSecrets function", func() {
			Expect(r.reconcileRolloutsSecrets(ctx, a)).To(Succeed())
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultRolloutsNotificationSecretName,
					Namespace: a.Namespace,
				},
			}
			Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
			Expect(secret.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
		})

		It("test for removeClusterScopedResourcesIfApplicable function", func() {

			By("creating default cluster-scoped ClusterRole/ClusterRoleBinding. These should be deleted by the call to removeClusterScopedResourcesIfApplicable")
			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: DefaultArgoRolloutsResourceName,
				},
			}
			Expect(r.Client.Create(ctx, clusterRole)).To(Succeed())

			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: DefaultArgoRolloutsResourceName,
				},
			}
			Expect(r.Client.Create(ctx, clusterRoleBinding)).To(Succeed())

			By("creating default cluster-scoped ClusterRole/ClusterRoleBinding with a different name. These should not be deleted")

			unrelatedRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unrelated-resource-should-not-be-deleted",
				},
			}
			Expect(r.Client.Create(ctx, unrelatedRole)).To(Succeed())

			unrelatedRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unrelated-resource-should-not-be-deleted",
				},
			}
			Expect(r.Client.Create(ctx, unrelatedRoleBinding)).To(Succeed())

			By("calling removeClusterScopedResourcesIfApplicable, which should delete the cluster scoped resources")
			Expect(r.removeClusterScopedResourcesIfApplicable(ctx)).To(Succeed())

			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRole), clusterRole)).ToNot(Succeed(),
				"ClusterRole should have been deleted")
			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), clusterRoleBinding)).ToNot(Succeed(), "ClusterRoleBinding should have been deleted")

			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(unrelatedRole), unrelatedRole)).To(Succeed(),
				"Unrelated ClusterRole should not have been deleted")
			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(unrelatedRoleBinding), unrelatedRoleBinding)).To(Succeed(), "Unrelated ClusterRoleBinding should not have been deleted")

			Expect(r.removeClusterScopedResourcesIfApplicable(ctx)).To(Succeed(), "calling the function again should not return an error")

		})

		Context("Verify whether existing labels are not getting removed from resources", func() {
			var err error
			It("verify for ServiceAccount", func() {
				serviceAccount := createServiceAccount(DefaultArgoRolloutsResourceName, a.Namespace, map[string]string{
					"my-label": "my-value",
				})
				Expect(r.Client.Create(ctx, serviceAccount)).To(Succeed())

				serviceAccount, err = r.reconcileRolloutsServiceAccount(ctx, a)
				Expect(err).ToNot(HaveOccurred())
				Expect(serviceAccount.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(serviceAccount.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(serviceAccount.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(serviceAccount.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})

			It("verify for Role", func() {
				role := createRole(DefaultArgoRolloutsResourceName, a.Namespace, map[string]string{
					"my-label": "my-value",
				})
				Expect(r.Client.Create(ctx, role)).To(Succeed())

				role, err = r.reconcileRolloutsRole(ctx, a)
				Expect(err).ToNot(HaveOccurred())

				Expect(role.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(role.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(role.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(role.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})

			It("verify for ClusterRole", func() {
				clusterRole := createClusterRole(DefaultArgoRolloutsResourceName, map[string]string{
					"my-label": "my-value",
				})
				Expect(r.Client.Create(ctx, clusterRole)).To(Succeed())

				clusterRole, err = r.reconcileRolloutsClusterRole(ctx, a)
				Expect(err).ToNot(HaveOccurred())

				Expect(clusterRole.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(clusterRole.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(clusterRole.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(clusterRole.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})
			It("verify for RoleBinding", func() {
				serviceAccount := createServiceAccount(DefaultArgoRolloutsResourceName, a.Namespace, map[string]string{
					"my-label": "my-value",
				})
				Expect(r.Client.Create(ctx, serviceAccount)).To(Succeed())

				role := createRole(DefaultArgoRolloutsResourceName, a.Namespace, map[string]string{
					"my-label": "my-value",
				})
				Expect(r.Client.Create(ctx, role)).To(Succeed())

				rb := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultArgoRolloutsResourceName,
						Namespace: a.Namespace,
						Labels: map[string]string{
							"my-label": "my-value",
						},
					},
				}
				Expect(r.Client.Create(ctx, rb)).To(Succeed())

				err = r.reconcileRolloutsRoleBinding(ctx, a, role, serviceAccount)
				Expect(err).ToNot(HaveOccurred())

				Expect(fetchObject(ctx, r.Client, a.Namespace, rb.Name, rb)).To(Succeed())

				Expect(rb.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(rb.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(rb.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(rb.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})

			It("verify for ClusterRoleBinding", func() {
				serviceAccount := createServiceAccount(DefaultArgoRolloutsResourceName, a.Namespace, map[string]string{
					"my-label": "my-value",
				})
				Expect(r.Client.Create(ctx, serviceAccount)).To(Succeed())

				clusterRole := createClusterRole(DefaultArgoRolloutsResourceName, map[string]string{
					"my-label": "my-value",
				})
				Expect(r.Client.Create(ctx, clusterRole)).To(Succeed())
				crb := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: DefaultArgoRolloutsResourceName,
						Labels: map[string]string{
							"my-label": "my-value",
						},
					},
				}
				Expect(r.Client.Create(ctx, crb)).To(Succeed())

				err = r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, serviceAccount, a)
				Expect(err).ToNot(HaveOccurred())

				Expect(fetchObject(ctx, r.Client, "", crb.Name, crb)).To(Succeed())

				Expect(crb.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(crb.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(crb.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(crb.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})

			It("verify for aggregate-to-admin ClusterRole", func() {
				clusterRoleAggregateToAdmin := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "argo-rollouts-aggregate-to-admin",
						Labels: map[string]string{
							"my-label": "my-value",
						},
					},
				}
				Expect(r.Client.Create(ctx, clusterRoleAggregateToAdmin)).To(Succeed())

				err = r.reconcileRolloutsAggregateToAdminClusterRole(ctx, a)
				Expect(err).ToNot(HaveOccurred())

				Expect(fetchObject(ctx, r.Client, "", clusterRoleAggregateToAdmin.Name, clusterRoleAggregateToAdmin)).To(Succeed())

				Expect(clusterRoleAggregateToAdmin.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(clusterRoleAggregateToAdmin.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(clusterRoleAggregateToAdmin.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(clusterRoleAggregateToAdmin.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})

			It("verify for aggregate-to-edit ClusterRole", func() {
				clusterRoleAggregateToEdit := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "argo-rollouts-aggregate-to-edit",
						Labels: map[string]string{
							"my-label": "my-value",
						},
					},
				}
				Expect(r.Client.Create(ctx, clusterRoleAggregateToEdit)).To(Succeed())

				err = r.reconcileRolloutsAggregateToEditClusterRole(ctx, a)
				Expect(err).ToNot(HaveOccurred())

				Expect(fetchObject(ctx, r.Client, "", clusterRoleAggregateToEdit.Name, clusterRoleAggregateToEdit)).To(Succeed())

				Expect(clusterRoleAggregateToEdit.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(clusterRoleAggregateToEdit.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(clusterRoleAggregateToEdit.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(clusterRoleAggregateToEdit.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})
			It("verify for aggregate-to-view ClusterRole", func() {
				clusterRoleAggregateToView := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "argo-rollouts-aggregate-to-view",
						Labels: map[string]string{
							"my-label": "my-value",
						},
					},
				}
				Expect(r.Client.Create(ctx, clusterRoleAggregateToView)).To(Succeed())

				err = r.reconcileRolloutsAggregateToViewClusterRole(ctx, a)
				Expect(err).ToNot(HaveOccurred())

				Expect(fetchObject(ctx, r.Client, "", clusterRoleAggregateToView.Name, clusterRoleAggregateToView)).To(Succeed())

				Expect(clusterRoleAggregateToView.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(clusterRoleAggregateToView.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(clusterRoleAggregateToView.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(clusterRoleAggregateToView.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})

			It("verify for Service", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultArgoRolloutsMetricsServiceName,
						Namespace: a.Namespace,
						Labels: map[string]string{
							"my-label": "my-value",
						},
					},
				}
				Expect(r.Client.Create(ctx, svc)).To(Succeed())

				err = r.reconcileRolloutsMetricsServiceAndMonitor(ctx, a)
				Expect(err).ToNot(HaveOccurred())

				Expect(fetchObject(ctx, r.Client, a.Namespace, svc.Name, svc)).To(Succeed())

				Expect(svc.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(svc.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(svc.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(svc.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})

			It("verify for Secret", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultRolloutsNotificationSecretName,
						Namespace: a.Namespace,
						Labels: map[string]string{
							"my-label": "my-value",
						},
					},
					Type: corev1.SecretTypeOpaque,
				}
				Expect(r.Client.Create(ctx, secret)).To(Succeed())

				err = r.reconcileRolloutsSecrets(ctx, a)
				Expect(err).ToNot(HaveOccurred())

				Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).To(Succeed())

				Expect(secret.ObjectMeta.Labels["my-label"]).ToNot(BeEmpty())
				Expect(secret.ObjectMeta.Labels["my-label"]).To(Equal("my-value"))
				Expect(secret.ObjectMeta.Labels["keylabel"]).To(Equal(a.Spec.AdditionalMetadata.Labels["keylabel"]))
				Expect(secret.ObjectMeta.Annotations["keyannotation"]).To(Equal(a.Spec.AdditionalMetadata.Annotations["keyannotation"]))
			})
		})
	})

	Context("Resource Cleanup test", func() {
		a := makeTestRolloutManager()
		tt := []struct {
			name     string
			resource client.Object
		}{
			{
				fmt.Sprintf("ServiceAccount %s", DefaultArgoRolloutsResourceName),
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultArgoRolloutsResourceName,
						Namespace: a.Namespace,
					},
				},
			},
			{
				fmt.Sprintf("Role %s", DefaultArgoRolloutsResourceName),
				&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultArgoRolloutsResourceName,
						Namespace: a.Namespace,
					},
				},
			},
			{
				fmt.Sprintf("RoleBinding %s", DefaultArgoRolloutsResourceName),
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultArgoRolloutsResourceName,
						Namespace: a.Namespace,
					},
				},
			},
			{
				fmt.Sprintf("Secret %s", DefaultRolloutsNotificationSecretName),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultRolloutsNotificationSecretName,
						Namespace: a.Namespace,
					},
					Type: corev1.SecretTypeOpaque,
				},
			},
			{
				fmt.Sprintf("Service %s", DefaultArgoRolloutsResourceName),
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultArgoRolloutsResourceName,
						Namespace: a.Namespace,
					},
				},
			},
			{
				fmt.Sprintf("ServiceMonitor %s", DefaultArgoRolloutsResourceName),
				&monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DefaultArgoRolloutsResourceName,
						Namespace: a.Namespace,
					},
				},
			},
		}

		for _, test := range tt {
			When(test.name, func() {
				It("Cleans up all resources created for RolloutManager", func() {

					ctx := context.Background()
					req := reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      a.Name,
							Namespace: a.Namespace,
						},
					}

					r := makeTestReconciler(a)
					err := createNamespace(r, a.Namespace)
					Expect(err).ToNot(HaveOccurred())

					res, err := r.Reconcile(ctx, req)
					Expect(err).NotTo(HaveOccurred())
					Expect(res.Requeue).Should(BeFalse(), "reconcile should not requeue request")

					err = r.Client.Delete(ctx, a)
					Expect(err).ToNot(HaveOccurred())
					Expect(fetchObject(ctx, r.Client, a.Namespace, test.name, test.resource)).ToNot(Succeed(), fmt.Sprintf("Expected %s to be deleted", test.name))
				})
			})
		}
	})

	Context("Rollouts Metrics ServiceMonitor test", func() {
		var (
			ctx context.Context
			a   *v1alpha1.RolloutManager
			r   *RolloutManagerReconciler
			req reconcile.Request
		)

		BeforeEach(func() {
			ctx = context.Background()
			a = makeTestRolloutManager()
			r = makeTestReconciler(a)
			err := createNamespace(r, a.Namespace)
			Expect(err).ToNot(HaveOccurred())
			req = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      a.Name,
					Namespace: a.Namespace,
				},
			}
		})

		It("Verify whether RolloutManager creating ServiceMonitor", func() {
			smCRD := &crdv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: serviceMonitorsCRDName,
				},
			}
			Expect(r.Client.Create(ctx, smCRD)).To(Succeed())

			res, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Requeue).Should(BeFalse(), "reconcile should not requeue request")

			expectedServiceMonitor := serviceMonitor()

			sm := &monitoringv1.ServiceMonitor{}
			Expect(r.Client.Get(ctx, types.NamespacedName{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: testNamespace,
			}, sm)).To(Succeed())

			Expect(sm.Name).To(Equal(expectedServiceMonitor.Name))
			Expect(sm.Namespace).To(Equal(expectedServiceMonitor.Namespace))
			Expect(sm.Spec).To(Equal(expectedServiceMonitor.Spec))

			service := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultArgoRolloutsMetricsServiceName,
					Namespace: testNamespace,
				},
			}
			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(&service), &service)).To(Succeed(), "service should exist after reconcile call")
		})

		It("Verify if ServiceMonitor exists, but has different content than we expect then it should update ServiceMonitor", func() {

			smCRD := &crdv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: serviceMonitorsCRDName,
				},
			}
			Expect(r.Client.Create(ctx, smCRD)).To(Succeed())

			existingServiceMonitor := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultArgoRolloutsResourceName,
					Namespace: testNamespace,
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": "test-label",
						},
					},
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: "metrics-test",
						},
					},
				},
			}

			Expect(r.Client.Create(ctx, existingServiceMonitor)).To(Succeed())

			res, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Requeue).Should(BeFalse(), "reconcile should not requeue request")

			expectedSM := serviceMonitor()

			Expect(r.Client.Get(ctx, types.NamespacedName{
				Name:      DefaultArgoRolloutsResourceName,
				Namespace: testNamespace,
			}, existingServiceMonitor)).To(Succeed())

			Expect(existingServiceMonitor.Name).To(Equal(expectedSM.Name))
			Expect(existingServiceMonitor.Namespace).To(Equal(expectedSM.Namespace))
			Expect(existingServiceMonitor.Spec).To(Equal(expectedSM.Spec))

			service := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultArgoRolloutsMetricsServiceName,
					Namespace: testNamespace,
				},
			}
			Expect(r.Client.Get(ctx, client.ObjectKeyFromObject(&service), &service)).To(Succeed(), "service should exist after reconcile call")

		})

		It("Verify ServiceMonitor is not created if the CRD does not exist.", func() {
			res, err := r.Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Requeue).Should(BeFalse(), "reconcile should not requeue request")

			sm := &monitoringv1.ServiceMonitor{}
			Expect(r.Client.Get(ctx, types.NamespacedName{
				Name:      DefaultArgoRolloutsMetricsServiceName,
				Namespace: testNamespace,
			}, sm)).ToNot(Succeed())
		})
	})

	Context("Rollouts notification secret reconciliation tests", func() {
		var (
			ctx context.Context
			a   v1alpha1.RolloutManager
			r   *RolloutManagerReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			a = *makeTestRolloutManager()
			r = makeTestReconciler(&a)
			err := createNamespace(r, a.Namespace)
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("Verify that for RolloutManager with SkipNotificationSecretDeployment, it either creates or doesn't create a Secret, based on that value", func(skipNotificationInitialValue bool) {

			By(fmt.Sprintf("Creating RolloutManager with SkipNotificationSecretDeployment set to initial value '%v'", skipNotificationInitialValue))
			a.Spec.SkipNotificationSecretDeployment = skipNotificationInitialValue
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("calling reconcileRolloutsSecrets")
			Expect(r.reconcileRolloutsSecrets(ctx, a)).To(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultRolloutsNotificationSecretName,
					Namespace: a.Namespace,
				},
				Type: corev1.SecretTypeOpaque,
			}

			if a.Spec.SkipNotificationSecretDeployment {
				Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).ToNot(Succeed(), "secret should not exist after reconcile call")
			} else {
				Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).To(Succeed(), "secret should exist after reconcile call")
			}

			By(fmt.Sprintf("Updating the RolloutManager with SkipNotificationSecretDeployment set to '%v'", !skipNotificationInitialValue))
			a.Spec.SkipNotificationSecretDeployment = !skipNotificationInitialValue
			Expect(r.Client.Update(ctx, &a)).To(Succeed())

			By("calling reconcileRolloutsSecrets")
			Expect(r.reconcileRolloutsSecrets(ctx, a)).To(Succeed())

			if a.Spec.SkipNotificationSecretDeployment {
				Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).ToNot(Succeed(), "secret should not exist after reconcile call")
			} else {
				Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).To(Succeed(), "secret should exist after reconcile call")
			}

		},
			Entry("SkipNotificationSecretDeployment is initially true", true),
			Entry("SkipNotificationSecretDeployment is initially false", false),
		)

		It("Verify that RolloutManager does not update an existing notification secret if it doesn't have the ownership", func() {
			By("Creating RolloutManager with SkipNotificationSecretDeployment set to true")

			a.Spec.SkipNotificationSecretDeployment = true
			Expect(r.Client.Update(ctx, &a)).To(Succeed())
			Expect(r.reconcileRolloutsSecrets(ctx, a)).To(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DefaultRolloutsNotificationSecretName,
					Namespace: a.Namespace,
				},
				Type: corev1.SecretTypeOpaque,
			}
			Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).ToNot(Succeed(), "secret should not exist after reconcile call")

			By("Create the secret without the owner reference")
			Expect(r.Client.Create(ctx, secret)).To(Succeed())

			By("Call reconcileRolloutsSecrets")
			Expect(r.reconcileRolloutsSecrets(ctx, a)).To(Succeed())

			By("Verifying that the secret was not updated")
			Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).To(Succeed(), "secret should exist after reconcile call, even though SkipNotificationSecretDeployment is still true")
			Expect(secret.OwnerReferences).To(BeNil())

			By("Adding another owner reference")
			testRef := metav1.OwnerReference{
				Name: "test",
			}
			secret.OwnerReferences = append(secret.OwnerReferences, testRef)
			Expect(r.Client.Update(ctx, secret)).To(Succeed())

			By("Updating the RolloutManager")
			Expect(r.reconcileRolloutsSecrets(ctx, a)).To(Succeed())

			By("Verifying that the secret was not updated")
			Expect(fetchObject(ctx, r.Client, a.Namespace, secret.Name, secret)).To(Succeed(), "secret should exist after reconcile call")
			Expect(secret.OwnerReferences).To(ContainElement(testRef))
			Expect(len(secret.OwnerReferences)).To(Equal(1))
		})
	})

})

func serviceMonitor() *monitoringv1.ServiceMonitor {
	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: testNamespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": DefaultArgoRolloutsMetricsServiceName,
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: "metrics",
				},
			},
		},
	}
	return sm
}

func createServiceAccount(name, namespace string, labels map[string]string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func createRole(name, namespace string, labels map[string]string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
}

func createClusterRole(name string, labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
