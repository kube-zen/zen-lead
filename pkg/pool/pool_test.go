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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetPoolFromPod(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected string
		found    bool
	}{
		{
			name: "pod with pool annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationPool: "my-pool",
					},
				},
			},
			expected: "my-pool",
			found:    true,
		},
		{
			name: "pod without pool annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: "",
			found:    false,
		},
		{
			name: "pod without annotations",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: "",
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, found := GetPoolFromPod(tt.pod)
			if found != tt.found {
				t.Errorf("Expected found=%v, got %v", tt.found, found)
			}
			if pool != tt.expected {
				t.Errorf("Expected pool=%s, got %s", tt.expected, pool)
			}
		})
	}
}

func TestIsParticipating(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "participating pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationJoin: "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "non-participating pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationJoin: "false",
					},
				},
			},
			expected: false,
		},
		{
			name: "pod without join annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsParticipating(tt.pod)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetCurrentRole(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected string
	}{
		{
			name: "leader pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationRole: RoleLeader,
					},
				},
			},
			expected: RoleLeader,
		},
		{
			name: "follower pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationRole: RoleFollower,
					},
				},
			},
			expected: RoleFollower,
		},
		{
			name: "pod without role",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCurrentRole(tt.pod)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestManager_FindCandidates(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	tests := []struct {
		name      string
		pods      []client.Object
		poolName  string
		namespace string
		expected  int
	}{
		{
			name: "find candidates",
			pods: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationPool: "my-pool",
							AnnotationJoin:  "true",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-2",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationPool: "my-pool",
							AnnotationJoin:  "true",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-3",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationPool: "other-pool",
							AnnotationJoin:  "true",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			poolName:  "my-pool",
			namespace: "default",
			expected:  2,
		},
		{
			name: "no candidates",
			pods: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationPool: "other-pool",
							AnnotationJoin:  "true",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			poolName:  "my-pool",
			namespace: "default",
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.pods...).
				Build()

			mgr := NewManager(fakeClient)
			candidates, err := mgr.FindCandidates(context.Background(), tt.namespace, tt.poolName)
			if err != nil {
				t.Fatalf("FindCandidates() error = %v", err)
			}

			if len(candidates) != tt.expected {
				t.Errorf("Expected %d candidates, got %d", tt.expected, len(candidates))
			}
		})
	}
}

func TestManager_UpdatePodRole(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-uid"),
			Annotations: map[string]string{
				AnnotationRole: RoleFollower,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pod).
		Build()

	mgr := NewManager(fakeClient)

	// Update role to leader
	if err := mgr.UpdatePodRole(context.Background(), pod, RoleLeader); err != nil {
		t.Fatalf("UpdatePodRole() error = %v", err)
	}

	// Verify update
	updatedPod := &corev1.Pod{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{
		Name:      "test-pod",
		Namespace: "default",
	}, updatedPod); err != nil {
		t.Fatalf("Failed to get updated pod: %v", err)
	}

	if updatedPod.Annotations[AnnotationRole] != RoleLeader {
		t.Errorf("Expected role %s, got %s", RoleLeader, updatedPod.Annotations[AnnotationRole])
	}
}

