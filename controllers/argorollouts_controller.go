/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rollouts

import (
	"context"

	rolloutsApi "github.com/iam-veeramalla/argo-rollouts-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that ArgoRolloutsReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &ArgoRolloutsReconciler{}

// ArgoRolloutsReconciler reconciles a ArgoRollouts object
type ArgoRolloutsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var log = logr.Log.WithName("rollouts-controller")

//+kubebuilder:rbac:groups=argoproj.io,resources=argorollouts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=argorollouts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=argoproj.io,resources=argorollouts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ArgoRollouts object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ArgoRolloutsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := logr.FromContext(ctx, "Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling ArgoRollouts")

	// Fetch the ArgoRollouts instance
	rollouts := &rolloutsApi.ArgoRollout{}
	err := r.Client.Get(ctx, req.NamespacedName, rollouts)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if err := r.reconcileRolloutsController(rollouts); err != nil {
		// Error reconciling ArgoCDExport sub-resources - requeue the request.
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ArgoRolloutsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bld := ctrl.NewControllerManagedBy(mgr)
	setResourceWatches(bld)
	return bld.Complete(r)
}
