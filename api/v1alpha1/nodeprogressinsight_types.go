/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeStatusInsightConditionType are types of conditions that can be reported on Node status insights
type NodeStatusInsightConditionType string

//goland:noinspection GoCommentStart
const (
	// Updating condition communicates whether the Node is updating
	NodeStatusInsightUpdating NodeStatusInsightConditionType = "Updating"
	// Degraded condition communicates whether the Node is degraded (problem observed)
	NodeStatusInsightDegraded NodeStatusInsightConditionType = "Degraded"
	// Available condition communicates whether the Node is available (accepting workloads)
	NodeStatusInsightAvailable NodeStatusInsightConditionType = "Available"
)

// NodeUpdatingReason are well-known reasons for the Updating condition on Node status insights
type NodeUpdatingReason string

//goland:noinspection GoCommentStart
const (
	// Draining is used with Updating=True when the Node is being drained
	NodeDraining NodeUpdatingReason = "Draining"
	// Updating is used with Updating=True when new node configuration is being applied
	NodeUpdating NodeUpdatingReason = "Updating"
	// Rebooting is used with Updating=True when the Node is rebooting into the new version
	NodeRebooting NodeUpdatingReason = "Rebooting"

	// Updated is used with Updating=False when the Node is prevented by configuration from updating
	NodePaused NodeUpdatingReason = "Paused"
	// Updated is used with Updating=False when the Node is waiting to be eventually updated
	NodeUpdatePending NodeUpdatingReason = "Pending"
	// Updated is used with Updating=False when the Node has been updated
	NodeCompleted NodeUpdatingReason = "Completed"

	// CannotDetermine is used with Updating=Unknown
	NodeCannotDetermine NodeUpdatingReason = "CannotDetermine"
)

// NodeProgressInsightSpec is empty for now, NodeProgressInsightSpec is purely status-reporting API. In the future spec may be used
// to hold configuration to drive what information is surfaced and how
type NodeProgressInsightSpec struct {
}

// NodeProgressInsightStatus reports the state of a Node during the update
type NodeProgressInsightStatus struct {
	// conditions provides details about the control plane update. Known conditions are:
	// - Updating: whether the Node is updating; When Updating=False, the reason field can be Updated, Pending, or Paused. When Updating=True, the reason field can be Draining, Updating, or Rebooting
	// - Available: whether the Node is available (accepting workloads)
	// - Degraded: whether the Node is degraded (problem observed)
	// +listType=map
	// +listMapKey=type
	// +optional
	// +kubebuilder:validation:MaxItems=5
	// +TODO: Add validations to enforce all known conditions are present (CEL+MinItems), once conditions stabilize
	// +TODO: Add validations to enforce that only known Reasons are used in conditions, once conditions stabilize
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// name is the name of the node
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// poolResource is the resource that represents the pool the node is a member of
	// +required
	// +kubebuilder:validation:XValidation:rule="self.group == 'machineconfiguration.openshift.io' && self.resource == 'machineconfigpools'",message="resource must be a machineconfigpools.machineconfiguration.openshift.io resource"
	PoolResource ResourceRef `json:"poolResource"`

	// scopeType describes whether the node belongs to control plane or a worker pool
	// +required
	Scope ScopeType `json:"scopeType"`

	// version is the OCP semantic version the Node is currently running, when known. This field abstracts the internal
	// cross-resource relations where OCP version is just one property of the MachineConfig that the Node happens to be
	// reconciled to by the Machine Config Operator, because it matches the selectors on the MachineConfigPool resource
	// tied to the MachineConfig. It should be considered and used as an inferred value, mostly suitable to be displayed
	// in the UIs. It is not guaranteed to be present for all Nodes.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`^((?:0|[1-9]\d*)[.](?:0|[1-9]\d*)[.](?:0|[1-9]\d*)(?:-(?:(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:[.](?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?)$`
	Version string `json:"version,omitempty"`

	// estimatedToComplete is the estimated time to complete the update, when known
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=duration
	EstimatedToComplete *metav1.Duration `json:"estimatedToComplete,omitempty"`

	// message is a short human-readable message about the node update status. It must be shorter than 100 characters.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MaxLength=100
	Message string `json:"message,omitempty"`
}

// NodeProgressInsight reports the state of a Node during the update
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=nodeprogressinsights,scope=Cluster
// +kubebuilder:metadata:annotations="description=Reports the state of a Node during the update"
// +kubebuilder:metadata:annotations="displayName=NodeProgressInsights"
// +kubebuilder:validation:XValidation:rule="!has(self.status) || self.status.name == self.metadata.name",message="When status is present, .status must match .metadata.name"
type NodeProgressInsight struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is standard Kubernetes object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is empty for now, NodeProgressInsight is purely status-reporting API. In the future spec may be used to hold
	// configuration to drive what information is surfaced and how
	// +required
	Spec NodeProgressInsightSpec `json:"spec"`
	// status exposes the health and status of the ongoing cluster update
	// +optional
	Status NodeProgressInsightStatus `json:"status"`
}

// +kubebuilder:object:root=true

// NodeProgressInsightList contains a list of NodeProgressInsight.
type NodeProgressInsightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeProgressInsight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeProgressInsight{}, &NodeProgressInsightList{})
}
