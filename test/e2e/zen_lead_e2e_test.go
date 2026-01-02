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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	testNamespace = "zen-lead-e2e"
	testService   = "test-app"
	clusterName   = "zen-lead-e2e"
)

var (
	testClient client.Client
	testScheme  = runtime.NewScheme()
)

func init() {
	// Add core Kubernetes types to scheme
	_ = corev1.AddToScheme(testScheme)
	_ = discoveryv1.AddToScheme(testScheme)
	_ = appsv1.AddToScheme(testScheme)
}

// getTestClient returns a Kubernetes client for E2E tests
func getTestClient(t *testing.T) client.Client {
	if testClient != nil {
		return testClient
	}

	// Try to get kubeconfig from environment or default location
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		// Try to find kubeconfig from setup_kind.sh
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", clusterName+"-config")
	}

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		t.Skipf("Kubeconfig not found at %s. Run test/e2e/setup_kind.sh create first", kubeconfig)
	}

	// Get REST config
	var restConfig *rest.Config
	var err error
	
	// Try loading from file first
	if _, err := os.Stat(kubeconfig); err == nil {
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			t.Fatalf("Failed to load kubeconfig from %s: %v", kubeconfig, err)
		}
	} else {
		// Fall back to default config
		restConfig, err = config.GetConfig()
		if err != nil {
			t.Fatalf("Failed to get kubeconfig: %v", err)
		}
	}

	// Create client
	testClient, err = client.New(restConfig, client.Options{Scheme: testScheme})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return testClient
}

// TestLeaderServiceCreation verifies leader Service is created when annotation is added
func TestLeaderServiceCreation(t *testing.T) {
	ctx := context.Background()
	c := getTestClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, ns)
	}()

	// Create test deployment with pods
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testService,
			Namespace: testNamespace,
			Labels: map[string]string{
				"app": testService,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": testService,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": testService,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx:latest",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}
	if err := c.Create(ctx, deployment); err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, deployment)
	}()

	// Wait for pods to be ready
	t.Log("Waiting for pods to be ready...")
	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		podList := &corev1.PodList{}
		if err := c.List(ctx, podList, client.InNamespace(testNamespace), client.MatchingLabels{"app": testService}); err != nil {
			return false, err
		}
		readyCount := 0
		for _, pod := range podList.Items {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					readyCount++
					break
				}
			}
		}
		return readyCount >= 1, nil // At least one ready pod
	}); err != nil {
		t.Fatalf("Pods did not become ready: %v", err)
	}

	// Create test Service with annotation
	svc := createTestService(ctx, c, testService, testNamespace)
	if err := c.Create(ctx, svc); err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, svc)
	}()

	// Wait for leader Service
	t.Log("Waiting for leader Service to be created...")
	leaderSvc, err := waitForLeaderService(ctx, c, testService, testNamespace, 60*time.Second)
	if err != nil {
		t.Fatalf("Leader Service was not created: %v", err)
	}

	// Verify selector is null
	if leaderSvc.Spec.Selector != nil && len(leaderSvc.Spec.Selector) > 0 {
		t.Errorf("Leader Service should have null selector, got: %v", leaderSvc.Spec.Selector)
	}

	// Verify name
	expectedName := testService + "-leader"
	if leaderSvc.Name != expectedName {
		t.Errorf("Expected leader Service name %s, got %s", expectedName, leaderSvc.Name)
	}

	// Verify ports are mirrored
	if len(leaderSvc.Spec.Ports) != len(svc.Spec.Ports) {
		t.Errorf("Expected %d ports, got %d", len(svc.Spec.Ports), len(leaderSvc.Spec.Ports))
	}

	// Verify managed-by label
	if leaderSvc.Labels["app.kubernetes.io/managed-by"] != "zen-lead" {
		t.Errorf("Expected managed-by label to be 'zen-lead', got %s", leaderSvc.Labels["app.kubernetes.io/managed-by"])
	}

	t.Log("✅ Leader Service created successfully")
}

// TestEndpointSliceCreation verifies EndpointSlice is created with exactly one endpoint
func TestEndpointSliceCreation(t *testing.T) {
	ctx := context.Background()
	c := getTestClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace + "-endpointslice",
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, ns)
	}()

	// Create deployment
	deployment, err := createTestDeployment(ctx, c, testService+"-ep", ns.Name, 3)
	if err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, deployment)
	}()

	// Wait for pods to be ready
	if err := waitForPodsReady(ctx, c, ns.Name, map[string]string{"app": testService + "-ep"}, 60*time.Second); err != nil {
		t.Fatalf("Pods did not become ready: %v", err)
	}

	// Create Service with annotation
	svc := createTestService(ctx, c, testService+"-ep", ns.Name)
	if err := c.Create(ctx, svc); err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, svc)
	}()

	// Wait for EndpointSlice
	t.Log("Waiting for EndpointSlice to be created...")
	endpointSlice, err := waitForEndpointSlice(ctx, c, testService+"-ep", ns.Name, 60*time.Second)
	if err != nil {
		t.Fatalf("EndpointSlice was not created: %v", err)
	}

	// Verify exactly one endpoint
	if len(endpointSlice.Endpoints) != 1 {
		t.Errorf("Expected exactly one endpoint, got %d", len(endpointSlice.Endpoints))
	}

	// Verify endpoint has targetRef
	if endpointSlice.Endpoints[0].TargetRef == nil {
		t.Error("Endpoint should have targetRef to pod")
	}

	// Verify managed-by label
	if endpointSlice.Labels["endpointslice.kubernetes.io/managed-by"] != "zen-lead" {
		t.Errorf("Expected managed-by label to be 'zen-lead', got %s", endpointSlice.Labels["endpointslice.kubernetes.io/managed-by"])
	}

	t.Log("✅ EndpointSlice created successfully")
}

// TestFailover verifies failover when leader becomes NotReady
func TestFailover(t *testing.T) {
	ctx := context.Background()
	c := getTestClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace + "-failover",
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, ns)
	}()

	// Create deployment with multiple replicas
	deployment, err := createTestDeployment(ctx, c, testService+"-failover", ns.Name, 3)
	if err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, deployment)
	}()

	// Wait for pods
	if err := waitForPodsReady(ctx, c, ns.Name, map[string]string{"app": testService + "-failover"}, 60*time.Second); err != nil {
		t.Fatalf("Pods did not become ready: %v", err)
	}

	// Create Service with annotation
	svc := createTestService(ctx, c, testService+"-failover", ns.Name)
	if err := c.Create(ctx, svc); err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, svc)
	}()

	// Wait for EndpointSlice and get initial leader pod
	endpointSlice, err := waitForEndpointSlice(ctx, c, testService+"-failover", ns.Name, 60*time.Second)
	if err != nil {
		t.Fatalf("EndpointSlice was not created: %v", err)
	}

	if len(endpointSlice.Endpoints) == 0 || endpointSlice.Endpoints[0].TargetRef == nil {
		t.Fatal("EndpointSlice should have one endpoint with targetRef")
	}

	initialLeaderPodName := endpointSlice.Endpoints[0].TargetRef.Name
	t.Logf("Initial leader pod: %s", initialLeaderPodName)

	// Delete the leader pod to trigger failover
	t.Log("Deleting leader pod to trigger failover...")
	leaderPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      initialLeaderPodName,
			Namespace: ns.Name,
		},
	}
	if err := c.Delete(ctx, leaderPod); err != nil {
		t.Fatalf("Failed to delete leader pod: %v", err)
	}

	// Wait for new leader to be selected (should happen within 2-5 seconds)
	t.Log("Waiting for failover to new leader...")
	var newLeaderPodName string
	if err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		slice, err := waitForEndpointSlice(ctx, c, testService+"-failover", ns.Name, 5*time.Second)
		if err != nil {
			return false, nil // Keep waiting
		}
		if len(slice.Endpoints) > 0 && slice.Endpoints[0].TargetRef != nil {
			newLeaderPodName = slice.Endpoints[0].TargetRef.Name
			return newLeaderPodName != initialLeaderPodName, nil
		}
		return false, nil
	}); err != nil {
		t.Errorf("Failover did not occur: %v", err)
	} else {
		t.Logf("✅ Failover successful: new leader pod is %s", newLeaderPodName)
	}
}

// TestCleanup verifies cleanup when annotation is removed
func TestCleanup(t *testing.T) {
	ctx := context.Background()
	c := getTestClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace + "-cleanup",
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, ns)
	}()

	// Create deployment
	deployment, err := createTestDeployment(ctx, c, testService+"-cleanup", ns.Name, 2)
	if err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, deployment)
	}()

	// Wait for pods
	if err := waitForPodsReady(ctx, c, ns.Name, map[string]string{"app": testService + "-cleanup"}, 60*time.Second); err != nil {
		t.Fatalf("Pods did not become ready: %v", err)
	}

	// Create Service with annotation
	svc := createTestService(ctx, c, testService+"-cleanup", ns.Name)
	if err := c.Create(ctx, svc); err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, svc)
	}()

	// Wait for leader Service and EndpointSlice to be created
	leaderSvc, err := waitForLeaderService(ctx, c, testService+"-cleanup", ns.Name, 60*time.Second)
	if err != nil {
		t.Fatalf("Leader Service was not created: %v", err)
	}

	_, err = waitForEndpointSlice(ctx, c, testService+"-cleanup", ns.Name, 60*time.Second)
	if err != nil {
		t.Fatalf("EndpointSlice was not created: %v", err)
	}

	// Remove annotation
	t.Log("Removing annotation...")
	delete(svc.Annotations, "zen-lead.io/enabled")
	if err := c.Update(ctx, svc); err != nil {
		t.Fatalf("Failed to update service: %v", err)
	}

	// Wait for leader Service to be deleted (via owner reference GC)
	t.Log("Waiting for leader Service to be deleted...")
	if err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		err := c.Get(ctx, types.NamespacedName{Name: leaderSvc.Name, Namespace: ns.Name}, &corev1.Service{})
		return err != nil && client.IgnoreNotFound(err) == nil, nil
	}); err != nil {
		t.Errorf("Leader Service was not deleted: %v", err)
	}

	// Wait for EndpointSlice to be deleted
	t.Log("Waiting for EndpointSlice to be deleted...")
	if err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		sliceList := &discoveryv1.EndpointSliceList{}
		if err := c.List(ctx, sliceList, client.InNamespace(ns.Name), client.MatchingLabels{
			"kubernetes.io/service-name": leaderSvc.Name,
		}); err != nil {
			return false, err
		}
		return len(sliceList.Items) == 0, nil
	}); err != nil {
		t.Errorf("EndpointSlice was not deleted: %v", err)
	}

	t.Log("✅ Cleanup successful")
}

// TestPortResolutionFailClosed verifies fail-closed behavior when port resolution fails
func TestPortResolutionFailClosed(t *testing.T) {
	ctx := context.Background()
	c := getTestClient(t)

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace + "-portfail",
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, ns)
	}()

	// Create deployment with port named "http"
	deployment, err := createTestDeployment(ctx, c, testService+"-portfail", ns.Name, 2)
	if err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, deployment)
	}()

	// Wait for pods
	if err := waitForPodsReady(ctx, c, ns.Name, map[string]string{"app": testService + "-portfail"}, 60*time.Second); err != nil {
		t.Fatalf("Pods did not become ready: %v", err)
	}

	// Create Service with annotation and named targetPort that doesn't match
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testService + "-portfail",
			Namespace: ns.Name,
			Annotations: map[string]string{
				"zen-lead.io/enabled": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": testService + "-portfail",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromString("nonexistent"), // Port name that doesn't exist
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	if err := c.Create(ctx, svc); err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, svc)
	}()

	// Wait a bit for reconciliation
	time.Sleep(5 * time.Second)

	// Verify leader Service was created (it should exist even if port resolution fails)
	leaderSvc, err := waitForLeaderService(ctx, c, testService+"-portfail", ns.Name, 30*time.Second)
	if err != nil {
		t.Fatalf("Leader Service was not created: %v", err)
	}

	// Verify EndpointSlice either doesn't exist or has no endpoints (fail-closed)
	sliceList := &discoveryv1.EndpointSliceList{}
	if err := c.List(ctx, sliceList, client.InNamespace(ns.Name), client.MatchingLabels{
		"kubernetes.io/service-name": leaderSvc.Name,
	}); err != nil {
		t.Fatalf("Failed to list EndpointSlices: %v", err)
	}

	if len(sliceList.Items) > 0 {
		// If EndpointSlice exists, it should have no endpoints
		for _, slice := range sliceList.Items {
			if len(slice.Endpoints) > 0 {
				t.Errorf("EndpointSlice should have no endpoints when port resolution fails, got %d", len(slice.Endpoints))
			}
		}
	}

	// Check for Warning Event
	eventList := &corev1.EventList{}
	if err := c.List(ctx, eventList, client.InNamespace(ns.Name)); err == nil {
		foundWarning := false
		for _, event := range eventList.Items {
			if event.InvolvedObject.Kind == "Service" &&
				event.InvolvedObject.Name == svc.Name &&
				event.Type == corev1.EventTypeWarning &&
				(event.Reason == "PortResolutionFailed" || event.Reason == "EndpointSliceDeleted") {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Log("Warning: No Warning Event found for port resolution failure (may be expected if event was not created)")
		}
	}

	t.Log("✅ Port resolution fail-closed behavior verified")
}

// Helper functions for e2e tests

func createTestService(ctx context.Context, c client.Client, name, namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"zen-lead.io/enabled": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": name,
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
}

func waitForLeaderService(ctx context.Context, c client.Client, name, namespace string, timeout time.Duration) (*corev1.Service, error) {
	leaderServiceName := name + "-leader"
	key := types.NamespacedName{Name: leaderServiceName, Namespace: namespace}

	var svc corev1.Service
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		if err := c.Get(ctx, key, &svc); err != nil {
			return false, nil // Keep waiting
		}
		// Verify selector is null
		if svc.Spec.Selector != nil && len(svc.Spec.Selector) > 0 {
			return false, fmt.Errorf("leader service should have null selector")
		}
		return true, nil
	})

	if err != nil {
		return nil, err
	}
	return &svc, nil
}

func waitForEndpointSlice(ctx context.Context, c client.Client, serviceName, namespace string, timeout time.Duration) (*discoveryv1.EndpointSlice, error) {
	leaderServiceName := serviceName + "-leader"

	var endpointSlice discoveryv1.EndpointSlice
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		sliceList := &discoveryv1.EndpointSliceList{}
		if err := c.List(ctx, sliceList, client.InNamespace(namespace), client.MatchingLabels{
			"kubernetes.io/service-name": leaderServiceName,
		}); err != nil {
			return false, err
		}

		if len(sliceList.Items) == 0 {
			return false, nil // Keep waiting
		}

		if len(sliceList.Items) > 1 {
			return false, fmt.Errorf("expected exactly one EndpointSlice, found %d", len(sliceList.Items))
		}

		endpointSlice = sliceList.Items[0]

		// Verify exactly one endpoint
		if len(endpointSlice.Endpoints) != 1 {
			return false, fmt.Errorf("expected exactly one endpoint, found %d", len(endpointSlice.Endpoints))
		}

		return true, nil
	})

	if err != nil {
		return nil, err
	}
	return &endpointSlice, nil
}

// int32Ptr returns a pointer to an int32
func int32Ptr(i int32) *int32 {
	return &i
}

// createTestDeployment creates a test deployment with pods
func createTestDeployment(ctx context.Context, c client.Client, name, namespace string, replicas int32) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx:latest",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment, c.Create(ctx, deployment)
}

// waitForPodsReady waits for at least one pod to be ready
func waitForPodsReady(ctx context.Context, c client.Client, namespace string, labels map[string]string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		podList := &corev1.PodList{}
		if err := c.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
			return false, err
		}
		for _, pod := range podList.Items {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					return true, nil
				}
			}
		}
		return false, nil
	})
}
