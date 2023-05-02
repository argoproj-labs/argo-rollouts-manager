package rollouts

import (
	"context"
	"fmt"
	"os"
	"reflect"

	rolloutsApi "github.com/iam-veeramalla/argo-rollouts-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconcile the Rollouts controller deployment.
func (r *RolloutManagerReconciler) reconcileRolloutsDeployment(cr *rolloutsApi.RolloutManager, sa *corev1.ServiceAccount) error {
	// Configuration for the desired deployment
	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabels(&desiredDeployment.ObjectMeta)

	desiredDeployment.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName,
				},
			},
			Spec: corev1.PodSpec{
				NodeSelector: defaultNodeSelector(),
			},
		},
	}

	if cr.Spec.NodePlacement != nil {
		desiredDeployment.Spec.Template.Spec.NodeSelector = appendStringMap(
			desiredDeployment.Spec.Template.Spec.NodeSelector, cr.Spec.NodePlacement.NodeSelector)
		desiredDeployment.Spec.Template.Spec.Tolerations = cr.Spec.NodePlacement.Tolerations
	}

	desiredPodSpec := &desiredDeployment.Spec.Template.Spec

	runAsNonRoot := true
	desiredPodSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}

	desiredPodSpec.ServiceAccountName = sa.ObjectMeta.Name

	desiredPodSpec.Containers = []corev1.Container{
		rolloutsContainer(cr),
	}

	// If the deployment for rollouts does not exist, create one.
	actualDeployment := &appsv1.Deployment{}
	if err := fetchObject(r.Client, cr.Namespace, DefaultArgoRolloutsResourceName, actualDeployment); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the deployment %s : %s", DefaultArgoRolloutsResourceName, err)
		}

		if err := controllerutil.SetControllerReference(cr, desiredDeployment, r.Scheme); err != nil {
			return err
		}
		log.Info(fmt.Sprintf("Creating deployment %s", DefaultArgoRolloutsResourceName))
		return r.Client.Create(context.TODO(), desiredDeployment)
	}

	actualPodSpec := actualDeployment.Spec.Template.Spec

	// If the Deployment already exists, make sure the values we care about are correct.
	deploymentsDifferent := !reflect.DeepEqual(actualPodSpec.Containers[0], desiredPodSpec.Containers) ||
		actualPodSpec.ServiceAccountName != desiredPodSpec.ServiceAccountName ||
		!reflect.DeepEqual(actualDeployment.Labels, desiredDeployment.Labels) ||
		!reflect.DeepEqual(actualDeployment.Spec.Template.Labels, desiredDeployment.Spec.Template.Labels) ||
		!reflect.DeepEqual(actualDeployment.Spec.Selector, desiredDeployment.Spec.Selector) ||
		!reflect.DeepEqual(actualDeployment.Spec.Template.Spec.NodeSelector, desiredDeployment.Spec.Template.Spec.NodeSelector) ||
		!reflect.DeepEqual(actualDeployment.Spec.Template.Spec.Tolerations, desiredDeployment.Spec.Template.Spec.Tolerations) ||
		!reflect.DeepEqual(actualPodSpec.SecurityContext, desiredPodSpec.SecurityContext)

	if deploymentsDifferent {
		actualDeployment.Spec.Template.Spec.Containers = desiredPodSpec.Containers
		actualDeployment.Spec.Template.Spec.ServiceAccountName = desiredPodSpec.ServiceAccountName
		actualDeployment.Labels = desiredDeployment.Labels
		actualDeployment.Spec.Template.Labels = desiredDeployment.Spec.Template.Labels
		actualDeployment.Spec.Selector = desiredDeployment.Spec.Selector
		actualDeployment.Spec.Template.Spec.NodeSelector = desiredDeployment.Spec.Template.Spec.NodeSelector
		actualDeployment.Spec.Template.Spec.Tolerations = desiredDeployment.Spec.Template.Spec.Tolerations
		actualDeployment.Spec.Template.Spec.SecurityContext = desiredPodSpec.SecurityContext
		return r.Client.Update(context.TODO(), actualDeployment)
	}
	return nil
}

func rolloutsContainer(cr *rolloutsApi.RolloutManager) corev1.Container {

	// Global proxy env vars go firstArgoRollouts
	rolloutsEnv := cr.Spec.Env

	// Environment specified in the CR take precedence over everything else
	rolloutsEnv = envMerge(rolloutsEnv, proxyEnvVars(), false)

	return corev1.Container{
		Args:            getRolloutsCommandArgs(cr),
		Env:             rolloutsEnv,
		Image:           getRolloutsContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			FailureThreshold: 3,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString("healthz"),
				},
			},
			InitialDelaySeconds: int32(30),
			PeriodSeconds:       int32(20),
			SuccessThreshold:    int32(1),
			TimeoutSeconds:      int32(10),
		},
		Name: "argo-rollouts",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8080,
				Name:          "healthz",
			},
			{
				ContainerPort: 8090,
				Name:          "metrics",
			},
		},
		ReadinessProbe: &corev1.Probe{
			FailureThreshold: int32(5),
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/metrics",
					Port: intstr.FromString("metrics"),
				},
			},
			InitialDelaySeconds: int32(10),
			PeriodSeconds:       int32(5),
			SuccessThreshold:    int32(1),
			TimeoutSeconds:      int32(4),
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(true),
			RunAsNonRoot:             boolPtr(true),
		},
	}
}

// boolPtr returns a pointer to val
func boolPtr(val bool) *bool {
	return &val
}

// Returns the container image for rollouts controller.
func getRolloutsContainerImage(cr *rolloutsApi.RolloutManager) string {
	defaultImg, defaultTag := false, false

	img := cr.Spec.Image
	tag := cr.Spec.Version

	// If spec is empty, use the defaults
	if img == "" {
		img = DefaultArgoRolloutsImage
		defaultImg = true
	}
	if tag == "" {
		tag = DefaultArgoRolloutsVersion
		defaultTag = true
	}

	// If an env var is specified then use that, but don't override the spec values (if they are present)
	if e := os.Getenv(ArgoRolloutsImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return combineImageTag(img, tag)
}

// getRolloutsCommand will return the command for the Rollouts controller component.
func getRolloutsCommandArgs(cr *rolloutsApi.RolloutManager) []string {
	args := make([]string, 0)

	args = append(args, "--namespaced")

	extraArgs := cr.Spec.ExtraCommandArgs
	err := isMergable(extraArgs, args)
	if err != nil {
		return args
	}

	args = append(args, extraArgs...)
	return args
}
