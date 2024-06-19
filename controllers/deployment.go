package rollouts

import (
	"context"
	"fmt"
	"os"
	"reflect"

	rolloutsmanagerv1alpha1 "github.com/argoproj-labs/argo-rollouts-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func generateDesiredRolloutsDeployment(cr rolloutsmanagerv1alpha1.RolloutManager, sa corev1.ServiceAccount) appsv1.Deployment {

	// NOTE: When updating this function, ensure that normalizeDeployment is updated as well. See that function for details.

	// Configuration for the desired deployment
	desiredDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultArgoRolloutsResourceName,
			Namespace: cr.Namespace,
		},
	}
	setRolloutsLabelsAndAnnotationsToObject(&desiredDeployment.ObjectMeta, &cr)

	// Add labels and annotations as well to the pod template
	labels := map[string]string{
		DefaultRolloutsSelectorKey: DefaultArgoRolloutsResourceName,
	}
	annotations := map[string]string{}
	if cr.Spec.AdditionalMetadata != nil {
		for k, v := range cr.Spec.AdditionalMetadata.Labels {
			labels[k] = v
		}
		for k, v := range cr.Spec.AdditionalMetadata.Annotations {
			annotations[k] = v
		}
	}

	desiredDeployment.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: corev1.PodSpec{
				NodeSelector: map[string]string{
					"kubernetes.io/os": "linux",
				},
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
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

	desiredPodSpec.Volumes = []corev1.Volume{
		{
			Name: "plugin-bin",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	return desiredDeployment
}

// Reconcile the Rollouts controller deployment.
func (r *RolloutManagerReconciler) reconcileRolloutsDeployment(ctx context.Context, cr rolloutsmanagerv1alpha1.RolloutManager, sa corev1.ServiceAccount) error {

	desiredDeployment := generateDesiredRolloutsDeployment(cr, sa)

	normalizedDesiredDeployment, err := normalizeDeployment(desiredDeployment)
	if err != nil {
		// If you see this warning in the logs, verify that normalizedDeployment is fully consistent with generateDesiredRolloutsDeployment. See normalizeDeployment for details.
		log.Error(fmt.Errorf("unexpected fail on normalizing generated rollouts Deployment"), "", "err", err)
		// We intentionally continue without returning, as the error is non-fatal at runtime
	}

	if !reflect.DeepEqual(normalizedDesiredDeployment, desiredDeployment) { // sanity test to verify that generateDesiredRolloutsDeployments and normalizeDeployment are consistent.
		// If you see this warning in the logs, verify that normalizedDeployment is fully consistent with generateDesiredRolloutsDeployment. See normalizeDeployment for details.
		deploymentsDifferent := identifyDeploymentDifference(normalizedDesiredDeployment, desiredDeployment)
		log.Error(fmt.Errorf("normalized form of desired Deployment was not equal: %v", deploymentsDifferent), "")
		// We intentionally continue without returning, as the error is non-fatal at runtime
	}

	// If the deployment for rollouts does not exist, create one.
	actualDeployment := &appsv1.Deployment{}

	if err := fetchObject(ctx, r.Client, cr.Namespace, DefaultArgoRolloutsResourceName, actualDeployment); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the Deployment %s: %w", DefaultArgoRolloutsResourceName, err)
		}

		if err := controllerutil.SetControllerReference(&cr, &desiredDeployment, r.Scheme); err != nil {
			return err
		}
		log.Info(fmt.Sprintf("Creating Deployment %s", DefaultArgoRolloutsResourceName))
		return r.Client.Create(ctx, &desiredDeployment)
	}

	normalizedActualDeployment, err := normalizeDeployment(*actualDeployment)

	if err != nil || !reflect.DeepEqual(normalizedActualDeployment, normalizedDesiredDeployment) {

		deploymentsDifferent := identifyDeploymentDifference(normalizedActualDeployment, normalizedDesiredDeployment)

		log.Info("updating Deployment due to detected difference: " + deploymentsDifferent)

		if deploymentsDifferent == "" {
			log.Error(fmt.Errorf("warning: a difference was detected by DeepEqual, but not by identifyDeploymentDifference"), "")
			// this error is a warning, only. Continue.
		}

		actualDeployment.Spec.Strategy = desiredDeployment.Spec.Strategy
		actualDeployment.Spec.Template.Spec.Containers = desiredDeployment.Spec.Template.Spec.Containers
		actualDeployment.Spec.Template.Spec.ServiceAccountName = desiredDeployment.Spec.Template.Spec.ServiceAccountName
		actualDeployment.Labels = desiredDeployment.Labels
		actualDeployment.Spec.Template.Labels = desiredDeployment.Spec.Template.Labels
		actualDeployment.Spec.Template.Annotations = desiredDeployment.Spec.Template.Annotations
		actualDeployment.Spec.Selector = desiredDeployment.Spec.Selector
		actualDeployment.Spec.Template.Spec.NodeSelector = desiredDeployment.Spec.Template.Spec.NodeSelector
		actualDeployment.Spec.Template.Spec.Tolerations = desiredDeployment.Spec.Template.Spec.Tolerations
		actualDeployment.Spec.Template.Spec.SecurityContext = desiredDeployment.Spec.Template.Spec.SecurityContext
		actualDeployment.Spec.Template.Spec.Volumes = desiredDeployment.Spec.Template.Spec.Volumes
		return r.Client.Update(ctx, actualDeployment)
	}
	return nil
}

// identifyDeploymentDifference is a simple comparison of the contents of two deployments, returning "" if they are the same, otherwise returning the name of the field that changed.
func identifyDeploymentDifference(x appsv1.Deployment, y appsv1.Deployment) string {

	xPodSpec := x.Spec.Template.Spec
	yPodSpec := y.Spec.Template.Spec

	if !reflect.DeepEqual(xPodSpec.Containers, yPodSpec.Containers) {
		return "Spec.Template.Spec.Containers"
	}

	if xPodSpec.ServiceAccountName != yPodSpec.ServiceAccountName {
		return "ServiceAccountName"
	}

	if !reflect.DeepEqual(x.Spec.Strategy, y.Spec.Strategy) {
		return ".Spec.Strategy"
	}

	if !reflect.DeepEqual(x.Annotations, y.Annotations) {
		return "Annotations"
	}

	if !reflect.DeepEqual(x.Labels, y.Labels) {
		return "Labels"
	}

	if !reflect.DeepEqual(x.Spec.Template.Labels, y.Spec.Template.Labels) {
		return ".Spec.Template.Labels"
	}

	if !reflect.DeepEqual(x.Spec.Template.Annotations, y.Spec.Template.Annotations) {
		return ".Spec.Template.Annotations"
	}

	if !reflect.DeepEqual(x.Spec.Selector, y.Spec.Selector) {
		return ".Spec.Selector"
	}

	if !reflect.DeepEqual(x.Spec.Template.Spec.NodeSelector, y.Spec.Template.Spec.NodeSelector) {
		return "Spec.Template.Spec.NodeSelector"
	}

	if !reflect.DeepEqual(x.Spec.Template.Spec.Tolerations, y.Spec.Template.Spec.Tolerations) {
		return "Spec.Template.Spec.Tolerations"
	}

	if !reflect.DeepEqual(xPodSpec.SecurityContext, yPodSpec.SecurityContext) {
		return "Spec.Template.Spec.SecurityContext"
	}

	if !reflect.DeepEqual(x.Spec.Template.Spec.Volumes, y.Spec.Template.Spec.Volumes) {
		return "Spec.Template.Spec.Volumes"
	}

	return ""
}

func rolloutsContainer(cr rolloutsmanagerv1alpha1.RolloutManager) corev1.Container {

	// NOTE: When updating this function, ensure that normalizeDeployment is updated as well. See that function for details.

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
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
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
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/home/argo-rollouts/plugin-bin",
				Name:      "plugin-bin",
			},
			{
				MountPath: "/tmp",
				Name:      "tmp",
			},
		},
	}

}

// One of the goals of an operator is to reconcile the live state of a resource on the cluster, with a target state for that resource. However, one of the challenges in doing so is that some fields of the resource will naturally differ from the values that are generated: for example, some field have default values which are only set after creation. This can make it challenging to compare the live/target status. Various strategies exist to handle.
//
// The strategy used in this file is implemented here in normalizeDeployment: normalizeDeployment will created a normalized representation of any input Deployment: the normal form will only contains fields which are relevant/useful to the operator. All other fields will be ingorewd.
//
// You can then use:
//
//	reflect.DeepEqual( normalizeDeployment( /* desired deployment */), normalizeDeployment( /* actual deployment from k8s*/))
//
// To determine if the actual deployment from k8s needs to be updated.
//
// NOTE: When updating 'generateDesiredRolloutsDeployment', ensure this function is updated as well.
// - Specifically, in generateDesiredRolloutsDeployment, if a new field of Deployment is modified, it should be added to the copy logic here.
func normalizeDeployment(inputParam appsv1.Deployment) (appsv1.Deployment, error) {

	input := inputParam.DeepCopy()

	res := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        input.ObjectMeta.Name,
			Namespace:   input.ObjectMeta.Namespace,
			Labels:      input.ObjectMeta.Labels,
			Annotations: input.ObjectMeta.Annotations,
		},
	}

	if input.Spec.Selector == nil {
		return appsv1.Deployment{}, fmt.Errorf("missing .spec.selector")
	}

	inputSpecSecurityContext := input.Spec.Template.Spec.SecurityContext

	if inputSpecSecurityContext == nil {
		return appsv1.Deployment{}, fmt.Errorf("missing .spec.template.spec.securityContext")
	}

	inputSpecVolumes := input.Spec.Template.Spec.Volumes
	if inputSpecVolumes == nil || len(inputSpecVolumes) != 2 {
		return appsv1.Deployment{}, fmt.Errorf("missing .spec.template.spec.volumes")
	}

	res.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: input.Spec.Selector.MatchLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      input.Spec.Template.Labels,
				Annotations: input.Spec.Template.Annotations,
			},
			Spec: corev1.PodSpec{
				NodeSelector:       input.Spec.Template.Spec.NodeSelector,
				Tolerations:        input.Spec.Template.Spec.Tolerations,
				ServiceAccountName: input.Spec.Template.Spec.ServiceAccountName,
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: input.Spec.Template.Spec.SecurityContext.RunAsNonRoot,
				},
				Volumes: []corev1.Volume{inputSpecVolumes[0], inputSpecVolumes[1]},
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: input.Spec.Strategy.Type,
			// we ignore the default values set in RollingUpdate:
		},
	}

	if len(input.Spec.Template.Spec.Containers) != 1 {
		return appsv1.Deployment{}, fmt.Errorf("incorrect number of .spec.template.spec.containers")
	}

	inputContainer := input.Spec.Template.Spec.Containers[0]
	inputLivenessProbe := inputContainer.LivenessProbe
	inputPorts := inputContainer.Ports
	inputReadinessProbe := inputContainer.ReadinessProbe
	inputSecurityContext := inputContainer.SecurityContext
	inputVolumeMounts := inputContainer.VolumeMounts

	if inputLivenessProbe == nil {
		return appsv1.Deployment{}, fmt.Errorf("incorrect liveness probe")
	}

	if inputLivenessProbe.ProbeHandler.HTTPGet == nil {
		return appsv1.Deployment{}, fmt.Errorf("incorrect http get in liveness probe")
	}

	if inputReadinessProbe == nil {
		return appsv1.Deployment{}, fmt.Errorf("incorrect readiness probe")
	}

	if inputReadinessProbe.ProbeHandler.HTTPGet == nil {
		return appsv1.Deployment{}, fmt.Errorf("incorrect http get in readiness probe")
	}

	if inputPorts == nil || len(inputPorts) != 2 {
		return appsv1.Deployment{}, fmt.Errorf("incorrect input ports")
	}

	if inputSecurityContext == nil || inputSecurityContext.Capabilities == nil {
		return appsv1.Deployment{}, fmt.Errorf("incorrect security context")
	}

	if inputVolumeMounts == nil || len(inputVolumeMounts) != 2 {
		return appsv1.Deployment{}, fmt.Errorf("incorrect volume mounts")
	}

	// Nil string slices need to be converted to empty string slices, because  reflect.DeepEqual(nil, []string{}) is false, despite being functionally the same, here.
	if len(inputContainer.Args) == 0 {
		inputContainer.Args = make([]string, 0)
	}

	if len(inputContainer.Env) == 0 {
		inputContainer.Env = make([]corev1.EnvVar, 0)
	}

	res.Spec.Template.Spec.Containers = []corev1.Container{{
		Args:            inputContainer.Args,
		Env:             inputContainer.Env,
		Image:           inputContainer.Image,
		ImagePullPolicy: inputContainer.ImagePullPolicy,
		LivenessProbe: &corev1.Probe{
			FailureThreshold: inputLivenessProbe.FailureThreshold,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: inputLivenessProbe.ProbeHandler.HTTPGet.Path,
					Port: inputLivenessProbe.ProbeHandler.HTTPGet.Port,
				},
			},
			InitialDelaySeconds: inputLivenessProbe.InitialDelaySeconds,
			PeriodSeconds:       inputLivenessProbe.PeriodSeconds,
			SuccessThreshold:    inputLivenessProbe.SuccessThreshold,
			TimeoutSeconds:      inputLivenessProbe.TimeoutSeconds,
		},
		Name: inputContainer.Name,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: inputPorts[0].ContainerPort,
				Name:          inputPorts[0].Name,
			},
			{
				ContainerPort: inputPorts[1].ContainerPort,
				Name:          inputPorts[1].Name,
			},
		},
		ReadinessProbe: &corev1.Probe{
			FailureThreshold: inputReadinessProbe.FailureThreshold,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: inputReadinessProbe.ProbeHandler.HTTPGet.Path,
					Port: inputReadinessProbe.ProbeHandler.HTTPGet.Port,
				},
			},
			InitialDelaySeconds: inputReadinessProbe.InitialDelaySeconds,
			PeriodSeconds:       inputReadinessProbe.PeriodSeconds,
			SuccessThreshold:    inputReadinessProbe.SuccessThreshold,
			TimeoutSeconds:      inputReadinessProbe.TimeoutSeconds,
		},
		Resources: inputContainer.Resources,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: inputSecurityContext.Capabilities.Drop,
			},
			AllowPrivilegeEscalation: inputSecurityContext.AllowPrivilegeEscalation,
			ReadOnlyRootFilesystem:   inputSecurityContext.ReadOnlyRootFilesystem,
			RunAsNonRoot:             inputSecurityContext.RunAsNonRoot,
			SeccompProfile:           inputSecurityContext.SeccompProfile,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      inputVolumeMounts[0].Name,
				MountPath: inputVolumeMounts[0].MountPath,
			},
			{
				Name:      inputVolumeMounts[1].Name,
				MountPath: inputVolumeMounts[1].MountPath,
			},
		},
	}}

	return res, nil

}

// boolPtr returns a pointer to val
func boolPtr(val bool) *bool {
	return &val
}

// Returns the container image for rollouts controller.
func getRolloutsContainerImage(cr rolloutsmanagerv1alpha1.RolloutManager) string {
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
func getRolloutsCommandArgs(cr rolloutsmanagerv1alpha1.RolloutManager) []string {
	args := make([]string, 0)

	if cr.Spec.NamespaceScoped {
		args = append(args, "--namespaced")
	}

	extraArgs := cr.Spec.ExtraCommandArgs
	err := isMergable(extraArgs, args)
	if err != nil {
		return args
	}

	args = append(args, extraArgs...)
	return args
}
