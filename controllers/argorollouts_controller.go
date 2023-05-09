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

	rolloutsApi "github.com/iam-veeramalla/argo-rollouts-manager/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that RolloutManagerReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &RolloutManagerReconciler{}

// RolloutManagerReconciler reconciles a RolloutManager object
type RolloutManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var log = logr.Log.WithName("rollouts-controller")

//+kubebuilder:rbac:groups=argoproj.io,resources=rolloutmanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=rolloutmanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=argoproj.io,resources=rolloutmanagers/finalizers,verbs=update
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps;endpoints;events;pods;namespaces;secrets;serviceaccounts;services;services/finalizers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=podtemplates;deployments;replicasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=deployments,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods/eviction,verbs=create
//+kubebuilder:rbac:groups="",resources=podtemplates,verbs=get;list;watch
//+kubebuilder:rbac:groups="appmesh.k8s.aws",resources=virtualnodes;virtualrouters,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="appmesh.k8s.aws",resources=virtualservices,verbs=get;list;watch
//+kubebuilder:rbac:groups="argoproj.io",resources=analysisruns;analysisruns/finalizers;experiments;experiments/finalizers,verbs=create;get;list;watch;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups="argoproj.io",resources=analysistemplates,verbs=create;get;list;watch;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups="argoproj.io",resources=clusteranalysistemplates,verbs=create;get;list;watch;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups="argoproj.io",resources=rollouts;rollouts/finalizers;rollouts/status;rollouts/scale,verbs=create;get;list;watch;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=create;get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;get;update
//+kubebuilder:rbac:groups="elbv2.k8s.aws",resources=targetgroupbindings,verbs=list;get
//+kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=create;get;list;watch;patch
//+kubebuilder:rbac:groups="getambassador.io",resources=ambassadormappings;mappings,verbs=create;watch;get;update;list;delete
//+kubebuilder:rbac:groups="networking.istio.io",resources=destinationrules;virtualservices,verbs=watch;get;update;patch;list
//+kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=create;watch;get;update;patch;list
//+kubebuilder:rbac:groups="split.smi-spec.io",resources=trafficsplits,verbs=create;watch;get;update;patch
//+kubebuilder:rbac:groups="traefik.containo.us",resources=traefikservices,verbs=watch;get;update
//+kubebuilder:rbac:groups="x.getambassador.io",resources=ambassadormappings;mappings,verbs=create;watch;get;update;list;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RolloutManager object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *RolloutManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := logr.FromContext(ctx, "Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling Rollout Managers")

	// Fetch the RolloutManager instance
	rollouts := &rolloutsApi.RolloutManager{}
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
		// Error reconciling RolloutManager sub-resources - requeue the request.
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RolloutManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bld := ctrl.NewControllerManagedBy(mgr)
	setResourceWatches(bld)
	return bld.Complete(r)
}
