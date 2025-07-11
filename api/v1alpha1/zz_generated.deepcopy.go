//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterOperatorProgressInsight) DeepCopyInto(out *ClusterOperatorProgressInsight) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterOperatorProgressInsight.
func (in *ClusterOperatorProgressInsight) DeepCopy() *ClusterOperatorProgressInsight {
	if in == nil {
		return nil
	}
	out := new(ClusterOperatorProgressInsight)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterOperatorProgressInsight) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterOperatorProgressInsightList) DeepCopyInto(out *ClusterOperatorProgressInsightList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterOperatorProgressInsight, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterOperatorProgressInsightList.
func (in *ClusterOperatorProgressInsightList) DeepCopy() *ClusterOperatorProgressInsightList {
	if in == nil {
		return nil
	}
	out := new(ClusterOperatorProgressInsightList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterOperatorProgressInsightList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterOperatorProgressInsightSpec) DeepCopyInto(out *ClusterOperatorProgressInsightSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterOperatorProgressInsightSpec.
func (in *ClusterOperatorProgressInsightSpec) DeepCopy() *ClusterOperatorProgressInsightSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterOperatorProgressInsightSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterOperatorProgressInsightStatus) DeepCopyInto(out *ClusterOperatorProgressInsightStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterOperatorProgressInsightStatus.
func (in *ClusterOperatorProgressInsightStatus) DeepCopy() *ClusterOperatorProgressInsightStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterOperatorProgressInsightStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVersionProgressInsight) DeepCopyInto(out *ClusterVersionProgressInsight) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVersionProgressInsight.
func (in *ClusterVersionProgressInsight) DeepCopy() *ClusterVersionProgressInsight {
	if in == nil {
		return nil
	}
	out := new(ClusterVersionProgressInsight)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterVersionProgressInsight) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVersionProgressInsightList) DeepCopyInto(out *ClusterVersionProgressInsightList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterVersionProgressInsight, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVersionProgressInsightList.
func (in *ClusterVersionProgressInsightList) DeepCopy() *ClusterVersionProgressInsightList {
	if in == nil {
		return nil
	}
	out := new(ClusterVersionProgressInsightList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterVersionProgressInsightList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVersionProgressInsightSpec) DeepCopyInto(out *ClusterVersionProgressInsightSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVersionProgressInsightSpec.
func (in *ClusterVersionProgressInsightSpec) DeepCopy() *ClusterVersionProgressInsightSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterVersionProgressInsightSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVersionProgressInsightStatus) DeepCopyInto(out *ClusterVersionProgressInsightStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Versions.DeepCopyInto(&out.Versions)
	in.StartedAt.DeepCopyInto(&out.StartedAt)
	if in.CompletedAt != nil {
		in, out := &in.CompletedAt, &out.CompletedAt
		*out = (*in).DeepCopy()
	}
	if in.EstimatedCompletedAt != nil {
		in, out := &in.EstimatedCompletedAt, &out.EstimatedCompletedAt
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVersionProgressInsightStatus.
func (in *ClusterVersionProgressInsightStatus) DeepCopy() *ClusterVersionProgressInsightStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterVersionProgressInsightStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlPlaneUpdateVersions) DeepCopyInto(out *ControlPlaneUpdateVersions) {
	*out = *in
	if in.Previous != nil {
		in, out := &in.Previous, &out.Previous
		*out = new(Version)
		(*in).DeepCopyInto(*out)
	}
	in.Target.DeepCopyInto(&out.Target)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlPlaneUpdateVersions.
func (in *ControlPlaneUpdateVersions) DeepCopy() *ControlPlaneUpdateVersions {
	if in == nil {
		return nil
	}
	out := new(ControlPlaneUpdateVersions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InsightImpact) DeepCopyInto(out *InsightImpact) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InsightImpact.
func (in *InsightImpact) DeepCopy() *InsightImpact {
	if in == nil {
		return nil
	}
	out := new(InsightImpact)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InsightRemediation) DeepCopyInto(out *InsightRemediation) {
	*out = *in
	if in.EstimatedFinish != nil {
		in, out := &in.EstimatedFinish, &out.EstimatedFinish
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InsightRemediation.
func (in *InsightRemediation) DeepCopy() *InsightRemediation {
	if in == nil {
		return nil
	}
	out := new(InsightRemediation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InsightScope) DeepCopyInto(out *InsightScope) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]ResourceRef, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InsightScope.
func (in *InsightScope) DeepCopy() *InsightScope {
	if in == nil {
		return nil
	}
	out := new(InsightScope)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeProgressInsight) DeepCopyInto(out *NodeProgressInsight) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeProgressInsight.
func (in *NodeProgressInsight) DeepCopy() *NodeProgressInsight {
	if in == nil {
		return nil
	}
	out := new(NodeProgressInsight)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeProgressInsight) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeProgressInsightList) DeepCopyInto(out *NodeProgressInsightList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NodeProgressInsight, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeProgressInsightList.
func (in *NodeProgressInsightList) DeepCopy() *NodeProgressInsightList {
	if in == nil {
		return nil
	}
	out := new(NodeProgressInsightList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeProgressInsightList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeProgressInsightSpec) DeepCopyInto(out *NodeProgressInsightSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeProgressInsightSpec.
func (in *NodeProgressInsightSpec) DeepCopy() *NodeProgressInsightSpec {
	if in == nil {
		return nil
	}
	out := new(NodeProgressInsightSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeProgressInsightStatus) DeepCopyInto(out *NodeProgressInsightStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.PoolResource = in.PoolResource
	if in.EstimatedToComplete != nil {
		in, out := &in.EstimatedToComplete, &out.EstimatedToComplete
		*out = new(v1.Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeProgressInsightStatus.
func (in *NodeProgressInsightStatus) DeepCopy() *NodeProgressInsightStatus {
	if in == nil {
		return nil
	}
	out := new(NodeProgressInsightStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceRef) DeepCopyInto(out *ResourceRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceRef.
func (in *ResourceRef) DeepCopy() *ResourceRef {
	if in == nil {
		return nil
	}
	out := new(ResourceRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpdateHealthInsight) DeepCopyInto(out *UpdateHealthInsight) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpdateHealthInsight.
func (in *UpdateHealthInsight) DeepCopy() *UpdateHealthInsight {
	if in == nil {
		return nil
	}
	out := new(UpdateHealthInsight)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UpdateHealthInsight) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpdateHealthInsightList) DeepCopyInto(out *UpdateHealthInsightList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]UpdateHealthInsight, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpdateHealthInsightList.
func (in *UpdateHealthInsightList) DeepCopy() *UpdateHealthInsightList {
	if in == nil {
		return nil
	}
	out := new(UpdateHealthInsightList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UpdateHealthInsightList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpdateHealthInsightSpec) DeepCopyInto(out *UpdateHealthInsightSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpdateHealthInsightSpec.
func (in *UpdateHealthInsightSpec) DeepCopy() *UpdateHealthInsightSpec {
	if in == nil {
		return nil
	}
	out := new(UpdateHealthInsightSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpdateHealthInsightStatus) DeepCopyInto(out *UpdateHealthInsightStatus) {
	*out = *in
	in.StartedAt.DeepCopyInto(&out.StartedAt)
	in.Scope.DeepCopyInto(&out.Scope)
	out.Impact = in.Impact
	in.Remediation.DeepCopyInto(&out.Remediation)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpdateHealthInsightStatus.
func (in *UpdateHealthInsightStatus) DeepCopy() *UpdateHealthInsightStatus {
	if in == nil {
		return nil
	}
	out := new(UpdateHealthInsightStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Version) DeepCopyInto(out *Version) {
	*out = *in
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = make([]VersionMetadata, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Version.
func (in *Version) DeepCopy() *Version {
	if in == nil {
		return nil
	}
	out := new(Version)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VersionMetadata) DeepCopyInto(out *VersionMetadata) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VersionMetadata.
func (in *VersionMetadata) DeepCopy() *VersionMetadata {
	if in == nil {
		return nil
	}
	out := new(VersionMetadata)
	in.DeepCopyInto(out)
	return out
}
