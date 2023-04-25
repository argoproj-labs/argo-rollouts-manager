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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ArgoRolloutSpec defines the desired state of ArgoRollout
type ArgoRolloutSpec struct {

	// Env lets you specify environment for Rollouts pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Extra Command arguments that would append to the Rollouts
	// ExtraCommandArgs will not be added, if one of these commands is already part of the Rollouts command
	// with same or different value.
	ExtraCommandArgs []string `json:"extraCommandArgs,omitempty"`

	// Image defines Argo Rollouts controller image (optional)
	Image string `json:"image,omitempty"`

	// NodePlacement defines NodeSelectors and Taints for Rollouts workloads
	NodePlacement *ArgoRolloutsNodePlacementSpec `json:"nodePlacement,omitempty"`

	// Version defines Argo Rollouts controller tag (optional)
	Version string `json:"version,omitempty"`
}

// ArgoRolloutsNodePlacementSpec is used to specify NodeSelector and Tolerations for Rollouts workloads
type ArgoRolloutsNodePlacementSpec struct {
	// NodeSelector is a field of PodSpec, it is a map of key value pairs used for node selection
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations allow the pods to schedule onto nodes with matching taints
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// ArgoRolloutStatus defines the observed state of ArgoRollout
type ArgoRolloutStatus struct {
	// RolloutController is a simple, high-level summary of where the RolloutController component is in its lifecycle.
	// There are three possible RolloutController values:
	// Pending: The RolloutController component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the required Pods for the RolloutController component are in a Ready state.
	// Unknown: The state of the RolloutController component could not be obtained.
	RolloutController string `json:"rolloutController,omitempty"`
	// Phase is a simple, high-level summary of where the ArgoRollout is in its lifecycle.
	// There are three possible phase values:
	// Pending: The ArgoRollout has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Available: All of the resources for the ArgoRollout are ready.
	// Unknown: The state of the ArgoRollout phase could not be obtained.
	Phase string `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ArgoRollout is the Schema for the argorollouts API
type ArgoRollout struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArgoRolloutSpec   `json:"spec,omitempty"`
	Status ArgoRolloutStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ArgoRolloutList contains a list of ArgoRollouts
type ArgoRolloutList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArgoRollout `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ArgoRollout{}, &ArgoRolloutList{})
}
