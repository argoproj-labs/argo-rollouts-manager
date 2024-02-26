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

var _ = Describe("ReconcileRolloutManager tests", func() {
	var ctx context.Context
	var req reconcile.Request
	var a *v1alpha1.RolloutManager
	var r *RolloutManagerReconciler

	BeforeEach(func() {
		ctx = context.Background()
		a = makeTestRolloutManager()
		r = makeTestReconciler(a)
		Expect(createNamespace(r, a.Namespace)).To(Succeed())

		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      a.Name,
				Namespace: a.Namespace,
			},
		}
	})

	It("Test to verify RolloutsResources", func() {
		res, err := r.Reconcile(ctx, req)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).Should(BeFalse(), "reconcile should not requeue request")

		sa := &corev1.ServiceAccount{}
		Expect(r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: testNamespace,
		}, sa)).To(Succeed(), fmt.Sprintf("failed to find the rollouts serviceaccount: %#v\n", err))

		role := &rbacv1.Role{}
		Expect(r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: testNamespace,
		}, role)).To(Succeed(), fmt.Sprintf("failed to find the rollouts role: %#v\n", err))

		roleBinding := &rbacv1.RoleBinding{}
		Expect(r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: testNamespace,
		}, roleBinding)).To(Succeed(), "failed to find the rollouts rolebinding")

		aggregateToAdminClusterRole := &rbacv1.ClusterRole{}
		Expect(r.Client.Get(ctx, types.NamespacedName{
			Name: "argo-rollouts-aggregate-to-admin",
		}, aggregateToAdminClusterRole)).To(Succeed(), fmt.Sprintf("failed to find the aggregateToAdmin ClusterRole: %#v\n", err))

		aggregateToEditClusterRole := &rbacv1.ClusterRole{}
		Expect(r.Client.Get(ctx, types.NamespacedName{
			Name: "argo-rollouts-aggregate-to-edit",
		}, aggregateToEditClusterRole)).To(Succeed(), fmt.Sprintf("failed to find the aggregateToEdit ClusterRole: %#v\n", err))

		aggregateToViewClusterRole := &rbacv1.ClusterRole{}
		Expect(r.Client.Get(ctx, types.NamespacedName{
			Name: "argo-rollouts-aggregate-to-view",
		}, aggregateToViewClusterRole)).To(Succeed(), fmt.Sprintf("failed to find the aggregateToView ClusterRole: %#v\n", err))

		service := &corev1.Service{}
		Expect(r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultArgoRolloutsMetricsServiceName,
			Namespace: a.Namespace,
		}, service)).To(Succeed(), fmt.Sprintf("failed to find the rollouts metrics service: %#v\n", err))

		secret := &corev1.Secret{}
		Expect(r.Client.Get(ctx, types.NamespacedName{
			Name:      DefaultRolloutsNotificationSecretName,
			Namespace: a.Namespace,
		}, secret)).To(Succeed(), fmt.Sprintf("failed to find the rollouts secret: %#v\n", err))
	})

	It("ReconcileAggregate to adminClusterRole test", func() {
		Expect(r.reconcileRolloutsAggregateToAdminClusterRole(context.Background(), a)).To(Succeed())
	})

	It("ReconcileAggregate to EditClusterRole test", func() {
		Expect(r.reconcileRolloutsAggregateToEditClusterRole(context.Background(), a)).To(Succeed())
	})

	It("ReconcileAggregate to ViewClusterRole", func() {
		Expect(r.reconcileRolloutsAggregateToViewClusterRole(context.Background(), a)).To(Succeed())
	})

	Context("RolloutManager Cleaup tests", func() {
		a = makeTestRolloutManager()

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
