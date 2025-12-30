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

package pool

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AnnotationPool is the annotation key for the pool name
	AnnotationPool = "zen-lead/pool"
	// AnnotationJoin is the annotation key to indicate participation
	AnnotationJoin = "zen-lead/join"
	// AnnotationRole is the annotation key for the role (leader/follower)
	AnnotationRole = "zen-lead/role"
	// AnnotationIdentity is the annotation key for custom identity
	AnnotationIdentity = "zen-lead/identity"

	// RoleLeader indicates this pod is the leader
	RoleLeader = "leader"
	// RoleFollower indicates this pod is a follower
	RoleFollower = "follower"
)

// Manager manages pools of candidates
type Manager struct {
	client client.Client
}

// NewManager creates a new pool manager
func NewManager(client client.Client) *Manager {
	return &Manager{
		client: client,
	}
}

// FindCandidates finds all pods participating in a pool
func (m *Manager) FindCandidates(ctx context.Context, namespace, poolName string) ([]corev1.Pod, error) {
	// List all pods in the namespace
	podList := &corev1.PodList{}
	if err := m.client.List(ctx, podList, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Filter pods by annotations
	var candidates []corev1.Pod
	for _, pod := range podList.Items {
		// Check if pod has the pool annotation
		if pool, ok := pod.Annotations[AnnotationPool]; !ok || pool != poolName {
			continue
		}

		// Check if pod is participating
		if !IsParticipating(&pod) {
			continue
		}

		// Only include running pods
		if pod.Status.Phase == corev1.PodRunning {
			candidates = append(candidates, pod)
		}
	}

	klog.V(4).InfoS("Found candidates for pool",
		"pool", poolName,
		"namespace", namespace,
		"count", len(candidates),
	)

	return candidates, nil
}

// UpdatePodRole updates the role annotation on a pod
func (m *Manager) UpdatePodRole(ctx context.Context, pod *corev1.Pod, role string) error {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	currentRole := pod.Annotations[AnnotationRole]
	if currentRole == role {
		// No change needed
		return nil
	}

	// Create a patch to update the annotation
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Annotations[AnnotationRole] = role

	if err := m.client.Patch(ctx, pod, patch); err != nil {
		return fmt.Errorf("failed to update pod role: %w", err)
	}

	klog.V(2).InfoS("Updated pod role",
		"pod", pod.Name,
		"namespace", pod.Namespace,
		"role", role,
	)

	return nil
}

// GetPoolFromPod extracts the pool name from a pod's annotations
func GetPoolFromPod(pod *corev1.Pod) (string, bool) {
	if pod.Annotations == nil {
		return "", false
	}

	pool, ok := pod.Annotations[AnnotationPool]
	return pool, ok
}

// IsParticipating checks if a pod is participating in leader election
func IsParticipating(pod *corev1.Pod) bool {
	if pod.Annotations == nil {
		return false
	}

	join, ok := pod.Annotations[AnnotationJoin]
	return ok && join == "true"
}

// GetCurrentRole returns the current role of a pod
func GetCurrentRole(pod *corev1.Pod) string {
	if pod.Annotations == nil {
		return ""
	}

	return pod.Annotations[AnnotationRole]
}
