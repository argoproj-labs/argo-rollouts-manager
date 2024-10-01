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
	UnsupportedRolloutManagerConfiguration          = "when there exists a cluster-scoped RolloutManager on the cluster, there may not exist another: only a single cluster-scoped RolloutManager is supported"
	UnsupportedRolloutManagerClusterScoped          = "when Subscription has environment variable NAMESPACE_SCOPED_ARGO_ROLLOUTS set to True, there may not exist any cluster-scoped RolloutManagers: in this case, only namespace-scoped RolloutManager resources are supported"
	UnsupportedRolloutManagerNamespaceScoped        = "when Subscription has environment variable NAMESPACE_SCOPED_ARGO_ROLLOUTS set to False, there may not exist any namespace-scoped RolloutManagers: only a single cluster-scoped RolloutManager is supported"
	UnsupportedRolloutManagerClusterScopedNamespace = "Namespace is not specified in CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES environment variable of Subscription resource. If you wish to install a cluster-scoped Argo Rollouts instance outside the default namespace, ensure it is defined in CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES"
)

// pluginItem is a clone of PluginItem from "github.com/argoproj/argo-rollouts/utils/plugin/types"
// We clone it here, to avoid a dependency on argo-rollouts.
type pluginItem struct {
	Name     string `json:"name" yaml:"name"`
	Location string `json:"location" yaml:"location"`
	Sha256   string `json:"sha256" yaml:"sha256"`
}

func setRolloutsLabelsAndAnnotationsToObject(obj *metav1.ObjectMeta, cr rolloutsmanagerv1alpha1.RolloutManager) {

	setRolloutsLabelsAndAnnotations(obj)

	setAdditionalRolloutsLabelsAndAnnotationsToObject(obj, cr)
}

func setAdditionalRolloutsLabelsAndAnnotationsToObject(obj *metav1.ObjectMeta, cr rolloutsmanagerv1alpha1.RolloutManager) {

	if cr.Spec.AdditionalMetadata != nil {
		if obj.Labels == nil {
			obj.Labels = map[string]string{}
		}
		if obj.Annotations == nil {
			obj.Annotations = map[string]string{}
		}
		for k, v := range cr.Spec.AdditionalMetadata.Labels {
			obj.Labels[k] = v
		}
		for k, v := range cr.Spec.AdditionalMetadata.Annotations {
			obj.Annotations[k] = v
		}
	}

}

func setRolloutsLabelsAndAnnotations(obj *metav1.ObjectMeta) {
	obj.Labels = map[string]string{}
	obj.Annotations = map[string]string{}
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

// combineStringMaps will combine multiple maps: maps defined earlier in the 'maps' slice may have their values overriden by maps defined later in the 'maps' slice.
func combineStringMaps(maps ...map[string]string) map[string]string {

	if maps == nil {
		return nil
	}

	res := make(map[string]string, 0)
	for idx := range maps {
		currMap := maps[idx]
		if currMap == nil {
			continue
		}
		for k, v := range maps[idx] {
			res[k] = v
		}
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
func validateRolloutsScope(cr rolloutsmanagerv1alpha1.RolloutManager, namespaceScopedArgoRolloutsController bool) (*reconcileStatusResult, error) {

	// If namespace-scoped Rollouts controller is allowed according to Subscription.Spec.Config.Env value
	if namespaceScopedArgoRolloutsController {

		// if RolloutManager being reconciled will create cluster-scoped Rollouts controller, then don't allow it.
		if !cr.Spec.NamespaceScoped {

			phaseFailure := rolloutsmanagerv1alpha1.PhaseFailure

			return &reconcileStatusResult{
				rolloutController: &phaseFailure,
				phase:             &phaseFailure,
			}, fmt.Errorf(UnsupportedRolloutManagerClusterScoped)

		}

		// allow only namespace-scoped Rollouts controller
		return nil, nil

	} else { // If cluster-scoped Rollout controller is allowed according to Subscription.Spec.Config.Env value

		// if RolloutManager being reconciled will create namespace-scoped Rollouts controller, then don't allow it.
		if cr.Spec.NamespaceScoped {

			phaseFailure := rolloutsmanagerv1alpha1.PhaseFailure

			return &reconcileStatusResult{
				rolloutController: &phaseFailure,
				phase:             &phaseFailure,
			}, fmt.Errorf(UnsupportedRolloutManagerNamespaceScoped)
		}

		// if cluster-scoped RolloutManager being reconciled, is not specified in CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES environment variable of Subscription resource,
		// then don't allow it.
		if !allowedClusterScopedNamespace(cr) {

			phaseFailure := rolloutsmanagerv1alpha1.PhaseFailure

			return &reconcileStatusResult{
				rolloutController: &phaseFailure,
				phase:             &phaseFailure,
			}, errors.New(UnsupportedRolloutManagerClusterScopedNamespace)
		}

		// allow cluster-scoped Rollouts for namespaces specified in CLUSTER_SCOPED_ARGO_ROLLOUTS_NAMESPACES environment variable of Subscription resource
		return nil, nil
	}
}

// allowedClusterScopedNamespace will check that current namespace is allowed to host cluster-scoped Argo Rollouts.
func allowedClusterScopedNamespace(cr rolloutsmanagerv1alpha1.RolloutManager) bool {
	clusterConfigNamespaces := splitList(os.Getenv(ClusterScopedArgoRolloutsNamespaces))
	if len(clusterConfigNamespaces) > 0 {
		for _, n := range clusterConfigNamespaces {
			if n == cr.Namespace {
				return true
			}
		}
	}
	return false
}

func splitList(s string) []string {
	elems := strings.Split(s, ",")
	for i := range elems {
		elems[i] = strings.TrimSpace(elems[i])
	}
	return elems
}

// checkForExistingRolloutManager will return error if more than one cluster-scoped RolloutManagers are created.
// because only one cluster-scoped or all namespace-scoped RolloutManagers are supported.
func checkForExistingRolloutManager(ctx context.Context, k8sClient client.Client, cr rolloutsmanagerv1alpha1.RolloutManager) (*reconcileStatusResult, error) {

	// if it is namespace-scoped then return no error
	// because multiple namespace-scoped RolloutManagers are allowed if validateRolloutsScope check is passed earlier.
	if cr.Spec.NamespaceScoped {
		return nil, nil
	}

	// get the list of all RolloutManagers available across all namespaces
	rolloutManagerList := rolloutsmanagerv1alpha1.RolloutManagerList{}
	if err := k8sClient.List(ctx, &rolloutManagerList); err != nil {
		return nil, fmt.Errorf("failed to get the list of RolloutManager CRs from cluster: %w", err)
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

			phaseFailure := rolloutsmanagerv1alpha1.PhaseFailure

			return &reconcileStatusResult{
				rolloutController: &phaseFailure,
				phase:             &phaseFailure,
			}, fmt.Errorf(UnsupportedRolloutManagerConfiguration)

		}
	}

	return nil, nil
}

func multipleRolloutManagersExist(err error) bool {
	return err.Error() == UnsupportedRolloutManagerConfiguration
}

func invalidRolloutScope(err error) bool {
	return err.Error() == UnsupportedRolloutManagerClusterScoped ||
		err.Error() == UnsupportedRolloutManagerNamespaceScoped
}

func invalidRolloutNamespace(err error) bool {
	return err.Error() == UnsupportedRolloutManagerClusterScopedNamespace
}

// updateStatusConditionOfRolloutManager calls Set Condition of RolloutManager status
func updateStatusConditionOfRolloutManager(ctx context.Context, rr reconcileStatusResult, rm *rolloutsmanagerv1alpha1.RolloutManager, k8sClient client.Client, log logr.Logger) error {

	changed, newConditions := insertOrUpdateConditionsInSlice(rr.condition, rm.Status.Conditions)

	if rr.phase != nil && *rr.phase != rm.Status.Phase {
		rm.Status.Phase = *rr.phase
		changed = true
	}

	if rr.rolloutController != nil && *rr.rolloutController != rm.Status.RolloutController {
		rm.Status.RolloutController = *rr.rolloutController
		changed = true
	}

	if changed {
		rm.Status.Conditions = newConditions

		if err := k8sClient.Status().Update(ctx, rm); err != nil {
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

// wrapCondition is a utility function which returns an empty reconcileStatusResult containing only the condition
func wrapCondition(cond metav1.Condition) reconcileStatusResult {
	return reconcileStatusResult{
		condition: cond,
	}
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

// removeUserLabelsAndAnnotations will remove any miscellaneous labels/annotations from obj, that are not used or expected by argo-rollouts-manager. For example, if a user added a label, "my-key": "my-value", to annotations of a Role that is created by our operator, this function would remove that label from 'obj'.
func removeUserLabelsAndAnnotations(obj *metav1.ObjectMeta, cr rolloutsmanagerv1alpha1.RolloutManager) {

	defaultLabelsAndAnnotations := metav1.ObjectMeta{}
	setRolloutsLabelsAndAnnotationsToObject(&defaultLabelsAndAnnotations, cr)

	for objectLabelKey := range obj.Labels {

		existsInDefault := false

		for defaultLabelKey := range defaultLabelsAndAnnotations.Labels {

			if defaultLabelKey == objectLabelKey {
				existsInDefault = true
				break
			}
		}

		if !existsInDefault {
			delete(obj.Labels, objectLabelKey)
		}

	}

	for objectAnnotationKey := range obj.Annotations {

		existsInDefault := false

		for defaultAnnotationKey := range defaultLabelsAndAnnotations.Annotations {

			if defaultAnnotationKey == objectAnnotationKey {
				existsInDefault = true
				break
			}
		}

		if !existsInDefault {
			delete(obj.Annotations, objectAnnotationKey)
		}
	}

}
