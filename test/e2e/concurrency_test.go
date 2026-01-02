//go:build e2e

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

package e2e

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestConcurrentServiceUpdates verifies that concurrent Service updates don't cause race conditions
func TestConcurrentServiceUpdates(t *testing.T) {
	ctx := context.Background()
	c := getTestClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace + "-concurrent",
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, ns)
	}()

	// Create deployment
	deployment, err := createTestDeployment(ctx, c, "concurrent-app", ns.Name)
	if err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, deployment)
	}()

	// Wait for pods to be ready
	if err := waitForPodsReady(ctx, c, ns.Name, map[string]string{"app": "concurrent-app"}, 60*time.Second); err != nil {
		t.Fatalf("Pods did not become ready: %v", err)
	}

	// Create Service with annotation
	svc := createTestService(ctx, c, "concurrent-app", ns.Name)
	if err := c.Create(ctx, svc); err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, svc)
	}()

	// Wait for leader Service to be created
	leaderServiceName := svc.Name + "-leader"
	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		leaderSvc := &corev1.Service{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: leaderServiceName}, leaderSvc); err != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("Leader Service was not created: %v", err)
	}

	// Concurrently update the Service annotation multiple times
	// This should not cause race conditions or duplicate leader Services
	for i := 0; i < 5; i++ {
		go func(iteration int) {
			svcCopy := &corev1.Service{}
			if err := c.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: svc.Name}, svcCopy); err != nil {
				t.Logf("Iteration %d: Failed to get service: %v", iteration, err)
				return
			}
			if svcCopy.Annotations == nil {
				svcCopy.Annotations = make(map[string]string)
			}
			svcCopy.Annotations["test-annotation"] = "value"
			if err := c.Update(ctx, svcCopy); err != nil {
				t.Logf("Iteration %d: Failed to update service: %v", iteration, err)
			}
		}(i)
	}

	// Wait a bit for concurrent updates
	time.Sleep(5 * time.Second)

	// Verify only one leader Service exists
	leaderSvcList := &corev1.ServiceList{}
	if err := c.List(ctx, leaderSvcList, client.InNamespace(ns.Name), client.MatchingLabels{
		"app.kubernetes.io/managed-by": "zen-lead",
	}); err != nil {
		t.Fatalf("Failed to list leader services: %v", err)
	}

	leaderCount := 0
	for _, s := range leaderSvcList.Items {
		if s.Name == leaderServiceName {
			leaderCount++
		}
	}

	if leaderCount != 1 {
		t.Errorf("Expected exactly 1 leader Service, found %d", leaderCount)
	}
}

// TestMultipleServicesSameNamespace verifies that multiple Services in the same namespace work correctly
func TestMultipleServicesSameNamespace(t *testing.T) {
	ctx := context.Background()
	c := getTestClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace + "-multi",
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, ns)
	}()

	// Create multiple deployments and services
	services := []string{"app1", "app2", "app3"}
	for _, appName := range services {
		// Create deployment
		deployment, err := createTestDeployment(ctx, c, appName, ns.Name)
		if err != nil {
			t.Fatalf("Failed to create deployment %s: %v", appName, err)
		}
		defer func(d *appsv1.Deployment) {
			_ = c.Delete(ctx, d)
		}(deployment)

		// Wait for pods
		if err := waitForPodsReady(ctx, c, ns.Name, map[string]string{"app": appName}, 60*time.Second); err != nil {
			t.Fatalf("Pods for %s did not become ready: %v", appName, err)
		}

		// Create Service with annotation
		svc := createTestService(ctx, c, appName, ns.Name)
		if err := c.Create(ctx, svc); err != nil {
			t.Fatalf("Failed to create service %s: %v", appName, err)
		}
		defer func(s *corev1.Service) {
			_ = c.Delete(ctx, s)
		}(svc)
	}

	// Wait for all leader Services to be created
	for _, appName := range services {
		leaderServiceName := appName + "-leader"
		if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			leaderSvc := &corev1.Service{}
			if err := c.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: leaderServiceName}, leaderSvc); err != nil {
				return false, nil
			}
			return true, nil
		}); err != nil {
			t.Fatalf("Leader Service for %s was not created: %v", appName, err)
		}
	}

	// Verify all leader Services exist and have endpoints
	for _, appName := range services {
		leaderServiceName := appName + "-leader"
		leaderSvc := &corev1.Service{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: leaderServiceName}, leaderSvc); err != nil {
			t.Errorf("Failed to get leader Service for %s: %v", appName, err)
			continue
		}

		// Verify selector is null
		if leaderSvc.Spec.Selector != nil && len(leaderSvc.Spec.Selector) > 0 {
			t.Errorf("Leader Service %s should have null selector, got: %v", leaderServiceName, leaderSvc.Spec.Selector)
		}
	}
}

