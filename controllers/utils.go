package rollouts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	UnsupportedRolloutManagerConfiguration = "when there exists a cluster-scoped RolloutManager on the cluster, there may not exist any other RolloutManagers on the cluster: only a single cluster-scoped RolloutManager, or multple namespace-scoped RolloutManagers, are supported, but not both"
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

// checkForExistingRolloutManager will return error if more than one cluster scoped RolloutManagers are created or combination of a cluster and namespace scoped RolloutManagers are created.
// because only one cluster scoped or all namespace scoped RolloutManagers are supported.
func checkForExistingRolloutManager(ctx context.Context, client client.Client, cr *rolloutsmanagerv1alpha1.RolloutManager, log logr.Logger) error {

	rolloutManagerList := rolloutsmanagerv1alpha1.RolloutManagerList{}
	if err := client.List(ctx, &rolloutManagerList); err != nil {
		return fmt.Errorf("failed to get the list of RolloutManager CRs from cluster: %w", err)
	}

	// if there is only one rollout manager, then check if same is being reconciled, if yes then continue the reconciling process
	if len(rolloutManagerList.Items) == 1 && rolloutManagerList.Items[0].Name == cr.Name && rolloutManagerList.Items[0].Namespace == cr.Namespace {
		return nil
	}

	// if there are more than one rollout managers available, then check if any cluster scoped rollout manager exists,
	// if yes then skip reconciliation of this CR, because only one cluster scoped or all namespace scoped rollout managers are supported
	for _, rolloutManager := range rolloutManagerList.Items {

		// if there is a cluster scoped rollout manager then skip reconciliation of this CR and set status to pending.
		if !rolloutManager.Spec.NamespaceScoped {
			cr.Status.Phase = rolloutsmanagerv1alpha1.PhaseFailure
			cr.Status.RolloutController = rolloutsmanagerv1alpha1.PhaseFailure

			if err := client.Status().Update(ctx, cr); err != nil {
				return fmt.Errorf("error updating the RolloutManager CR status: %w", err)
			}
			return fmt.Errorf(UnsupportedRolloutManagerConfiguration)
		}
	}
	// either there are no existing rollout managers or all are namespace scoped, so continue reconciliation of this CR
	return nil
}

func multipleRolloutManagersExist(err error) bool {
	return err.Error() == UnsupportedRolloutManagerConfiguration
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

// updateStatusConditionOfRolloutManager calls SetCondition() with RolloutManager conditions
func updateStatusConditionOfRolloutManager(ctx context.Context, newCondition metav1.Condition, rm *rolloutsmanagerv1alpha1.RolloutManager, k8sClient client.Client, log logr.Logger) error {

	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rm), rm); err != nil {
		log.Error(err, "unable to fetch RolloutManager")
		return nil
	}

	changed, newConditions := insertOrUpdateConditionsInSlice(newCondition, rm.Status.Conditions)

	if changed {
		rm.Status.Conditions = newConditions

		if err := k8sClient.Status().Update(ctx, rm); err != nil {
			log.Error(err, "unable to update RolloutManager status condition")
			return err
		}
	}
	return nil
}

// createCondition returns Condition based on input provided.
// 1. Returns Success condition if no error message is provided, all fields are default.
// 2. If Reason is provided, it returns Failed condition having all default fields except Reason.
// 3. If Message is provided, it returns Failed condition having all default fields except Message.
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
