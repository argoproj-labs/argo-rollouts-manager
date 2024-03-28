package rollouts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	UnsupportedRolloutManagerConfiguration   = "when there exists a cluster-scoped RolloutManager on the cluster, there may not exist another: only a single cluster-scoped RolloutManager is supported."
	UnsupportedRolloutManagerClusterScoped   = "when Subscription having environmet variable NAMESPACE_SCOPED_ARGO_ROLLOUTS set to True, there may not exist any cluster-scoped RolloutManager: only a single namespace-scoped RolloutManager is supported in a namespace."
	UnsupportedRolloutManagerNamespaceScoped = "when Subscription having environmet variable NAMESPACE_SCOPED_ARGO_ROLLOUTS set to False, there may not exist any namespace-scoped RolloutManager: only cluster-scoped RolloutManager is supported."
)

func setRolloutsLabels(obj *metav1.ObjectMeta) {
	obj.Labels = map[string]string{}
	obj.Labels["app.kubernetes.io/name"] = DefaultArgoRolloutsResourceName
	obj.Labels["app.kubernetes.io/part-of"] = DefaultArgoRolloutsResourceName
	obj.Labels["app.kubernetes.io/component"] = DefaultArgoRolloutsResourceName
}

// fetchObject will retrieve the object with the given namespace and name using the Kubernetes API.
// The result will be stored in the given object.
func fetchObject(ctx context.Context, client client.Client, namespace string, name string, obj client.Object) error {
	if namespace == "" {
		return client.Get(ctx, types.NamespacedName{Name: name}, obj)
	}
	return client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
}

// Appends the map `add` to the given map `src` and return the result.
func appendStringMap(src map[string]string, add map[string]string) map[string]string {
	res := src
	if len(src) <= 0 {
		res = make(map[string]string, len(add))
	}
	for key, val := range add {
		res[key] = val
	}
	return res
}

// Merges two slices of EnvVar entries into a single one. If existing
// has an EnvVar with same Name attribute as one in merge, the EnvVar is not
// merged unless override is set to true.
func envMerge(existing []corev1.EnvVar, merge []corev1.EnvVar, override bool) []corev1.EnvVar {
	ret := []corev1.EnvVar{}
	final := map[string]corev1.EnvVar{}
	for _, e := range existing {
		final[e.Name] = e
	}
	for _, m := range merge {
		if _, ok := final[m.Name]; ok {
			if override {
				final[m.Name] = m
			}
		} else {
			final[m.Name] = m
		}
	}

	for _, v := range final {
		ret = append(ret, v)
	}

	// sort result slice by env name
	sort.SliceStable(ret,
		func(i, j int) bool {
			return ret[i].Name < ret[j].Name
		})

	return ret
}

func caseInsensitiveGetenv(s string) (string, string) {
	if v := os.Getenv(s); v != "" {
		return s, v
	}
	ls := strings.ToLower(s)
	if v := os.Getenv(ls); v != "" {
		return ls, v
	}
	return "", ""
}

func proxyEnvVars(vars ...corev1.EnvVar) []corev1.EnvVar {
	result := []corev1.EnvVar{}
	result = append(result, vars...)
	proxyKeys := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}
	for _, p := range proxyKeys {
		if k, v := caseInsensitiveGetenv(p); k != "" {
			result = append(result, corev1.EnvVar{Name: k, Value: v})
		}
	}
	return result
}

// Returns the combined image and tag in the proper format for tags and digests.
// If the provided tag is a SHA Digest, return the combinedImageTag in format `image@SHA:245344..`.
// Whereas if the provided tag is a version, return the combinedImageTag in format `image:vx.y.z`.
func combineImageTag(img string, tag string) string {
	if strings.Contains(tag, ":") {
		return fmt.Sprintf("%s@%s", img, tag)
	} else if len(tag) > 0 {
		return fmt.Sprintf("%s:%s", img, tag)
	}
	// No tag provided, use the default
	return img
}

// contains returns true if a string is part of the given slice.
func contains(s []string, g string) bool {
	for _, a := range s {
		if a == g {
			return true
		}
	}
	return false
}

// isMergable returns error if any of the extraArgs is already part of the default command Arguments.
func isMergable(extraArgs []string, cmd []string) error {
	if len(extraArgs) > 0 {
		for _, arg := range extraArgs {
			if len(arg) > 2 && arg[:2] == "--" {
				if ok := contains(cmd, arg); ok {
					err := errors.New("duplicate argument error")
					log.Error(err, fmt.Sprintf("Arg %s is already part of the default command arguments", arg))
					return err
				}
			}
		}
	}
	return nil
}

// validateRolloutsScope will check scope of Rollouts controller configured in RolloutManager and scope allowed by Admin (Configured in Subscription.Spec.Config.Env)
func validateRolloutsScope(ctx context.Context, client client.Client, cr *rolloutsmanagerv1alpha1.RolloutManager, namespaceScopedArgoRolloutsController bool) error {

	// If namespace-scoped Rollouts controller is allowed according to Subscription.Spec.Config.Env value
	if namespaceScopedArgoRolloutsController {

		// if RolloutManager being reconciled will create cluster-scoped Rollouts controller, then don't allow it.
		if !cr.Spec.NamespaceScoped {

			if err := UpdateDeploymentStatusWithFunction(ctx, client, cr, func(cr *rolloutsmanagerv1alpha1.RolloutManager) {

				cr.Status.Phase = rolloutsmanagerv1alpha1.PhaseFailure
				cr.Status.RolloutController = rolloutsmanagerv1alpha1.PhaseFailure
			}); err != nil {
				return fmt.Errorf("error updating the RolloutManager CR status: %w", err)
			}

			return fmt.Errorf(UnsupportedRolloutManagerClusterScoped)
		}

		// allow only namespace-scoped Rollouts controller
		return nil

	} else { // If cluster-scoped Rollout controller is allowed according to Subscription.Spec.Config.Env value

		// if RolloutManager being reconciled will create namespace-scoped Rollouts controller, then don't allow it.
		if cr.Spec.NamespaceScoped {

			if err := UpdateDeploymentStatusWithFunction(ctx, client, cr, func(cr *rolloutsmanagerv1alpha1.RolloutManager) {

				cr.Status.Phase = rolloutsmanagerv1alpha1.PhaseFailure
				cr.Status.RolloutController = rolloutsmanagerv1alpha1.PhaseFailure
			}); err != nil {
				return fmt.Errorf("error updating the RolloutManager CR status: %w", err)
			}

			return fmt.Errorf(UnsupportedRolloutManagerNamespaceScoped)
		}

		// allow only cluster-scoped RolloutManager
		return nil
	}
}

// checkForExistingRolloutManager will return error if more than one cluster-scoped RolloutManagers are created.
// because only one cluster-scoped or all namespace-scoped RolloutManagers are supported.
func checkForExistingRolloutManager(ctx context.Context, client client.Client, cr *rolloutsmanagerv1alpha1.RolloutManager) error {

	// if it is namespace-scoped then return no error
	// because multiple namespace-scoped RolloutManagers are allowed if validateRolloutsScope check is passed earlier.
	if cr.Spec.NamespaceScoped {
		return nil
	}

	// get the list of all RolloutManagers available across all namespaces
	rolloutManagerList := rolloutsmanagerv1alpha1.RolloutManagerList{}
	if err := client.List(ctx, &rolloutManagerList); err != nil {
		return fmt.Errorf("failed to get the list of RolloutManager CRs from cluster: %w", err)
	}

	// if there is only one RolloutsManager, then check if same is being reconciled, if yes then continue the reconciling process
	if len(rolloutManagerList.Items) == 1 && rolloutManagerList.Items[0].Name == cr.Name && rolloutManagerList.Items[0].Namespace == cr.Namespace {
		return nil
	}

	// if there are more than one RolloutManagers available, then check if any cluster-scoped RolloutManager exists,
	// if yes then return error for this CR, because only one cluster-scoped RolloutManagers is supported
	for _, rolloutManager := range rolloutManagerList.Items {

		// if current RolloutManager is being iterated, then skip it, because we are looking for other cluster-scoped RolloutManagers.
		if rolloutManager.Name == cr.Name && rolloutManager.Namespace == cr.Namespace {
			continue
		}

		// if there is a another cluster-scoped RolloutManager available in cluster then skip reconciliation of this one and set status to failure.
		if !rolloutManager.Spec.NamespaceScoped {
			if err := UpdateDeploymentStatusWithFunction(ctx, client, cr, func(cr *rolloutsmanagerv1alpha1.RolloutManager) {

				cr.Status.Phase = rolloutsmanagerv1alpha1.PhaseFailure
				cr.Status.RolloutController = rolloutsmanagerv1alpha1.PhaseFailure
			}); err != nil {
				return fmt.Errorf("error updating the RolloutManager CR status: %w", err)
			}

			return fmt.Errorf(UnsupportedRolloutManagerConfiguration)
		}
	}
	// either there are no existing RolloutManagers or all are namespace-scoped or only one cluster-scoped RolloutManagers exists,
	// so continue reconciliation of this CR
	return nil
}

func multipleRolloutManagersExist(err error) bool {
	return err.Error() == UnsupportedRolloutManagerConfiguration
}

func invalidRolloutScope(err error) bool {
	return err.Error() == UnsupportedRolloutManagerClusterScoped ||
		err.Error() == UnsupportedRolloutManagerNamespaceScoped
}

// updateStatusConditionOfRolloutManager calls Set Condition of RolloutManager status
func updateStatusConditionOfRolloutManager(ctx context.Context, newCondition metav1.Condition, rm *rolloutsmanagerv1alpha1.RolloutManager, k8sClient client.Client, log logr.Logger) error {

	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rm), rm); err != nil {
		log.Error(err, "unable to fetch RolloutManager")
		return nil
	}

	changed, newConditions := insertOrUpdateConditionsInSlice(newCondition, rm.Status.Conditions)

	if changed {
		err := UpdateDeploymentStatusWithFunction(ctx, k8sClient, rm, func(rm *rolloutsmanagerv1alpha1.RolloutManager) {
			rm.Status.Conditions = newConditions
		})

		if err != nil {
			log.Error(err, "unable to update RolloutManager status condition")
			return err
		}
	}
	return nil
}

// insertOrUpdateConditionsInSlice is a generic function for inserting/updating metav1.Condition into a slice of []metav1.Condition
func insertOrUpdateConditionsInSlice(newCondition metav1.Condition, existingConditions []metav1.Condition) (bool, []metav1.Condition) {

	// Check if condition with same type is already set, if Yes then check if content is same,
	// If content is not same update LastTransitionTime

	index := -1
	for i, Condition := range existingConditions {
		if Condition.Type == newCondition.Type {
			index = i
			break
		}
	}

	now := metav1.Now()

	changed := false

	if index == -1 {
		newCondition.LastTransitionTime = now
		existingConditions = append(existingConditions, newCondition)
		changed = true

	} else if existingConditions[index].Message != newCondition.Message ||
		existingConditions[index].Reason != newCondition.Reason ||
		existingConditions[index].Status != newCondition.Status {

		newCondition.LastTransitionTime = now
		existingConditions[index] = newCondition
		changed = true
	}

	return changed, existingConditions

}

func UpdateDeploymentStatusWithFunction(ctx context.Context, k8sClient client.Client, rm *rolloutsmanagerv1alpha1.RolloutManager,
	mutationFn func(*rolloutsmanagerv1alpha1.RolloutManager)) error {

	return untilSuccess(k8sClient, func(k8sClient client.Client) error {

		// Retrieve the latest version of the RolloutManager resource
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rm), rm)
		if err != nil {
			return err
		}

		// Call the mutation function, to change the RolloutManager
		mutationFn(rm)

		// Attempt to update the object with the change made by the mutation function
		err = k8sClient.Status().Update(ctx, rm)

		// Report back the error, if we hit one
		return err
	})

}

// createCondition returns Condition based on input provided.
// 1. Returns Success condition if no error message is provided, all fields are default.
// 2. If more than 1 reasons are there then its an internal error.
// 3. If 1 Reason is provided, it returns Failed condition having all default fields except Reason.
// 4. If Message is provided, it returns Failed condition having all default fields except Message.
func createCondition(message string, reason ...string) metav1.Condition {

	if message == "" {
		return metav1.Condition{
			Type:    rolloutsmanagerv1alpha1.RolloutManagerConditionType,
			Reason:  rolloutsmanagerv1alpha1.RolloutManagerReasonSuccess,
			Message: "",
			Status:  metav1.ConditionTrue,
		}
	}

	if len(reason) > 0 {

		if len(reason) > 1 { // Only 0 or 1 reasons are supported.
			return metav1.Condition{
				Type:    rolloutsmanagerv1alpha1.RolloutManagerConditionType,
				Reason:  rolloutsmanagerv1alpha1.RolloutManagerReasonErrorOccurred,
				Message: "An internal error occurred",
				Status:  metav1.ConditionTrue,
			}
		}

		return metav1.Condition{
			Type:    rolloutsmanagerv1alpha1.RolloutManagerConditionType,
			Reason:  reason[0],
			Message: message,
			Status:  metav1.ConditionFalse,
		}
	}

	return metav1.Condition{
		Type:    rolloutsmanagerv1alpha1.RolloutManagerConditionType,
		Reason:  rolloutsmanagerv1alpha1.RolloutManagerReasonErrorOccurred,
		Message: message,
		Status:  metav1.ConditionFalse,
	}
}

// UntilSuccess will keep trying a K8s operation until it succeeds, or times out.
func untilSuccess(k8sClient client.Client, f func(k8sClient client.Client) error) error {

	err := wait.PollImmediate(time.Second*1, time.Minute*2, func() (done bool, err error) {
		funcError := f(k8sClient)
		return funcError == nil, nil
	})

	return err
}
