package rollouts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func setRolloutsLabels(obj *metav1.ObjectMeta) {
	obj.Labels = map[string]string{}
	obj.Labels["app.kubernetes.io/name"] = DefaultArgoRolloutsResourceName
	obj.Labels["app.kubernetes.io/part-of"] = DefaultArgoRolloutsResourceName
	obj.Labels["app.kubernetes.io/component"] = DefaultArgoRolloutsResourceName
}

func setRolloutsAggregatedClusterRoleLabels(obj *metav1.ObjectMeta, name string) {

	obj.Labels = map[string]string{}
	obj.Labels["app.kubernetes.io/component"] = "aggregate-cluster-role"
	obj.Labels["app.kubernetes.io/name"] = name
	obj.Labels["app.kubernetes.io/part-of"] = DefaultArgoRolloutsResourceName
	obj.Labels["rbac.authorization.k8s.io/aggregate-to-admin"] = "true"
}

// fetchObject will retrieve the object with the given namespace and name using the Kubernetes API.
// The result will be stored in the given object.
func fetchObject(client client.Client, namespace string, name string, obj client.Object) error {
	return client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
}

// getPolicyRules returns the policy rules for Argo Rollouts Role.
func getPolicyRules() []rbacv1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/status",
				"rollouts/finalizers",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"analysisruns",
				"analysisruns/finalizers",
				"experiments",
				"experiments/finalizers",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"analysistemplates",
				"clusteranalysistemplates",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"replicasets",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
				"apps",
			},
			Resources: []string{
				"deployments",
				"podtemplates",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"services",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"patch",
				"create",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"coordination.k8s.io",
			},
			Resources: []string{
				"leases",
			},
			Verbs: []string{
				"create",
				"get",
				"update",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"list",
				"update",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods/eviction",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"networking.k8s.io",
				"extensions",
			},
			Resources: []string{
				"ingresses",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"networking.istio.io",
			},
			Resources: []string{
				"virtualservices",
				"destinationrules",
			},
			Verbs: []string{
				"watch",
				"get",
				"update",
				"patch",
				"list",
			},
		},
		{
			APIGroups: []string{
				"split.smi-spec.io",
			},
			Resources: []string{
				"trafficsplits",
			},
			Verbs: []string{
				"create",
				"watch",
				"get",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"getambassador.io",
				"x.getambassador.io",
			},
			Resources: []string{
				"mappings",
				"ambassadormappings",
			},
			Verbs: []string{
				"create",
				"watch",
				"get",
				"update",
				"list",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"endpoints",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"elbv2.k8s.aws",
			},
			Resources: []string{
				"targetgroupbindings",
			},
			Verbs: []string{
				"list",
				"get",
			},
		},
		{
			APIGroups: []string{
				"appmesh.k8s.aws",
			},
			Resources: []string{
				"virtualservices",
			},
			Verbs: []string{
				"watch",
				"get",
				"list",
			},
		},
		{
			APIGroups: []string{
				"appmesh.k8s.aws",
			},
			Resources: []string{
				"virtualnodes",
				"virtualrouters",
			},
			Verbs: []string{
				"watch",
				"get",
				"list",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"traefik.containo.us",
			},
			Resources: []string{
				"traefikservices",
			},
			Verbs: []string{
				"watch",
				"get",
				"update",
			},
		},
	}
}

// Returns PolicyRules for the Cluster Role argo-rollouts-aggregate-to-admin
func getAggregateToAdminPolicyRules() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/scale",
				"rollouts/status",
				"experiments",
				"analysistemplates",
				"clusteranalysistemplates",
				"analysisruns",
			},
			Verbs: []string{
				"create",
				"delete",
				"deletecollection",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
	}
}

// Returns PolicyRules for the Cluster Role argo-rollouts-aggregate-to-edit
func getAggregateToEditPolicyRules() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/scale",
				"rollouts/status",
				"experiments",
				"analysistemplates",
				"clusteranalysistemplates",
				"analysisruns",
			},
			Verbs: []string{
				"create",
				"delete",
				"deletecollection",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
	}
}

// Returns PolicyRules for the Cluster Role argo-rollouts-aggregate-to-view
func getAggregateToViewPolicyRules() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/scale",
				"experiments",
				"analysistemplates",
				"clusteranalysistemplates",
				"analysisruns",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
}

// Returns the defult nodeSelector for Argo Rollouts workloads
func defaultNodeSelector() map[string]string {
	return map[string]string{
		"kubernetes.io/os": "linux",
	}
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
