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

// MachineConfigPoolProgressInsightSpec is empty for now, MachineConfigPoolProgressInsight is purely status-reporting API. In the future spec may be used
// to hold configuration to drive what information is surfaced and how
type MachineConfigPoolProgressInsightSpec struct {
}

// MachineConfigPoolProgressInsightConditionType are types of conditions that can be reported on MachineConfigPool progress insights
type MachineConfigPoolProgressInsightConditionType string

//goland:noinspection GoCommentStart,GoUnusedConst
const (
	// Updating condition communicates whether the MachineConfigPool is updating
	MachineConfigPoolProgressInsightUpdating MachineConfigPoolProgressInsightConditionType = "Updating"
	MachineConfigPoolProgressInsightHealthy  MachineConfigPoolProgressInsightConditionType = "Healthy"
)

// MachineConfigPoolUpdatingReason are well-known reasons for the Updating condition on MachineConfigPool progress insights
type MachineConfigPoolUpdatingReason string

//goland:noinspection GoCommentStart,GoUnusedConst
const (
	// Updated is used with Updating=False when all nodes in MachineConfigPool completed updating
	MachineConfigPoolUpdatingReasonUpdated MachineConfigPoolUpdatingReason = "Updated"
	// Pending is used with Updating=False when MachineConfigPool is not updating yet but is expected to start updating eventually
	MachineConfigPoolUpdatingReasonPending MachineConfigPoolUpdatingReason = "Pending"
	// Paused is used with Updating=False when some nodes are running outdated versions but the MCP is paused
	MachineConfigPoolUpdatingReasonPaused MachineConfigPoolUpdatingReason = "Paused"
	// Progressing is used with Updating=True when the ClusterOperator is updating
	MachineConfigPoolUpdatingReasonProgressing MachineConfigPoolUpdatingReason = "Progressing"
	// CannotDetermine is used with Updating=Unknown
	MachineConfigPoolUpdatingCannotDetermine MachineConfigPoolUpdatingReason = "CannotDetermine"
)

// PoolAssessment is a brief summary assessment of the pool update process. This value is human-oriented, and while it
// looks like a state/phase enum, it is not meant to be used as such. Assessment is meant as human-oriented brief
// summary matching the state expressed in conditions (taking into account various relations between them, like
// ordering or precedence), intended to be directly used in UIs and reports. For machine-oriented conditional behavior
// depending on the state, the conditions should be used instead.
type PoolAssessment string

//goland:noinspection ALL
const (
	// Pending means the nodes in the pool will be updated but none have even started yet
	PoolPending PoolAssessment = "Pending"
	// Completed means all nodes in the pool have been updated
	PoolCompleted PoolAssessment = "Completed"
	// Degraded means the process of updating the pool suffers from an observed problem
	PoolDegraded PoolAssessment = "Degraded"
	// Excluded means some (or all) nodes in the pool would be normally updated but a configuration (such as paused MCP)
	// prevents that from happening
	PoolExcluded PoolAssessment = "Excluded"
	// Progressing means the nodes in the pool are being updated and no problems or slowness are detected
	PoolProgressing PoolAssessment = "Progressing"
)

// NodeSummaryType are types of summaries (how many nodes match certain criteria, such as updated, degraded, etc.)
// reported for a node pool
// +kubebuilder:validation:Enum=Total;Available;Progressing;Outdated;Draining;Excluded;Degraded
type NodeSummaryType string

//goland:noinspection ALL
const (
	// Total is the total number of nodes in the pool
	NodesTotal NodeSummaryType = "Total"
	// Available is the number of nodes in the pool that are available (accepting workloads)
	NodesAvailable NodeSummaryType = "Available"
	// Progressing is the number of nodes in the pool that are updating
	NodesProgressing NodeSummaryType = "Progressing"
	// Outdated is the number of nodes in the pool that are running an outdated version
	NodesOutdated NodeSummaryType = "Outdated"
	// Draining is the number of nodes in the pool that are being drained
	NodesDraining NodeSummaryType = "Draining"
	// Excluded is the number of nodes in the pool that would normally be updated but configuration (such as paused MCP)
	// prevents that from happening
	NodesExcluded NodeSummaryType = "Excluded"
	// Degraded is the number of nodes in the pool that are degraded
	NodesDegraded NodeSummaryType = "Degraded"
)

// NodeSummary is a count of nodes matching certain criteria (e.g. updated, degraded, etc.)
type NodeSummary struct {
	// type is the type of the summary. Valid values are: Total, Available, Progressing, Outdated, Draining, Excluded, Degraded
	// The summaries are not exclusive, a single node may be counted in multiple summaries.
	// +required
	Type NodeSummaryType `json:"type"`

	// count is the number of nodes matching the criteria, between 0 and 2000
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2000
	Count int32 `json:"count"`
}

// MachineConfigPoolProgressInsightStatus reports the state of a MachineConfigPool resource (which represents a pool of nodes
// update in standalone clusters), during a cluster update.
type MachineConfigPoolProgressInsightStatus struct {
	// conditions provide details about the machine config pool update. It contains at most 5 items. Known conditions are:
	// - Updating: whether the pool is updating; When Updating=False, the reason field can be Pending, Updated or Excluded
	// +listType=map
	// +listMapKey=type
	// +optional
	// +kubebuilder:validation:MaxItems=5
	// +TODO: Add validations to enforce all known conditions are present (CEL+MinItems), once conditions stabilize
	// +TODO: Add validations to enforce that only known Reasons are used in conditions, once conditions stabilize
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// name is the name of the machine config pool
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	Name string `json:"name"`

	// scopeType describes whether the pool is a control plane or a worker pool
	// +required
	Scope ScopeType `json:"scopeType"`

	// assessment is a brief summary assessment of the pool update process. This value is human-oriented, and while it
	// looks like a state/phase enum, it is not meant to be used as such. Assessment is meant as human-oriented brief
	// summary matching the state expressed in conditions (taking into account various relations between them, like
	// ordering or precedence), intended to be directly used in UIs and reports. For machine-oriented conditional behavior
	// depending on the state, the conditions should be used instead.
	//
	// The known values are: Pending, Completed, Degraded, Excluded, Progressing. The API is not restricted to these
	// values, and valid values can be even brief phrases, up to 64 characters long.
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Assessment PoolAssessment `json:"assessment"`

	// completionPercent is a percentage of the pool update completion (0-100)
	// +required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Completion int32 `json:"completionPercent"`

	// summaries is a list of counts of nodes matching certain criteria (e.g. updated, degraded, etc.). Maximum 7 items can be listed.
	// +listType=map
	// +listMapKey=type
	// +optional
	// +kubebuilder:validation:MaxItems=7
	Summaries []NodeSummary `json:"summaries,omitempty"`
}

// MachineConfigPoolProgressInsight provides summary information about an ongoing node pool update in Standalone clusters
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=machineconfigpoolprogressinsights,scope=Cluster
// +kubebuilder:metadata:annotations="description=Provides summary information about an ongoing node pool update in Standalone clusters"
// +kubebuilder:metadata:annotations="displayName=MachineConfigPoolProgressInsights"
// +kubebuilder:validation:XValidation:rule="!has(self.status) || self.status.name == self.metadata.name",message="When status is present, .status.name must match .metadata.name"
type MachineConfigPoolProgressInsight struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is standard Kubernetes object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is empty for now, MachineConfigPoolProgressInsight is purely status-reporting API. In the future spec may be used to hold
	// configuration to drive what information is surfaced and how
	// +required
	Spec MachineConfigPoolProgressInsightSpec `json:"spec"`
	// status exposes the health and status of the ongoing cluster update
	// +optional
	Status MachineConfigPoolProgressInsightStatus `json:"status"`
}

// +kubebuilder:object:root=true

// MachineConfigPoolProgressInsightList is a list of MachineConfigPoolProgressInsight resources
type MachineConfigPoolProgressInsightList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is standard Kubernetes object metadata
	// +optional
	metav1.ListMeta `json:"metadata"`

	// items is a list of MachineConfigPoolProgressInsight resources
	// +optional
	// +kubebuilder:validation:MaxItems=1024
	Items []MachineConfigPoolProgressInsight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MachineConfigPoolProgressInsight{}, &MachineConfigPoolProgressInsightList{})
}
