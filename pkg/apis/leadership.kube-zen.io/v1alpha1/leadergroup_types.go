/*
Copyright 2025 Kube-ZEN Contributors

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
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LeaderGroupType defines the type of leadership group.
type LeaderGroupType string

const (
	// LeaderGroupTypeRouting is for network-level routing (Profile A).
	// zen-lead creates a leader Service + EndpointSlice.
	LeaderGroupTypeRouting LeaderGroupType = "routing"

	// LeaderGroupTypeController is for controller HA (Profile C).
	// zen-lead creates and manages a Lease resource.
	LeaderGroupTypeController LeaderGroupType = "controller"
)

// LeaderGroupSpec defines the desired state of LeaderGroup
type LeaderGroupSpec struct {
	// Type determines what zen-lead manages:
	// - "routing": Creates leader Service + EndpointSlice (Profile A)
	// - "controller": Creates and manages Lease for controller HA (Profile C)
	// +kubebuilder:validation:Enum=routing;controller
	// +kubebuilder:default=controller
	Type LeaderGroupType `json:"type"`

	// Selector is used for routing type to select pods.
	// Required when Type=routing.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Component is the component name for controller type.
	// Used to derive Lease name deterministically.
	// Required when Type=controller.
	// +optional
	Component string `json:"component,omitempty"`

	// Lease settings for controller type.
	// +optional
	Lease *LeaseSettings `json:"lease,omitempty"`

	// Routing settings for routing type.
	// +optional
	Routing *RoutingSettings `json:"routing,omitempty"`
}

// LeaseSettings configures Lease behavior for controller type.
type LeaseSettings struct {
	// Duration is how long a leader holds the lease before it expires.
	// Default: 15s (controller-runtime default)
	// +kubebuilder:default=15s
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// RenewDeadline is the time to renew the lease before losing leadership.
	// Default: 10s (controller-runtime default)
	// +kubebuilder:default=10s
	// +optional
	RenewDeadline *metav1.Duration `json:"renewDeadline,omitempty"`

	// RetryPeriod is how often to retry acquiring leadership.
	// Default: 2s (controller-runtime default)
	// +kubebuilder:default=2s
	// +optional
	RetryPeriod *metav1.Duration `json:"retryPeriod,omitempty"`
}

// RoutingSettings configures routing behavior for routing type.
type RoutingSettings struct {
	// Enabled enables routing (creates leader Service + EndpointSlice).
	// Default: true
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// LeaderGroupStatus defines the observed state of LeaderGroup
type LeaderGroupStatus struct {
	// HolderIdentity is the identity of the current leader (from Lease).
	// Only populated for controller type.
	// +optional
	HolderIdentity string `json:"holderIdentity,omitempty"`

	// RenewTime is when the lease was last renewed (from Lease).
	// Only populated for controller type.
	// +optional
	RenewTime *metav1.Time `json:"renewTime,omitempty"`

	// LeaseDurationSeconds is the lease duration in seconds (from Lease).
	// Only populated for controller type.
	// +optional
	LeaseDurationSeconds *int32 `json:"leaseDurationSeconds,omitempty"`

	// FencingToken is a monotonic token for advanced safety (optional).
	// Only populated for controller type.
	// +optional
	FencingToken *int64 `json:"fencingToken,omitempty"`

	// ObservedLeaseResourceVersion is the resource version of the observed Lease.
	// Used for drift detection.
	// +optional
	ObservedLeaseResourceVersion string `json:"observedLeaseResourceVersion,omitempty"`

	// Conditions represent the latest available observations of LeaderGroup state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Component",type="string",JSONPath=".spec.component"
// +kubebuilder:printcolumn:name="Holder",type="string",JSONPath=".status.holderIdentity"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LeaderGroup is the Schema for the leadergroups API
// LeaderGroup allows zen-lead to manage leadership for components (Profile C).
// For network-only routing (Profile A), use Service annotations instead.
type LeaderGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LeaderGroupSpec   `json:"spec,omitempty"`
	Status LeaderGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LeaderGroupList contains a list of LeaderGroup
type LeaderGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LeaderGroup `json:"items"`
}

// DeepCopyObject implements runtime.Object for LeaderGroup
func (in *LeaderGroup) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// DeepCopyObject implements runtime.Object for LeaderGroupList
func (in *LeaderGroupList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// DeepCopy creates a deep copy of LeaderGroup
func (in *LeaderGroup) DeepCopy() *LeaderGroup {
	if in == nil {
		return nil
	}
	out := new(LeaderGroup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out
func (in *LeaderGroup) DeepCopyInto(out *LeaderGroup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy creates a deep copy of LeaderGroupSpec
func (in *LeaderGroupSpec) DeepCopy() *LeaderGroupSpec {
	if in == nil {
		return nil
	}
	out := new(LeaderGroupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out
func (in *LeaderGroupSpec) DeepCopyInto(out *LeaderGroupSpec) {
	*out = *in
	out.Type = in.Type
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	out.Component = in.Component
	if in.Lease != nil {
		in, out := &in.Lease, &out.Lease
		*out = new(LeaseSettings)
		(*in).DeepCopyInto(*out)
	}
	if in.Routing != nil {
		in, out := &in.Routing, &out.Routing
		*out = new(RoutingSettings)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy creates a deep copy of LeaderGroupStatus
func (in *LeaderGroupStatus) DeepCopy() *LeaderGroupStatus {
	if in == nil {
		return nil
	}
	out := new(LeaderGroupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out
func (in *LeaderGroupStatus) DeepCopyInto(out *LeaderGroupStatus) {
	*out = *in
	out.HolderIdentity = in.HolderIdentity
	if in.RenewTime != nil {
		in, out := &in.RenewTime, &out.RenewTime
		*out = (*in).DeepCopy()
	}
	if in.LeaseDurationSeconds != nil {
		in, out := &in.LeaseDurationSeconds, &out.LeaseDurationSeconds
		*out = new(int32)
		**out = **in
	}
	if in.FencingToken != nil {
		in, out := &in.FencingToken, &out.FencingToken
		*out = new(int64)
		**out = **in
	}
	out.ObservedLeaseResourceVersion = in.ObservedLeaseResourceVersion
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy creates a deep copy of LeaderGroupList
func (in *LeaderGroupList) DeepCopy() *LeaderGroupList {
	if in == nil {
		return nil
	}
	out := new(LeaderGroupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out
func (in *LeaderGroupList) DeepCopyInto(out *LeaderGroupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LeaderGroup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy creates a deep copy of LeaseSettings
func (in *LeaseSettings) DeepCopy() *LeaseSettings {
	if in == nil {
		return nil
	}
	out := new(LeaseSettings)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out
func (in *LeaseSettings) DeepCopyInto(out *LeaseSettings) {
	*out = *in
	if in.Duration != nil {
		in, out := &in.Duration, &out.Duration
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.RenewDeadline != nil {
		in, out := &in.RenewDeadline, &out.RenewDeadline
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.RetryPeriod != nil {
		in, out := &in.RetryPeriod, &out.RetryPeriod
		*out = new(metav1.Duration)
		**out = **in
	}
}

// DeepCopy creates a deep copy of RoutingSettings
func (in *RoutingSettings) DeepCopy() *RoutingSettings {
	if in == nil {
		return nil
	}
	out := new(RoutingSettings)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out
func (in *RoutingSettings) DeepCopyInto(out *RoutingSettings) {
	*out = *in
}

func init() {
	SchemeBuilder.Register(&LeaderGroup{}, &LeaderGroupList{})
}

