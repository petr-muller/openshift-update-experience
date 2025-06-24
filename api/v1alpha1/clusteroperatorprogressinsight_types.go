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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterOperatorProgressInsightSpec is empty for now, ClusterOperatorProgressInsightSpec is purely status-reporting API. In the future spec may be used
// to hold configuration to drive what information is surfaced and how
type ClusterOperatorProgressInsightSpec struct {
}

// ClusterOperatorProgressInsightConditionType are types of conditions that can be reported on ClusterOperator status insights
type ClusterOperatorProgressInsightConditionType string

//goland:noinspection GoCommentStart
const (
	// Updating condition communicates whether the ClusterOperator is updating
	ClusterOperatorProgressInsightUpdating ClusterOperatorProgressInsightConditionType = "Updating"
	// Healthy condition communicates whether the ClusterOperator is considered healthy
	ClusterOperatorProgressInsightHealthy ClusterOperatorProgressInsightConditionType = "Healthy"
)

// ClusterOperatorUpdatingReason are well-known reasons for the Updating condition on ClusterOperator status insights
type ClusterOperatorUpdatingReason string

//goland:noinspection GoCommentStart
const (
	// Updated is used with Updating=False when the ClusterOperator finished updating
	ClusterOperatorUpdatingReasonUpdated ClusterOperatorUpdatingReason = "Updated"
	// Pending is used with Updating=False when the ClusterOperator is not updating and is still running previous version
	ClusterOperatorUpdatingReasonPending ClusterOperatorUpdatingReason = "Pending"
	// Progressing is used with Updating=True when the ClusterOperator is updating
	ClusterOperatorUpdatingReasonProgressing ClusterOperatorUpdatingReason = "Progressing"
	// CannotDetermine is used with Updating=Unknown
	ClusterOperatorUpdatingCannotDetermine ClusterOperatorUpdatingReason = "CannotDetermine"
)

// ClusterOperatorHealthyReason are well-known reasons for the Healthy condition on ClusterOperator status insights
type ClusterOperatorHealthyReason string

//goland:noinspection GoCommentStart
const (
	// AsExpected is used with Healthy=True when no issues are observed
	ClusterOperatorHealthyReasonAsExpected ClusterOperatorHealthyReason = "AsExpected"
	// Unavailable is used with Healthy=False when the ClusterOperator has Available=False condition
	ClusterOperatorHealthyReasonUnavailable ClusterOperatorHealthyReason = "Unavailable"
	// Degraded is used with Healthy=False when the ClusterOperator has Degraded=True condition
	ClusterOperatorHealthyReasonDegraded ClusterOperatorHealthyReason = "Degraded"
	// CannotDetermine is used with Healthy=Unknown
	ClusterOperatorHealthyReasonCannotDetermine ClusterOperatorHealthyReason = "CannotDetermine"
)

// ClusterOperatorProgressInsightStatus reports the state of a ClusterOperator resource (which represents a control plane
// component update in standalone clusters), during the update
type ClusterOperatorProgressInsightStatus struct {
	// conditions provide details about the operator. It contains at most 10 items. Known conditions are:
	// - Updating: whether the operator is updating; When Updating=False, the reason field can be Pending or Updated
	// - Healthy: whether the operator is considered healthy; When Healthy=False, the reason field can be Unavailable or Degraded, and Unavailable is "stronger" than Degraded
	// +listType=map
	// +listMapKey=type
	// +optional
	// +kubebuilder:validation:MaxItems=5
	// +TODO: Add validations to enforce all known conditions are present (CEL+MinItems), once conditions stabilize
	// +TODO: Add validations to enforce that only known Reasons are used in conditions, once conditions stabilize
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// name is the name of the operator, equal to the name of the corresponding clusteroperators.config.openshift.io resource
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	Name string `json:"name"`
}

// ClusterOperatorProgressInsight reports the state of a Cluster Operator (an individual control plane component) during an update
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=clusteroperatorprogressinsights,scope=Cluster
// +kubebuilder:metadata:annotations="description=Provides information about a Cluster Operator update"
// +kubebuilder:metadata:annotations="displayName=ClusterOperatorProgressInsights"
// +kubebuilder:validation:XValidation:rule="!has(self.status) || self.status.name == self.metadata.name",message="When status is present, .status must match .metadata.name"
type ClusterOperatorProgressInsight struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is standard Kubernetes object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is empty for now, ClusterOperatorProgressInsight is purely status-reporting API. In the future spec may be used to hold
	// configuration to drive what information is surfaced and how
	// +required
	Spec ClusterOperatorProgressInsightSpec `json:"spec"`
	// status exposes the health and status of the ongoing cluster operator update
	// +optional
	Status ClusterOperatorProgressInsightStatus `json:"status"`
}

// +kubebuilder:object:root=true

// ClusterOperatorProgressInsightList contains a list of ClusterOperatorProgressInsight.
type ClusterOperatorProgressInsightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterOperatorProgressInsight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterOperatorProgressInsight{}, &ClusterOperatorProgressInsightList{})
}
