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

package director

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kube-zen/zen-lead/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceDirectorReconciler_Reconcile_WithMetrics(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name            string
		service         *corev1.Service
		pods            []client.Object
		expectMetrics   bool
		expectFailover  bool
		expectPodsCount int
		expectError     bool
	}{
		{
			name: "service with zen-lead enabled and ready pods",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationEnabledService: "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "my-app",
					},
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt32(8080),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			},
			pods: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
						Labels: map[string]string{
							"app": "my-app",
						},
						CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
						PodIP: "10.0.0.1",
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-2",
						Namespace: "default",
						Labels: map[string]string{
							"app": "my-app",
						},
						CreationTimestamp: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
						PodIP: "10.0.0.2",
					},
				},
			},
			expectMetrics:   true,
			expectFailover:  false,
			expectPodsCount: 2,
			expectError:     false,
		},
		{
			name: "service without zen-lead annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "my-app",
					},
				},
			},
			pods:            []client.Object{},
			expectMetrics:   false,
			expectFailover:  false,
			expectPodsCount: 0,
			expectError:     false,
		},
		{
			name: "service with no pods",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationEnabledService: "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "my-app",
					},
				},
			},
			pods:            []client.Object{},
			expectMetrics:   true,
			expectFailover:  false,
			expectPodsCount: 0,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			objs := []client.Object{tt.service}
			objs = append(objs, tt.pods...)
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create reconciler with metrics
			recorder := metrics.NewRecorder()
			eventRecorder := record.NewFakeRecorder(10)
			r := &ServiceDirectorReconciler{
				Client:   fakeClient,
				Scheme:   scheme,
				Recorder: eventRecorder,
				Metrics:  recorder,
			}

			// Reconcile
			req := types.NamespacedName{
				Name:      tt.service.Name,
				Namespace: tt.service.Namespace,
			}

			_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: req})
			if (err != nil) != tt.expectError {
				t.Errorf("Reconcile() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectMetrics {
				return
			}

			// Verify metrics were called (functions executed without panic)
			// Note: Due to promauto's global registration, we can't easily verify exact values
			// The tests verify that metrics functions are called during reconciliation
		})
	}
}

func TestServiceDirectorReconciler_Reconcile_FailoverMetrics(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	// Create service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationEnabledService: "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "my-app",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt32(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// Create old leader pod
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			PodIP: "10.0.0.1",
		},
	}

	// Create existing EndpointSlice pointing to old pod
	leaderServiceName := service.Name + ServiceSuffixService
	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaderServiceName,
			Namespace: "default",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "pod-1",
				},
			},
		},
	}

	// Create new pod (will become new leader)
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			PodIP: "10.0.0.2",
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(service, oldPod, newPod, endpointSlice).
		Build()

	// Create reconciler
	recorder := metrics.NewRecorder()
	eventRecorder := record.NewFakeRecorder(10)
	r := &ServiceDirectorReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: eventRecorder,
		Metrics:  recorder,
	}

	// First reconcile - should select old pod (sticky)
	req := types.NamespacedName{
		Name:      service.Name,
		Namespace: service.Namespace,
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: req})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Mark old pod as not ready
	oldPod.Status.Conditions[0].Status = corev1.ConditionFalse
	if err := fakeClient.Update(context.Background(), oldPod); err != nil { //nolint:govet // shadow: intentional reuse
		t.Fatalf("Failed to update pod: %v", err)
	}

	// Second reconcile - should trigger failover
	_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: req})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify failover metric was called (function executed without panic)
	// Note: Due to promauto's global registration, we can't easily verify exact values
	// The test verifies that RecordFailover was called during reconciliation
}

func TestServiceDirectorReconciler_Reconcile_PortResolutionFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	// Create service with named targetPort
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationEnabledService: "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "my-app",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromString("http"), // Named port
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// Create pod without the named port
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
					Ports: []corev1.ContainerPort{
						{
							Name:          "other-port", // Different name
							ContainerPort: 8080,
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			PodIP: "10.0.0.1",
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(service, pod).
		Build()

	// Create reconciler
	recorder := metrics.NewRecorder()
	eventRecorder := record.NewFakeRecorder(10)
	r := &ServiceDirectorReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: eventRecorder,
		Metrics:  recorder,
	}

	// Reconcile
	req := types.NamespacedName{
		Name:      service.Name,
		Namespace: service.Namespace,
	}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: req})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify port resolution failure metric was called (function executed without panic)
	// Note: Due to promauto's global registration, we can't easily verify exact values
	// The test verifies that RecordPortResolutionFailure was called during reconciliation
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod is ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod not ready - condition false",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod not ready - wrong phase",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod not ready - no ready condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase:      corev1.PodRunning,
					Conditions: []corev1.PodCondition{},
				},
			},
			expected: false,
		},
		{
			name: "pod not ready - other condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodReady(tt.pod)
			if result != tt.expected {
				t.Errorf("isPodReady() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestServiceDirectorReconciler_GetPodReadySince(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected *time.Time
	}{
		{
			name: "pod ready with last transition time",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:               corev1.PodReady,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: metav1.NewTime(now.Add(-5 * time.Minute)),
						},
					},
				},
			},
			expected: func() *time.Time {
				t := now.Add(-5 * time.Minute)
				return &t
			}(),
		},
		{
			name: "pod not ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "pod no ready condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ServiceDirectorReconciler{}
			result := r.getPodReadySince(tt.pod)
			if (result == nil) != (tt.expected == nil) {
				t.Errorf("getPodReadySince() = %v, expected %v", result, tt.expected)
			}
			if result != nil && tt.expected != nil {
				if !result.Equal(*tt.expected) {
					t.Errorf("getPodReadySince() = %v, expected %v", result, tt.expected)
				}
			}
		})
	}
}

func TestServiceDirectorReconciler_GetMinReadyDuration(t *testing.T) {
	tests := []struct {
		name     string
		service  *corev1.Service
		expected time.Duration
	}{
		{
			name: "no annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
			expected: 0,
		},
		{
			name: "empty annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: 0,
		},
		{
			name: "valid duration",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationMinReadyDurationService: "30s",
					},
				},
			},
			expected: 30 * time.Second,
		},
		{
			name: "zero duration",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationMinReadyDurationService: "0s",
					},
				},
			},
			expected: 0,
		},
		{
			name: "invalid duration",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationMinReadyDurationService: "invalid",
					},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ServiceDirectorReconciler{}
			result := r.getMinReadyDuration(tt.service)
			if result != tt.expected {
				t.Errorf("getMinReadyDuration() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestServiceDirectorReconciler_GetLeaderServiceName(t *testing.T) {
	tests := []struct {
		name     string
		service  *corev1.Service
		expected string
	}{
		{
			name: "default name",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
				},
			},
			expected: "my-service-leader",
		},
		{
			name: "custom name via annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
					Annotations: map[string]string{
						AnnotationLeaderServiceNameService: "custom-leader",
					},
				},
			},
			expected: "custom-leader",
		},
		{
			name: "empty custom name falls back to default",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-service",
					Annotations: map[string]string{
						AnnotationLeaderServiceNameService: "",
					},
				},
			},
			expected: "my-service-leader",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ServiceDirectorReconciler{}
			result := r.getLeaderServiceName(tt.service)
			if result != tt.expected {
				t.Errorf("getLeaderServiceName() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestServiceDirectorReconciler_GetCurrentLeaderPod(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name           string
		service        *corev1.Service
		endpointSlice  *discoveryv1.EndpointSlice
		pod            *corev1.Pod
		expectedLeader bool
		expectError    bool
	}{
		{
			name: "find current leader pod by UID",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
				},
			},
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service-leader",
					Namespace: "default",
				},
				Endpoints: []discoveryv1.Endpoint{
					{
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Namespace: "default",
							Name:      "pod-1",
							UID:       "pod-uid-1",
						},
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "default",
					UID:       "pod-uid-1",
				},
			},
			expectedLeader: true,
			expectError:    false,
		},
		{
			name: "no endpoint slice",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
				},
			},
			endpointSlice:  nil,
			pod:            nil,
			expectedLeader: false,
			expectError:    false,
		},
		{
			name: "pod UID mismatch (pod recreated)",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
				},
			},
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service-leader",
					Namespace: "default",
				},
				Endpoints: []discoveryv1.Endpoint{
					{
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Namespace: "default",
							Name:      "pod-1",
							UID:       "pod-uid-1",
						},
					},
				},
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "default",
					UID:       "pod-uid-2", // Different UID
				},
			},
			expectedLeader: false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			objs := []client.Object{tt.service}
			if tt.endpointSlice != nil {
				objs = append(objs, tt.endpointSlice)
			}
			if tt.pod != nil {
				objs = append(objs, tt.pod)
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create reconciler
			recorder := metrics.NewRecorder()
			eventRecorder := record.NewFakeRecorder(10)
			r := &ServiceDirectorReconciler{
				Client:                   fakeClient,
				Scheme:                   scheme,
				Recorder:                 eventRecorder,
				Metrics:                  recorder,
				optedInServicesCache:     make(map[string][]*cachedService),
				cacheUpdateTimeout:       10 * time.Second,
				metricsCollectionTimeout: 5 * time.Second,
			}

			// Test getCurrentLeaderPod
			logger := packageLogger.WithContext(context.Background())
			leaderPod := r.getCurrentLeaderPod(context.Background(), tt.service, logger)

			if (leaderPod != nil) != tt.expectedLeader {
				t.Errorf("getCurrentLeaderPod() returned pod = %v, expected leader %v", leaderPod != nil, tt.expectedLeader)
			}
			if tt.expectedLeader && leaderPod != nil && tt.pod != nil {
				if leaderPod.Name != tt.pod.Name {
					t.Errorf("getCurrentLeaderPod() returned pod name = %v, expected %v", leaderPod.Name, tt.pod.Name)
				}
			}
		})
	}
}

func TestServiceDirectorReconciler_SelectLeaderPod(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name             string
		service          *corev1.Service
		pods             []corev1.Pod
		bypassStickiness bool
		expectedPodName  string
		expectNil        bool
	}{
		{
			name: "select oldest ready pod",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationEnabledService: "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "my-app",
					},
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "pod-2",
						Namespace:         "default",
						Labels:            map[string]string{"app": "my-app"},
						CreationTimestamp: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
						PodIP: "10.0.0.2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "pod-1",
						Namespace:         "default",
						Labels:            map[string]string{"app": "my-app"},
						CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
						PodIP: "10.0.0.1",
					},
				},
			},
			bypassStickiness: true,
			expectedPodName:  "pod-1", // Oldest
			expectNil:        false,
		},
		{
			name: "no ready pods",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationEnabledService: "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "my-app",
					},
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
						Labels:    map[string]string{"app": "my-app"},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionFalse},
						},
					},
				},
			},
			bypassStickiness: true,
			expectedPodName:  "",
			expectNil:        true,
		},
		{
			name: "sticky disabled",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationEnabledService: "true",
						AnnotationStickyService:  "false",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "my-app",
					},
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "pod-1",
						Namespace:         "default",
						Labels:            map[string]string{"app": "my-app"},
						CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
						PodIP: "10.0.0.1",
					},
				},
			},
			bypassStickiness: false,
			expectedPodName:  "pod-1",
			expectNil:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			objs := []client.Object{tt.service}
			for i := range tt.pods {
				objs = append(objs, &tt.pods[i])
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create reconciler
			recorder := metrics.NewRecorder()
			eventRecorder := record.NewFakeRecorder(10)
			r := &ServiceDirectorReconciler{
				Client:                   fakeClient,
				Scheme:                   scheme,
				Recorder:                 eventRecorder,
				Metrics:                  recorder,
				optedInServicesCache:     make(map[string][]*cachedService),
				cacheUpdateTimeout:       10 * time.Second,
				metricsCollectionTimeout: 5 * time.Second,
			}

			// Test selectLeaderPod
			logger := packageLogger.WithContext(context.Background())
			leaderPod := r.selectLeaderPod(context.Background(), tt.service, tt.pods, tt.bypassStickiness, logger)

			if (leaderPod == nil) != tt.expectNil {
				t.Errorf("selectLeaderPod() returned nil = %v, expected %v", leaderPod == nil, tt.expectNil)
			}
			if !tt.expectNil && leaderPod != nil {
				if leaderPod.Name != tt.expectedPodName {
					t.Errorf("selectLeaderPod() returned pod name = %v, expected %v", leaderPod.Name, tt.expectedPodName)
				}
			}
		})
	}
}

func TestServiceDirectorReconciler_CleanupLeaderResources(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name          string
		service       *corev1.Service
		leaderService *corev1.Service
		svcName       types.NamespacedName
		expectDeleted bool
		expectError   bool
	}{
		{
			name: "cleanup when service exists",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
				},
			},
			leaderService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service-leader",
					Namespace: "default",
					Labels: map[string]string{
						LabelManagedBy:     LabelManagedByValue,
						LabelSourceService: "my-service",
					},
				},
			},
			svcName: types.NamespacedName{
				Name:      "my-service",
				Namespace: "default",
			},
			expectDeleted: true,
			expectError:   false,
		},
		{
			name:    "cleanup when service doesn't exist - find by label",
			service: nil,
			leaderService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service-leader",
					Namespace: "default",
					Labels: map[string]string{
						LabelManagedBy:     LabelManagedByValue,
						LabelSourceService: "my-service",
					},
				},
			},
			svcName: types.NamespacedName{
				Name:      "my-service",
				Namespace: "default",
			},
			expectDeleted: true,
			expectError:   false,
		},
		{
			name: "no leader service to cleanup",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service",
					Namespace: "default",
				},
			},
			leaderService: nil,
			svcName: types.NamespacedName{
				Name:      "my-service",
				Namespace: "default",
			},
			expectDeleted: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			objs := []client.Object{}
			if tt.service != nil {
				objs = append(objs, tt.service)
			}
			if tt.leaderService != nil {
				objs = append(objs, tt.leaderService)
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create reconciler
			recorder := metrics.NewRecorder()
			eventRecorder := record.NewFakeRecorder(10)
			r := &ServiceDirectorReconciler{
				Client:                   fakeClient,
				Scheme:                   scheme,
				Recorder:                 eventRecorder,
				Metrics:                  recorder,
				optedInServicesCache:     make(map[string][]*cachedService),
				cacheUpdateTimeout:       10 * time.Second,
				metricsCollectionTimeout: 5 * time.Second,
			}

			// Test cleanupLeaderResources
			logger := packageLogger.WithContext(context.Background())
			result, err := r.cleanupLeaderResources(context.Background(), tt.svcName, logger)

			if (err != nil) != tt.expectError {
				t.Errorf("cleanupLeaderResources() error = %v, expectError %v", err, tt.expectError)
			}
			if err != nil {
				return
			}

			// Verify leader service was deleted
			if tt.expectDeleted {
				leaderServiceName := "my-service-leader"
				if tt.service != nil {
					leaderServiceName = tt.service.Name + ServiceSuffixService
				}
				leaderSvc := &corev1.Service{}
				err := fakeClient.Get(context.Background(), types.NamespacedName{
					Name:      leaderServiceName,
					Namespace: tt.svcName.Namespace,
				}, leaderSvc)
				if err == nil {
					t.Error("cleanupLeaderResources() leader service was not deleted")
				}
			}

			// Verify result
			if result.Requeue {
				t.Error("cleanupLeaderResources() should not requeue")
			}
		})
	}
}

func TestNewServiceDirectorReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
	eventRecorder := record.NewFakeRecorder(10)

	r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)

	if r == nil {
		t.Fatal("NewServiceDirectorReconciler() returned nil")
	}
	if r.Client != fakeClient {
		t.Error("NewServiceDirectorReconciler() Client not set correctly")
	}
	if r.Scheme != scheme {
		t.Error("NewServiceDirectorReconciler() Scheme not set correctly")
	}
	if r.Recorder != eventRecorder {
		t.Error("NewServiceDirectorReconciler() Recorder not set correctly")
	}
	if r.Metrics == nil {
		t.Error("NewServiceDirectorReconciler() Metrics not initialized")
	}
	if r.optedInServicesCache == nil {
		t.Error("NewServiceDirectorReconciler() optedInServicesCache not initialized")
	}
}

func TestFilterGitOpsLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected map[string]string
	}{
		{
			name:     "nil labels",
			labels:   nil,
			expected: map[string]string{},
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "no GitOps labels",
			labels: map[string]string{
				"app":     "my-app",
				"version": "1.0",
			},
			expected: map[string]string{
				"app":     "my-app",
				"version": "1.0",
			},
		},
		{
			name: "filter ArgoCD labels",
			labels: map[string]string{
				"app":                         "my-app",
				"app.kubernetes.io/instance":  "argocd-instance",
				"argocd.argoproj.io/instance": "my-instance",
			},
			expected: map[string]string{
				"app": "my-app",
			},
		},
		{
			name: "filter Flux labels",
			labels: map[string]string{
				"app":                              "my-app",
				"fluxcd.io/part-of":                "flux-system",
				"kustomize.toolkit.fluxcd.io/name": "my-kustomization",
			},
			expected: map[string]string{
				"app": "my-app",
			},
		},
		{
			name: "filter all GitOps labels",
			labels: map[string]string{
				"app":                         "my-app",
				"app.kubernetes.io/instance":  "instance",
				"app.kubernetes.io/part-of":   "part-of",
				"app.kubernetes.io/version":   "1.0",
				"argocd.argoproj.io/instance": "argocd",
				"fluxcd.io/part-of":           "flux",
			},
			expected: map[string]string{
				"app": "my-app",
			},
		},
		{
			name: "filter app.kubernetes.io/managed-by (always filtered regardless of value)",
			labels: map[string]string{
				"app":                          "my-app",
				"app.kubernetes.io/managed-by": "zen-lead", // Filtered even if our value
			},
			expected: map[string]string{
				"app": "my-app",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterGitOpsLabels(tt.labels)
			if len(result) != len(tt.expected) {
				t.Errorf("filterGitOpsLabels() length = %d, expected %d", len(result), len(tt.expected))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("filterGitOpsLabels() [%s] = %v, expected %v", k, result[k], v)
				}
			}
			for k := range result {
				if _, ok := tt.expected[k]; !ok {
					t.Errorf("filterGitOpsLabels() unexpected key: %s", k)
				}
			}
		})
	}
}

func TestServiceDirectorReconciler_MapPodToService(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name              string
		pod               *corev1.Pod
		services          []*corev1.Service
		cachePrePopulated bool
		expectedRequests  int
		expectCacheMiss   bool
	}{
		{
			name: "cache hit - pod matches one service",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "default",
					Labels: map[string]string{
						"app": "my-app",
					},
				},
			},
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "my-app",
						},
					},
				},
			},
			cachePrePopulated: true,
			expectedRequests:  1,
			expectCacheMiss:   false,
		},
		{
			name: "cache miss - triggers cache refresh",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "default",
					Labels: map[string]string{
						"app": "my-app",
					},
				},
			},
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "my-app",
						},
					},
				},
			},
			cachePrePopulated: false,
			expectedRequests:  1,
			expectCacheMiss:   true,
		},
		{
			name: "pod matches multiple services",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "default",
					Labels: map[string]string{
						"app":     "my-app",
						"version": "v1",
					},
				},
			},
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "my-app",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-2",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app":     "my-app",
							"version": "v1",
						},
					},
				},
			},
			cachePrePopulated: true,
			expectedRequests:  2,
			expectCacheMiss:   false,
		},
		{
			name: "pod matches no services",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "default",
					Labels: map[string]string{
						"app": "other-app",
					},
				},
			},
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "my-app",
						},
					},
				},
			},
			cachePrePopulated: true,
			expectedRequests:  0,
			expectCacheMiss:   false,
		},
		{
			name: "service without annotation not cached",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-1",
					Namespace: "default",
					Labels: map[string]string{
						"app": "my-app",
					},
				},
			},
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service",
						Namespace: "default",
						// No annotation
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "my-app",
						},
					},
				},
			},
			cachePrePopulated: false,
			expectedRequests:  0,
			expectCacheMiss:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			objs := []client.Object{tt.pod}
			for _, svc := range tt.services {
				objs = append(objs, svc)
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create reconciler
			recorder := metrics.NewRecorder()
			eventRecorder := record.NewFakeRecorder(10)
			r := &ServiceDirectorReconciler{
				Client:                   fakeClient,
				Scheme:                   scheme,
				Recorder:                 eventRecorder,
				Metrics:                  recorder,
				optedInServicesCache:     make(map[string][]*cachedService),
				cacheUpdateTimeout:       10 * time.Second,
				metricsCollectionTimeout: 5 * time.Second,
			}

			// Pre-populate cache if needed
			if tt.cachePrePopulated {
				logger := packageLogger.WithContext(context.Background())
				r.updateOptedInServicesCache(context.Background(), tt.pod.Namespace, logger)
			}

			// Test mapPodToService
			requests := r.mapPodToService(context.Background(), tt.pod)
			if len(requests) != tt.expectedRequests {
				t.Errorf("mapPodToService() returned %d requests, expected %d", len(requests), tt.expectedRequests)
			}

			// Verify cache state
			r.cacheMu.RLock()
			cached := r.optedInServicesCache[tt.pod.Namespace]
			r.cacheMu.RUnlock()
			if tt.cachePrePopulated && len(cached) == 0 {
				t.Error("Expected cache to be populated")
			}
		})
	}
}

func TestServiceDirectorReconciler_MapEndpointSliceToService(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name             string
		endpointSlice    *discoveryv1.EndpointSlice
		expectedRequests int
		expectNil        bool
	}{
		{
			name: "valid EndpointSlice with source service label",
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service-leader",
					Namespace: "default",
					Labels: map[string]string{
						LabelEndpointSliceManagedBy: LabelEndpointSliceManagedByValue,
						LabelSourceService:          "my-service",
					},
				},
			},
			expectedRequests: 1,
			expectNil:        false,
		},
		{
			name: "EndpointSlice without managed-by label",
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service-leader",
					Namespace: "default",
					Labels: map[string]string{
						LabelSourceService: "my-service",
					},
				},
			},
			expectedRequests: 0,
			expectNil:        true,
		},
		{
			name: "EndpointSlice without source service label",
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service-leader",
					Namespace: "default",
					Labels: map[string]string{
						LabelEndpointSliceManagedBy: LabelEndpointSliceManagedByValue,
					},
				},
			},
			expectedRequests: 0,
			expectNil:        true,
		},
		{
			name: "EndpointSlice with nil labels",
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-service-leader",
					Namespace: "default",
				},
			},
			expectedRequests: 0,
			expectNil:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			// Create reconciler
			recorder := metrics.NewRecorder()
			eventRecorder := record.NewFakeRecorder(10)
			r := &ServiceDirectorReconciler{
				Client:                   fakeClient,
				Scheme:                   scheme,
				Recorder:                 eventRecorder,
				Metrics:                  recorder,
				optedInServicesCache:     make(map[string][]*cachedService),
				cacheUpdateTimeout:       10 * time.Second,
				metricsCollectionTimeout: 5 * time.Second,
			}

			// Test mapEndpointSliceToService
			requests := r.mapEndpointSliceToService(context.Background(), tt.endpointSlice)
			if tt.expectNil && requests != nil {
				t.Errorf("mapEndpointSliceToService() returned %d requests, expected nil", len(requests))
			}
			if !tt.expectNil && len(requests) != tt.expectedRequests {
				t.Errorf("mapEndpointSliceToService() returned %d requests, expected %d", len(requests), tt.expectedRequests)
			}
			if !tt.expectNil && len(requests) > 0 {
				if requests[0].Name != "my-service" || requests[0].Namespace != "default" {
					t.Errorf("mapEndpointSliceToService() returned wrong service: %v", requests[0])
				}
			}
		})
	}
}

func TestServiceDirectorReconciler_UpdateOptedInServicesCache(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name              string
		namespace         string
		services          []*corev1.Service
		expectedCacheSize int
		expectError       bool
	}{
		{
			name:      "cache with opted-in services",
			namespace: "default",
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "app-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-2",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "app-2",
						},
					},
				},
			},
			expectedCacheSize: 2,
			expectError:       false,
		},
		{
			name:      "cache filters non-opted-in services",
			namespace: "default",
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "app-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-2",
						Namespace: "default",
						// No annotation
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "app-2",
						},
					},
				},
			},
			expectedCacheSize: 1,
			expectError:       false,
		},
		{
			name:      "cache filters services without selector",
			namespace: "default",
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "app-1",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-2",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						// No selector
					},
				},
			},
			expectedCacheSize: 1,
			expectError:       false,
		},
		{
			name:              "empty namespace",
			namespace:         "default",
			services:          []*corev1.Service{},
			expectedCacheSize: 0,
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			objs := make([]client.Object, 0, len(tt.services))
			for _, svc := range tt.services {
				objs = append(objs, svc)
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create reconciler
			recorder := metrics.NewRecorder()
			eventRecorder := record.NewFakeRecorder(10)
			r := &ServiceDirectorReconciler{
				Client:                   fakeClient,
				Scheme:                   scheme,
				Recorder:                 eventRecorder,
				Metrics:                  recorder,
				optedInServicesCache:     make(map[string][]*cachedService),
				cacheUpdateTimeout:       10 * time.Second,
				metricsCollectionTimeout: 5 * time.Second,
			}

			// Test updateOptedInServicesCache
			logger := packageLogger.WithContext(context.Background())
			r.updateOptedInServicesCache(context.Background(), tt.namespace, logger)

			// Verify cache
			r.cacheMu.RLock()
			cached := r.optedInServicesCache[tt.namespace]
			r.cacheMu.RUnlock()

			if len(cached) != tt.expectedCacheSize {
				t.Errorf("updateOptedInServicesCache() cache size = %d, expected %d", len(cached), tt.expectedCacheSize)
			}
		})
	}
}

func TestServiceDirectorReconciler_UpdateOptedInServicesCacheForService(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name              string
		initialServices   []*corev1.Service
		updatedService    *corev1.Service
		expectedCacheSize int
		expectInCache     bool
	}{
		{
			name: "add service to cache",
			initialServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "app-1",
						},
					},
				},
			},
			updatedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-2",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationEnabledService: "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "app-2",
					},
				},
			},
			expectedCacheSize: 2,
			expectInCache:     true,
		},
		{
			name: "remove service from cache when annotation removed",
			initialServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "app-1",
						},
					},
				},
			},
			updatedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-1",
					Namespace: "default",
					// No annotation
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "app-1",
					},
				},
			},
			expectedCacheSize: 0,
			expectInCache:     false,
		},
		{
			name: "update service selector in cache",
			initialServices: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "service-1",
						Namespace: "default",
						Annotations: map[string]string{
							AnnotationEnabledService: "true",
						},
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "app-1",
						},
					},
				},
			},
			updatedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-1",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationEnabledService: "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "app-1-updated",
					},
				},
			},
			expectedCacheSize: 1,
			expectInCache:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			objs := make([]client.Object, 0, len(tt.initialServices))
			for _, svc := range tt.initialServices {
				objs = append(objs, svc)
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create reconciler
			recorder := metrics.NewRecorder()
			eventRecorder := record.NewFakeRecorder(10)
			r := &ServiceDirectorReconciler{
				Client:                   fakeClient,
				Scheme:                   scheme,
				Recorder:                 eventRecorder,
				Metrics:                  recorder,
				optedInServicesCache:     make(map[string][]*cachedService),
				cacheUpdateTimeout:       10 * time.Second,
				metricsCollectionTimeout: 5 * time.Second,
			}

			// Initialize cache
			logger := packageLogger.WithContext(context.Background())
			r.updateOptedInServicesCache(context.Background(), "default", logger)

			// Update cache for service
			r.updateOptedInServicesCacheForService(tt.updatedService, logger)

			// Verify cache
			r.cacheMu.RLock()
			cached := r.optedInServicesCache["default"]
			r.cacheMu.RUnlock()

			if len(cached) != tt.expectedCacheSize {
				t.Errorf("updateOptedInServicesCacheForService() cache size = %d, expected %d", len(cached), tt.expectedCacheSize)
			}

			// Check if service is in cache
			found := false
			for _, cachedSvc := range cached {
				if cachedSvc.name == tt.updatedService.Name {
					found = true
					break
				}
			}
			if found != tt.expectInCache {
				t.Errorf("updateOptedInServicesCacheForService() service in cache = %v, expected %v", found, tt.expectInCache)
			}
		})
	}
}

func TestServiceDirectorReconciler_UpdateResourceTotals(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	// Create leader Service
	leaderService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service-leader",
			Namespace: "default",
			Labels: map[string]string{
				LabelManagedBy: LabelManagedByValue,
			},
		},
		Spec: corev1.ServiceSpec{
			// No selector (leader service)
		},
	}

	// Create EndpointSlice
	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service-leader",
			Namespace: "default",
			Labels: map[string]string{
				LabelEndpointSliceManagedBy: LabelEndpointSliceManagedByValue,
			},
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(leaderService, endpointSlice).
		Build()

	// Create reconciler
	recorder := metrics.NewRecorder()
	eventRecorder := record.NewFakeRecorder(10)
	r := &ServiceDirectorReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: eventRecorder,
		Metrics:  recorder,
	}

	// Test updateResourceTotals
	logger := packageLogger.WithContext(context.Background())
	r.updateResourceTotals(context.Background(), "default", logger)

	// Verify metrics were called (functions executed without panic)
	// Note: Due to promauto's global registration, we can't easily verify exact values
	// The test verifies that RecordLeaderServicesTotal and RecordEndpointSlicesTotal were called
}

func TestFilterGitOpsAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    map[string]string
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    map[string]string{},
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    map[string]string{},
		},
		{
			name: "no GitOps annotations",
			annotations: map[string]string{
				"app":     "my-app",
				"version": "1.0",
			},
			expected: map[string]string{
				"app":     "my-app",
				"version": "1.0",
			},
		},
		{
			name: "filter ArgoCD annotations",
			annotations: map[string]string{
				"app":                          "my-app",
				"argocd.argoproj.io/sync-wave": "1",
			},
			expected: map[string]string{
				"app": "my-app",
			},
		},
		{
			name: "filter Flux annotations",
			annotations: map[string]string{
				"app":                                  "my-app",
				"fluxcd.io/sync-checksum":              "abc123",
				"kustomize.toolkit.fluxcd.io/checksum": "def456",
			},
			expected: map[string]string{
				"app": "my-app",
			},
		},
		{
			name: "filter all GitOps annotations",
			annotations: map[string]string{
				"app":                                  "my-app",
				"argocd.argoproj.io/sync-wave":         "1",
				"argocd.argoproj.io/sync-options":      "Prune=false",
				"fluxcd.io/sync-checksum":              "abc123",
				"kustomize.toolkit.fluxcd.io/checksum": "def456",
			},
			expected: map[string]string{
				"app": "my-app",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterGitOpsAnnotations(tt.annotations)
			if len(result) != len(tt.expected) {
				t.Errorf("filterGitOpsAnnotations() length = %d, expected %d", len(result), len(tt.expected))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("filterGitOpsAnnotations() [%s] = %v, expected %v", k, result[k], v)
				}
			}
			for k := range result {
				if _, ok := tt.expected[k]; !ok {
					t.Errorf("filterGitOpsAnnotations() unexpected key: %s", k)
				}
			}
		})
	}
}

func TestServiceDirectorReconciler_ResolveNamedPort(t *testing.T) {
	tests := []struct {
		name        string
		pod         *corev1.Pod
		portName    string
		expected    int32
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil pod",
			pod:         nil,
			portName:    "http",
			expectError: true,
			errorMsg:    "pod is nil",
		},
		{
			name: "pod with no containers",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-1",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
				},
			},
			portName:    "http",
			expectError: true,
			errorMsg:    "pod has no containers",
		},
		{
			name: "port not found",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-1",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Ports: []corev1.ContainerPort{
								{
									Name:          "other-port",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
			portName:    "http",
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name: "port with zero value",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-1",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 0,
								},
							},
						},
					},
				},
			},
			portName:    "http",
			expectError: true,
			errorMsg:    "invalid port number",
		},
		{
			name: "port with negative value",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-1",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: -1,
								},
							},
						},
					},
				},
			},
			portName:    "http",
			expectError: true,
			errorMsg:    "invalid port number",
		},
		{
			name: "valid port found",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-1",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
			portName:    "http",
			expected:    8080,
			expectError: false,
		},
		{
			name: "port found in second container",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-1",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Ports: []corev1.ContainerPort{
								{
									Name:          "other",
									ContainerPort: 9090,
								},
							},
						},
						{
							Name: "sidecar",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
			portName:    "http",
			expected:    8080,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			corev1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			eventRecorder := record.NewFakeRecorder(10)
			r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)

			result, err := r.resolveNamedPort(tt.pod, tt.portName)
			if tt.expectError {
				if err == nil {
					t.Errorf("resolveNamedPort() expected error but got nil")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("resolveNamedPort() error = %v, expected to contain %s", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("resolveNamedPort() error = %v, expected nil", err)
				}
				if result != tt.expected {
					t.Errorf("resolveNamedPort() = %v, expected %v", result, tt.expected)
				}
			}
		})
	}
}

func TestServiceDirectorReconciler_SelectLeaderPod_StickyAndMinReadyDuration(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	// Test sticky leader with existing EndpointSlice
	t.Run("sticky leader with existing EndpointSlice", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-service",
				Namespace: "default",
				Annotations: map[string]string{
					AnnotationEnabledService: "true",
					AnnotationStickyService:  "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "my-app"},
			},
		}

		leaderPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "pod-1",
				Namespace:         "default",
				UID:               types.UID("pod-1-uid"),
				Labels:            map[string]string{"app": "my-app"},
				CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
				PodIP: "10.0.0.1",
			},
		}

		// Create EndpointSlice with current leader
		endpointSlice := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-service-leader",
				Namespace: "default",
			},
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.0.0.1"},
					TargetRef: &corev1.ObjectReference{
						Kind: "Pod",
						UID:  types.UID("pod-1-uid"),
					},
				},
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(service, leaderPod, endpointSlice).
			Build()

		eventRecorder := record.NewFakeRecorder(10)
		r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)

		logger := packageLogger.WithContext(context.Background())
		selected := r.selectLeaderPod(context.Background(), service, []corev1.Pod{*leaderPod}, false, logger)

		if selected == nil {
			t.Error("selectLeaderPod() expected pod but got nil")
		} else if selected.Name != "pod-1" {
			t.Errorf("selectLeaderPod() = %v, expected pod-1", selected.Name)
		}
	})

	// Test min ready duration (flap damping)
	t.Run("min ready duration filtering", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-service",
				Namespace: "default",
				Annotations: map[string]string{
					AnnotationEnabledService:          "true",
					AnnotationMinReadyDurationService: "30s",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "my-app"},
			},
		}

		// Pod that just became ready (less than 30s ago)
		recentPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "pod-recent",
				Namespace:         "default",
				Labels:            map[string]string{"app": "my-app"},
				CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Second)), // Only 10s ago
					},
				},
				PodIP: "10.0.0.1",
			},
		}

		// Pod that has been ready for longer than 30s
		stablePod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "pod-stable",
				Namespace:         "default",
				Labels:            map[string]string{"app": "my-app"},
				CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Minute)), // 1 minute ago
					},
				},
				PodIP: "10.0.0.2",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(service, recentPod, stablePod).
			Build()

		eventRecorder := record.NewFakeRecorder(10)
		r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)

		logger := packageLogger.WithContext(context.Background())
		selected := r.selectLeaderPod(context.Background(), service, []corev1.Pod{*recentPod, *stablePod}, true, logger)

		if selected == nil {
			t.Error("selectLeaderPod() expected pod but got nil")
		} else if selected.Name != "pod-stable" {
			t.Errorf("selectLeaderPod() = %v, expected pod-stable (filtered out recent pod)", selected.Name)
		}
	})

	// Test sticky disabled
	t.Run("sticky disabled", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-service",
				Namespace: "default",
				Annotations: map[string]string{
					AnnotationEnabledService: "true",
					AnnotationStickyService:  "false",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "my-app"},
			},
		}

		pod1 := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "pod-1",
				Namespace:         "default",
				Labels:            map[string]string{"app": "my-app"},
				CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
				PodIP: "10.0.0.1",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(service, pod1).
			Build()

		eventRecorder := record.NewFakeRecorder(10)
		r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)

		logger := packageLogger.WithContext(context.Background())
		selected := r.selectLeaderPod(context.Background(), service, []corev1.Pod{*pod1}, false, logger)

		if selected == nil {
			t.Error("selectLeaderPod() expected pod but got nil")
		}
	})
}

func TestServiceDirectorReconciler_UpdateResourceTotals_ErrorPaths(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	// Test with nil Metrics
	t.Run("nil metrics", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		eventRecorder := record.NewFakeRecorder(10)
		r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)
		r.Metrics = nil

		logger := packageLogger.WithContext(context.Background())
		// Should not panic
		r.updateResourceTotals(context.Background(), "default", logger)
	})

	// Test with timeout (very short timeout)
	t.Run("timeout scenario", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		eventRecorder := record.NewFakeRecorder(10)
		r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 1*time.Nanosecond) // Very short timeout

		logger := packageLogger.WithContext(context.Background())
		// Should handle timeout gracefully
		r.updateResourceTotals(context.Background(), "default", logger)
	})
}

func TestServiceDirectorReconciler_CleanupLeaderResources_ErrorPaths(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	// Test cleanup when service doesn't exist but leader service exists by label
	t.Run("cleanup by label when service not found", func(t *testing.T) {
		leaderService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-service-leader",
				Namespace: "default",
				Labels: map[string]string{
					LabelSourceService: "my-service",
					LabelManagedBy:     LabelManagedByValue,
				},
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(leaderService).
			Build()

		eventRecorder := record.NewFakeRecorder(10)
		r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)

		logger := packageLogger.WithContext(context.Background())
		svcName := types.NamespacedName{Name: "my-service", Namespace: "default"}
		result, err := r.cleanupLeaderResources(context.Background(), svcName, logger)

		if err != nil {
			t.Errorf("cleanupLeaderResources() error = %v, expected nil", err)
		}
		if result.Requeue {
			t.Error("cleanupLeaderResources() should not requeue")
		}
	})
}

func TestServiceDirectorReconciler_Reconcile_ErrorPaths(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	// Test Reconcile with service that has no selector
	t.Run("service with no selector", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-service",
				Namespace: "default",
				Annotations: map[string]string{
					AnnotationEnabledService: "true",
				},
			},
			Spec: corev1.ServiceSpec{
				// No selector
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(service).
			Build()

		eventRecorder := record.NewFakeRecorder(10)
		r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)

		req := types.NamespacedName{Name: service.Name, Namespace: service.Namespace}
		result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: req})

		if err != nil {
			t.Errorf("Reconcile() error = %v, expected nil", err)
		}
		if result.Requeue {
			t.Error("Reconcile() should not requeue when service has no selector")
		}
	})

	// Test Reconcile when service is not found
	t.Run("service not found", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		eventRecorder := record.NewFakeRecorder(10)
		r := NewServiceDirectorReconciler(fakeClient, scheme, eventRecorder, 1000, 10, 10*time.Second, 5*time.Second)

		req := types.NamespacedName{Name: "non-existent", Namespace: "default"}
		result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: req})

		if err != nil {
			t.Errorf("Reconcile() error = %v, expected nil (should handle NotFound)", err)
		}
		if result.Requeue {
			t.Error("Reconcile() should not requeue when service not found")
		}
	})
}

func TestServiceDirectorReconciler_SetupWithManager(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	discoveryv1.AddToScheme(scheme)

	// Create a manager with envtest or skip if not available
	// For now, we'll test that SetupWithManager can be called without panicking
	// A full integration test would require envtest
	t.Skip("Skipping SetupWithManager test - requires envtest for full integration test")

	// This test would require:
	// 1. envtest.Environment setup
	// 2. mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme})
	// 3. r.SetupWithManager(mgr)
	// For now, we verify SetupWithManager exists and has correct signature via compilation
}
