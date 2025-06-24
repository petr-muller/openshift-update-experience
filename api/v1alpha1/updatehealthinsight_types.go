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

// UpdateHealthInsightSpec is empty for now, UpdateHealthInsightSpec is purely status-reporting API. In the future spec may be used
// to hold configuration to drive what information is surfaced and how
type UpdateHealthInsightSpec struct {
}

// InsightScope is a list of resources involved in the insight
type InsightScope struct {
	// type is either ControlPlane or WorkerPool
	// +required
	Type ScopeType `json:"type"`

	// resources is a list of resources involved in the insight, of any group/kind. Maximum 16 resources can be listed.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=128
	Resources []ResourceRef `json:"resources,omitempty"`
}

// InsightImpactLevel describes the severity of the impact the reported condition has on the cluster or update
// +kubebuilder:validation:Enum=Unknown;Info;Warning;Error;Critical
type InsightImpactLevel string

//goland:noinspection GoCommentStart
const (
	// UnknownImpactLevel is used when the impact level is not known
	UnknownImpactLevel InsightImpactLevel = "Unknown"
	// info should be used for insights that are strictly informational or even positive (things go well or
	// something recently healed)
	InfoImpactLevel InsightImpactLevel = "Info"
	// warning should be used for insights that explain a minor or transient problem. Anything that requires
	// admin attention or manual action should not be a warning but at least an error.
	WarningImpactLevel InsightImpactLevel = "Warning"
	// error should be used for insights that inform about a problem that requires admin attention. Insights of
	// level error and higher should be as actionable as possible, and should be accompanied by links to documentation,
	// KB articles or other resources that help the admin to resolve the problem.
	ErrorImpactLevel InsightImpactLevel = "Error"
	// critical should be used rarely, for insights that inform about a severe problem, threatening with data
	// loss, destroyed cluster or other catastrophic consequences. Insights of this level should be accompanied by
	// links to documentation, KB articles or other resources that help the admin to resolve the problem, or at least
	// prevent the severe consequences from happening.
	CriticalInfoLevel InsightImpactLevel = "Critical"
)

// InsightImpactType describes the type of the impact the reported condition has on the cluster or update
// +kubebuilder:validation:Enum=None;Unknown;API Availability;Cluster Capacity;Application Availability;Application Outage;Data Loss;Update Speed;Update Stalled
type InsightImpactType string

const (
	NoneImpactType                    InsightImpactType = "None"
	UnknownImpactType                 InsightImpactType = "Unknown"
	ApiAvailabilityImpactType         InsightImpactType = "API Availability"
	ClusterCapacityImpactType         InsightImpactType = "Cluster Capacity"
	ApplicationAvailabilityImpactType InsightImpactType = "Application Availability"
	ApplicationOutageImpactType       InsightImpactType = "Application Outage"
	DataLossImpactType                InsightImpactType = "Data Loss"
	UpdateSpeedImpactType             InsightImpactType = "Update Speed"
	UpdateStalledImpactType           InsightImpactType = "Update Stalled"
)

// InsightImpact describes the impact the reported condition has on the cluster or update
type InsightImpact struct {
	// level is the severity of the impact. Valid values are Unknown, Info, Warning, Error, Critical.
	// +required
	Level InsightImpactLevel `json:"level"`

	// type is the type of the impact. Valid values are None, Unknown, API Availability, Cluster Capacity,
	// Application Availability, Application Outage, Data Loss, Update Speed, Update Stalled.
	// +required
	Type InsightImpactType `json:"type"`

	// summary is a short summary of the impact. It must not be empty and must be shorter than 256 characters.
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:MinLength=1
	Summary string `json:"summary"`

	// description is a human-oriented, possibly longer-form description of the condition reported by the insight It must
	// be shorter than 4096 characters.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MaxLength=4096
	Description string `json:"description,omitempty"`
}

// InsightRemediation contains information about how to resolve or prevent the reported condition
type InsightRemediation struct {
	// reference is a URL where administrators can find information to resolve or prevent the reported condition
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:XValidation:rule="isURL(self)",message="reference must a valid URL"
	Reference string `json:"reference"`

	// estimatedFinish is the estimated time when the informer expects the condition to be resolved, if applicable.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time
	EstimatedFinish *metav1.Time `json:"estimatedFinish,omitempty"`
}

// UpdateHealthInsightStatus reports a piece of actionable information produced by an insight producer about the health
// of the cluster in the context of an update
type UpdateHealthInsightStatus struct {
	// startedAt is the time when the condition reported by the insight started
	// +required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time
	StartedAt metav1.Time `json:"startedAt"`

	// scope is list of objects involved in the insight
	// +required
	Scope InsightScope `json:"scope"`

	// impact describes the impact the reported condition has on the cluster or update
	// +required
	Impact InsightImpact `json:"impact"`

	// remediation contains information about how to resolve or prevent the reported condition
	// +required
	Remediation InsightRemediation `json:"remediation"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UpdateHealthInsight is a piece of actionable information produced by an insight producer about the health
// of the cluster in the context of an update
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=updatehealthinsights,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2012
// +openshift:file-pattern=cvoRunLevel=0000_00,operatorName=cluster-version-operator,operatorOrdering=02
// +openshift:enable:FeatureGate=UpgradeStatus
// +kubebuilder:metadata:annotations="description=Reports a piece of actionable information about the health of the cluster in the context of an update"
// +kubebuilder:metadata:annotations="displayName=UpdateHealthInsights"
// UpdateHealthInsight is a piece of actionable information produced by an insight producer about the health
// of the cluster in the context of an update
type UpdateHealthInsight struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is standard Kubernetes object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is empty for now, UpdateHealthInsight is purely status-reporting API. In the future spec may be used to hold
	// configuration to drive what information is surfaced and how
	// +required
	Spec UpdateHealthInsightSpec `json:"spec"`
	// status reports a piece of actionable information produced by an insight producer about the health
	// of the cluster in the context of an update
	// +optional
	Status UpdateHealthInsightStatus `json:"status"`
}

// +kubebuilder:object:root=true

// UpdateHealthInsightList contains a list of UpdateHealthInsight.
type UpdateHealthInsightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UpdateHealthInsight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UpdateHealthInsight{}, &UpdateHealthInsightList{})
}
