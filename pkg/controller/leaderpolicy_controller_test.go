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

package controller

import (
	"context"
	"testing"
	"time"

	coordinationv1alpha1 "github.com/kube-zen/zen-lead/pkg/apis/coordination.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lead/pkg/pool"
	corev1 "k8s.io/api/core/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLeaderPolicyReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	coordinationv1alpha1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	coordinationv1.AddToScheme(scheme)

	tests := []struct {
		name           string
		policy         *coordinationv1alpha1.LeaderPolicy
		pods           []client.Object
		lease          *coordinationv1.Lease
		expectedPhase  string
		expectedLeader bool
	}{
		{
			name: "no candidates",
			policy: &coordinationv1alpha1.LeaderPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: coordinationv1alpha1.LeaderPolicySpec{
					LeaseDurationSeconds: 15,
					IdentityStrategy:      "pod",
					FollowerMode:          "standby",
				},
			},
			pods:           []client.Object{},
			expectedPhase:  "Electing",
			expectedLeader: false,
		},
		{
			name: "with candidates but no lease",
			policy: &coordinationv1alpha1.LeaderPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: coordinationv1alpha1.LeaderPolicySpec{
					LeaseDurationSeconds: 15,
					IdentityStrategy:      "pod",
					FollowerMode:          "standby",
				},
			},
			pods: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
						Annotations: map[string]string{
							pool.AnnotationPool: "test-pool",
							pool.AnnotationJoin:  "true",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			expectedPhase:  "Electing",
			expectedLeader: false,
		},
		{
			name: "with leader",
			policy: &coordinationv1alpha1.LeaderPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: coordinationv1alpha1.LeaderPolicySpec{
					LeaseDurationSeconds: 15,
					IdentityStrategy:      "pod",
					FollowerMode:          "standby",
				},
			},
			pods: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
						UID:       types.UID("pod-1-uid"),
						Annotations: map[string]string{
							pool.AnnotationPool: "test-pool",
							pool.AnnotationJoin:  "true",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			lease: &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pool",
					Namespace: "default",
				},
				Spec: coordinationv1.LeaseSpec{
					HolderIdentity: stringPtr("pod-1"),
					AcquireTime:    &metav1.MicroTime{Time: time.Now()},
				},
			},
			expectedPhase:  "Stable",
			expectedLeader: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []client.Object{tt.policy}
			objs = append(objs, tt.pods...)
			if tt.lease != nil {
				objs = append(objs, tt.lease)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			poolMgr := pool.NewManager(fakeClient)
			r := &LeaderPolicyReconciler{
				Client:  fakeClient,
				Scheme:  scheme,
				PoolMgr: poolMgr,
			}

			req := types.NamespacedName{
				Name:      tt.policy.Name,
				Namespace: tt.policy.Namespace,
			}

			_, err := r.Reconcile(context.Background(), req)
			if err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			// Verify policy status
			updatedPolicy := &coordinationv1alpha1.LeaderPolicy{}
			if err := fakeClient.Get(context.Background(), req, updatedPolicy); err != nil {
				t.Fatalf("Failed to get updated policy: %v", err)
			}

			if updatedPolicy.Status.Phase != tt.expectedPhase {
				t.Errorf("Expected phase %s, got %s", tt.expectedPhase, updatedPolicy.Status.Phase)
			}

			if tt.expectedLeader && updatedPolicy.Status.CurrentHolder == nil {
				t.Error("Expected leader but got nil")
			}

			if !tt.expectedLeader && updatedPolicy.Status.CurrentHolder != nil {
				t.Error("Expected no leader but got one")
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

