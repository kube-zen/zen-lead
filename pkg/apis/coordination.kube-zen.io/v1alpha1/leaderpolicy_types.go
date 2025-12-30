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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LeaderPolicySpec defines the desired state of LeaderPolicy
type LeaderPolicySpec struct {
	// LeaseDurationSeconds is how long a leader holds the lease before it expires.
	// Default: 15 seconds
	// +kubebuilder:default=15
	// +kubebuilder:validation:Minimum=5
	// +kubebuilder:validation:Maximum=300
	LeaseDurationSeconds int32 `json:"leaseDurationSeconds,omitempty"`

	// IdentityStrategy determines how pod identity is derived.
	// - "pod": Uses Pod Name/UID (default)
	// - "custom": Uses annotation value from zen-lead/identity
	// +kubebuilder:default=pod
	// +kubebuilder:validation:Enum=pod;custom
	IdentityStrategy string `json:"identityStrategy,omitempty"`

	// FollowerMode defines what happens to non-leader pods.
	// - "standby": Pods stay running but are marked as followers (default)
	// - "scaleDown": Pods scale to 0 (requires HPA integration, advanced)
	// +kubebuilder:default=standby
	// +kubebuilder:validation:Enum=standby;scaleDown
	FollowerMode string `json:"followerMode,omitempty"`

	// RenewDeadlineSeconds is the time to renew the lease before losing leadership.
	// Default: 10 seconds (must be < LeaseDurationSeconds)
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=60
	RenewDeadlineSeconds int32 `json:"renewDeadlineSeconds,omitempty"`

	// RetryPeriodSeconds is how often to retry acquiring leadership.
	// Default: 2 seconds
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	RetryPeriodSeconds int32 `json:"retryPeriodSeconds,omitempty"`
}

// LeaderHolder represents the current leader
type LeaderHolder struct {
	// Name is the pod name or custom identity
	Name string `json:"name"`

	// UID is the pod UID (if identityStrategy is "pod")
	UID string `json:"uid,omitempty"`

	// StartTime is when this leader acquired the lease
	StartTime metav1.Time `json:"startTime"`

	// Namespace is the namespace of the leader pod
	Namespace string `json:"namespace,omitempty"`
}

// LeaderPolicyStatus defines the observed state of LeaderPolicy
type LeaderPolicyStatus struct {
	// Phase indicates the current election phase
	// - "Electing": No leader yet, candidates are competing
	// - "Stable": A leader is active and holding the lease
	// +kubebuilder:validation:Enum=Electing;Stable
	Phase string `json:"phase,omitempty"`

	// CurrentHolder is the current leader (if any)
	CurrentHolder *LeaderHolder `json:"currentHolder,omitempty"`

	// Candidates is the number of pods participating in the election
	Candidates int32 `json:"candidates,omitempty"`

	// LastTransitionTime is when the phase last changed
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Conditions represent the latest observations of the policy state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Leader",type="string",JSONPath=".status.currentHolder.name"
// +kubebuilder:printcolumn:name="Candidates",type="integer",JSONPath=".status.candidates"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LeaderPolicy is the Schema for the leaderpolicies API
type LeaderPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LeaderPolicySpec   `json:"spec,omitempty"`
	Status LeaderPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LeaderPolicyList contains a list of LeaderPolicy
type LeaderPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LeaderPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LeaderPolicy{}, &LeaderPolicyList{})
}

