package rollouts

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Resource creation and cleanup tests", func() {

	Context("Resource creation test", func() {
		var (
			ctx context.Context
			a   *v1alpha1.RolloutManager
			r   *RolloutManagerReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			a = makeTestRolloutManager()
			r = makeTestReconciler(a)
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
			clusterRole, err := r.reconcileRolloutsClusterRole(ctx)
			Expect(err).ToNot(HaveOccurred())

			By("Modify Rules of Role.")
			clusterRole.Rules[0].Verbs = append(clusterRole.Rules[0].Verbs, "test")
			Expect(r.Client.Update(ctx, clusterRole)).To(Succeed())

			By("Reconciler should revert modifications.")
			clusterRole, err = r.reconcileRolloutsClusterRole(ctx)
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
			clusterRole, err := r.reconcileRolloutsClusterRole(ctx)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, sa)).To(Succeed())

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
			Expect(r.reconcileRolloutsClusterRoleBinding(ctx, clusterRole, sa)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", crb.Name, crb)).To(Succeed())
			Expect(crb.Subjects).To(Equal(subTemp))
		})

		It("Test for reconcileRolloutsAggregateToAdminClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToAdminClusterRole(ctx)).To(Succeed())

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
			Expect(r.reconcileRolloutsAggregateToAdminClusterRole(ctx)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToAdminPolicyRules()))
		})

		It("Test for reconcileRolloutsAggregateToEditClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToEditClusterRole(ctx)).To(Succeed())

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
			Expect(r.reconcileRolloutsAggregateToEditClusterRole(ctx)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToEditPolicyRules()))
		})

		It("Test for reconcileRolloutsAggregateToViewClusterRole function", func() {
			Expect(r.reconcileRolloutsAggregateToViewClusterRole(ctx)).To(Succeed())

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
			Expect(r.reconcileRolloutsAggregateToViewClusterRole(ctx)).To(Succeed())
			Expect(fetchObject(ctx, r.Client, "", clusterRole.Name, clusterRole)).To(Succeed())
			Expect(clusterRole.Rules).To(Equal(GetAggregateToViewPolicyRules()))
		})

		It("Test for reconcileRolloutsMetricsService function", func() {
			Expect(r.reconcileRolloutsMetricsService(ctx, a)).To(Succeed())
		})

		It("Test for reconcileRolloutsSecrets function", func() {
			Expect(r.reconcileRolloutsSecrets(ctx, a)).To(Succeed())
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
					resources := []runtime.Object{a}

					r := makeTestReconciler(resources...)
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

})
