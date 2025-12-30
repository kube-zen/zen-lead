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
	"fmt"
	"sort"
	"time"

	coordinationv1alpha1 "github.com/kube-zen/zen-lead/pkg/apis/coordination.kube-zen.io/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// LabelPool is the label key for the pool name
	LabelPool = "zen-lead/pool"
	// ServiceSuffix is the suffix for the leader service name
	ServiceSuffix = "-leader"
	// AnnotationLeaderServiceName allows specifying custom leader service name
	AnnotationLeaderServiceName = "zen-lead.io/leader-service-name"
	// AnnotationEnabled enables zen-lead for a Service
	AnnotationEnabled = "zen-lead.io/enabled"
)

// DirectorReconciler reconciles LeaderPolicy resources to route traffic to leader pods
// via a selector-less Service with controller-managed EndpointSlice.
// This approach is non-invasive: it does not mutate workload pods or interfere with existing Services.
type DirectorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewDirectorReconciler creates a new DirectorReconciler
func NewDirectorReconciler(client client.Client, scheme *runtime.Scheme) *DirectorReconciler {
	return &DirectorReconciler{
		Client: client,
		Scheme: scheme,
	}
}

// Reconcile watches a LeaderPolicy and routes traffic to the leader pod via selector-less Service + EndpointSlice
func (r *DirectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := klog.FromContext(ctx)
	logger = logger.WithValues("leaderpolicy", req.NamespacedName)

	// Fetch the LeaderPolicy
	policy := &coordinationv1alpha1.LeaderPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		// LeaderPolicy not found, ignore
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger = logger.WithValues("pool", policy.Name)

	// Find all Deployments with this pool label
	deploymentList := &appsv1.DeploymentList{}
	if err := r.List(ctx, deploymentList, client.InNamespace(req.Namespace), client.MatchingLabels{LabelPool: policy.Name}); err != nil {
		logger.Error(err, "Failed to list deployments")
		return ctrl.Result{}, err
	}

	// Also check for Services with zen-lead.io/enabled annotation
	serviceList := &corev1.ServiceList{}
	if err := r.List(ctx, serviceList, client.InNamespace(req.Namespace)); err != nil {
		logger.Error(err, "Failed to list services")
		return ctrl.Result{}, err
	}

	var servicesToProcess []*corev1.Service
	for i := range serviceList.Items {
		svc := &serviceList.Items[i]
		if svc.Annotations != nil && svc.Annotations[AnnotationEnabled] == "true" {
			// Check if this service references our pool (via annotation or label)
			poolName := ""
			if svc.Annotations != nil {
				poolName = svc.Annotations[LabelPool]
			}
			if poolName == "" && svc.Labels != nil {
				poolName = svc.Labels[LabelPool]
			}
			if poolName == policy.Name {
				servicesToProcess = append(servicesToProcess, svc)
			}
		}
	}

	// Process each deployment in the pool
	for i := range deploymentList.Items {
		deployment := &deploymentList.Items[i]

		// Smart Default: If replicas > 1, assume HA is desired
		if !ShouldEnableHA(*deployment.Spec.Replicas) {
			logger.V(4).Info("Skipping HA setup: single replica deployment",
				"deployment", deployment.Name,
				"replicas", *deployment.Spec.Replicas)
			continue
		}

		// Find the existing Service for this deployment (if any)
		var sourceService *corev1.Service
		// Try to find a Service that matches the deployment selector
		for j := range serviceList.Items {
			svc := &serviceList.Items[j]
			if svc.Namespace == deployment.Namespace {
				// Check if service selector matches deployment selector
				if labelsMatch(svc.Spec.Selector, deployment.Spec.Selector.MatchLabels) {
					sourceService = svc
					break
				}
			}
		}

		if err := r.reconcileDeployment(ctx, deployment, policy, sourceService, logger); err != nil {
			logger.Error(err, "Failed to reconcile deployment", "deployment", deployment.Name)
			// Continue with other deployments
			continue
		}
	}

	// Process Services with zen-lead.io/enabled annotation
	for _, svc := range servicesToProcess {
		if err := r.reconcileService(ctx, svc, policy, logger); err != nil {
			logger.Error(err, "Failed to reconcile service", "service", svc.Name)
			continue
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// reconcileDeployment reconciles a single deployment for the pool
func (r *DirectorReconciler) reconcileDeployment(ctx context.Context, deployment *appsv1.Deployment, policy *coordinationv1alpha1.LeaderPolicy, sourceService *corev1.Service, logger klog.Logger) error {
	// List all Pods belonging to this Deployment
	podList := &corev1.PodList{}
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return fmt.Errorf("failed to create pod selector: %w", err)
	}

	if err := r.List(ctx, podList, client.InNamespace(deployment.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		logger.Info("No pods found for deployment", "deployment", deployment.Name)
		// Clear leader service endpoints if no pods exist
		return r.clearLeaderService(ctx, deployment, policy, logger)
	}

	// Select leader pod using controller-driven selection (no pod mutation)
	leaderPod := r.selectLeaderPod(podList.Items)

	if leaderPod == nil {
		logger.Info("No ready leader pod found for deployment", "deployment", deployment.Name)
		// Clear leader service endpoints
		return r.clearLeaderService(ctx, deployment, policy, logger)
	}

	logger = logger.WithValues("leader_pod", leaderPod.Name, "deployment", deployment.Name)

	// Determine leader service name
	leaderServiceName := r.getLeaderServiceName(deployment, policy)

	// Determine ports from source service or deployment
	ports := r.getServicePorts(sourceService, deployment)

	// Create or update the selector-less leader Service
	if err := r.reconcileLeaderService(ctx, deployment, policy, leaderServiceName, ports, logger); err != nil {
		return fmt.Errorf("failed to reconcile leader service: %w", err)
	}

	// Create or update EndpointSlice pointing to leader pod
	if err := r.reconcileEndpointSlice(ctx, deployment, policy, leaderServiceName, leaderPod, ports, logger); err != nil {
		return fmt.Errorf("failed to reconcile endpoint slice: %w", err)
	}

	logger.Info("Traffic routed to leader pod",
		"pod", leaderPod.Name,
		"pool", policy.Name,
		"service", leaderServiceName,
	)

	return nil
}

// reconcileService reconciles a Service with zen-lead.io/enabled annotation
func (r *DirectorReconciler) reconcileService(ctx context.Context, svc *corev1.Service, policy *coordinationv1alpha1.LeaderPolicy, logger klog.Logger) error {
	// Find pods matching the service selector
	podList := &corev1.PodList{}
	if len(svc.Spec.Selector) == 0 {
		logger.Info("Service has no selector, skipping", "service", svc.Name)
		return nil
	}

	if err := r.List(ctx, podList, client.InNamespace(svc.Namespace), client.MatchingLabels(svc.Spec.Selector)); err != nil {
		return fmt.Errorf("failed to list pods for service: %w", err)
	}

	if len(podList.Items) == 0 {
		logger.Info("No pods found for service", "service", svc.Name)
		// Clear leader service endpoints
		return r.clearLeaderServiceForService(ctx, svc, policy, logger)
	}

	// Select leader pod
	leaderPod := r.selectLeaderPod(podList.Items)
	if leaderPod == nil {
		logger.Info("No ready leader pod found for service", "service", svc.Name)
		return r.clearLeaderServiceForService(ctx, svc, policy, logger)
	}

	// Determine leader service name
	leaderServiceName := r.getLeaderServiceNameForService(svc)

	// Use service ports
	ports := svc.Spec.Ports

	// Create or update selector-less leader Service
	if err := r.reconcileLeaderServiceForService(ctx, svc, policy, leaderServiceName, ports, logger); err != nil {
		return fmt.Errorf("failed to reconcile leader service: %w", err)
	}

	// Create or update EndpointSlice
	if err := r.reconcileEndpointSliceForService(ctx, svc, policy, leaderServiceName, leaderPod, ports, logger); err != nil {
		return fmt.Errorf("failed to reconcile endpoint slice: %w", err)
	}

	logger.Info("Traffic routed to leader pod via service",
		"pod", leaderPod.Name,
		"service", svc.Name,
		"leader_service", leaderServiceName,
	)

	return nil
}

// selectLeaderPod selects the leader pod using controller-driven selection
// Strategy: If current leader (from previous EndpointSlice) is still Ready, keep it.
// Otherwise, select oldest Ready pod (stable, predictable).
func (r *DirectorReconciler) selectLeaderPod(pods []corev1.Pod) *corev1.Pod {
	// Filter to Ready pods only
	var readyPods []corev1.Pod
	for _, pod := range pods {
		if isPodReady(&pod) {
			readyPods = append(readyPods, pod)
		}
	}

	if len(readyPods) == 0 {
		return nil
	}

	// Sort by creation timestamp (oldest first), then by name (lexical) as tie-breaker
	sort.Slice(readyPods, func(i, j int) bool {
		if !readyPods[i].CreationTimestamp.Equal(&readyPods[j].CreationTimestamp) {
			return readyPods[i].CreationTimestamp.Before(&readyPods[j].CreationTimestamp)
		}
		return readyPods[i].Name < readyPods[j].Name
	})

	// Return oldest Ready pod
	return &readyPods[0]
}

// isPodReady checks if a pod is Ready
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// reconcileLeaderService creates or updates a selector-less Service
func (r *DirectorReconciler) reconcileLeaderService(ctx context.Context, deployment *appsv1.Deployment, policy *coordinationv1alpha1.LeaderPolicy, serviceName string, ports []corev1.ServicePort, logger klog.Logger) error {
	service := &corev1.Service{}
	serviceKey := types.NamespacedName{
		Name:      serviceName,
		Namespace: deployment.Namespace,
	}

	if err := r.Get(ctx, serviceKey, service); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get service: %w", err)
		}
		// Service doesn't exist, create it
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: deployment.Namespace,
				Labels: map[string]string{
					LabelPool: policy.Name,
					"zen-lead.io/managed": "true",
					"zen-lead.io/for":     deployment.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       deployment.Name,
						UID:        deployment.UID,
						Controller: func() *bool { b := true; return &b }(),
					},
				},
			},
			Spec: corev1.ServiceSpec{
				// CRITICAL: No selector - we manage endpoints manually via EndpointSlice
				Selector: nil,
				Ports:    ports,
				Type:      corev1.ServiceTypeClusterIP,
			},
		}

		if err := r.Create(ctx, service); err != nil {
			return fmt.Errorf("failed to create leader service: %w", err)
		}
		logger.Info("Created selector-less leader service", "service", serviceName)
		return nil
	}

	// Service exists, ensure it has no selector and ports are correct
	originalService := service.DeepCopy()
	service.Spec.Selector = nil // Ensure no selector
	service.Spec.Ports = ports   // Update ports

	if err := r.Patch(ctx, service, client.MergeFrom(originalService)); err != nil {
		return fmt.Errorf("failed to patch leader service: %w", err)
	}

	logger.V(4).Info("Updated leader service", "service", serviceName)
	return nil
}

// reconcileLeaderServiceForService creates/updates leader service for an annotated Service
func (r *DirectorReconciler) reconcileLeaderServiceForService(ctx context.Context, svc *corev1.Service, policy *coordinationv1alpha1.LeaderPolicy, leaderServiceName string, ports []corev1.ServicePort, logger klog.Logger) error {
	leaderService := &corev1.Service{}
	serviceKey := types.NamespacedName{
		Name:      leaderServiceName,
		Namespace: svc.Namespace,
	}

	if err := r.Get(ctx, serviceKey, leaderService); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get leader service: %w", err)
		}
		// Create selector-less leader service
		leaderService = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      leaderServiceName,
				Namespace: svc.Namespace,
				Labels: map[string]string{
					LabelPool: policy.Name,
					"zen-lead.io/managed": "true",
					"zen-lead.io/for":     svc.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       svc.Name,
						UID:        svc.UID,
						Controller: func() *bool { b := true; return &b }(),
					},
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: nil, // No selector
				Ports:    ports,
				Type:      corev1.ServiceTypeClusterIP,
			},
		}

		if err := r.Create(ctx, leaderService); err != nil {
			return fmt.Errorf("failed to create leader service: %w", err)
		}
		logger.Info("Created selector-less leader service", "service", leaderServiceName)
		return nil
	}

	// Update existing service
	originalService := leaderService.DeepCopy()
	leaderService.Spec.Selector = nil
	leaderService.Spec.Ports = ports

	if err := r.Patch(ctx, leaderService, client.MergeFrom(originalService)); err != nil {
		return fmt.Errorf("failed to patch leader service: %w", err)
	}

	return nil
}

// reconcileEndpointSlice creates or updates an EndpointSlice pointing to the leader pod
func (r *DirectorReconciler) reconcileEndpointSlice(ctx context.Context, deployment *appsv1.Deployment, policy *coordinationv1alpha1.LeaderPolicy, serviceName string, leaderPod *corev1.Pod, ports []corev1.ServicePort, logger klog.Logger) error {
	endpointSliceName := serviceName
	endpointSlice := &discoveryv1.EndpointSlice{}
	endpointSliceKey := types.NamespacedName{
		Name:      endpointSliceName,
		Namespace: deployment.Namespace,
	}

	// Convert ServicePorts to EndpointPorts
	endpointPorts := make([]discoveryv1.EndpointPort, len(ports))
	for i, port := range ports {
		endpointPorts[i] = discoveryv1.EndpointPort{
			Name:     &port.Name,
			Port:     &port.Port,
			Protocol: &port.Protocol,
		}
	}

	// Build endpoint from pod
	var endpointAddresses []string
	var nodeName *string
	var targetRef *corev1.ObjectReference

	if leaderPod.Status.PodIP != "" {
		endpointAddresses = []string{leaderPod.Status.PodIP}
		if leaderPod.Spec.NodeName != "" {
			nodeName = &leaderPod.Spec.NodeName
		}
		targetRef = &corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: leaderPod.Namespace,
			Name:      leaderPod.Name,
			UID:       leaderPod.UID,
		}
	}

	endpoint := discoveryv1.Endpoint{
		Addresses: endpointAddresses,
		Conditions: discoveryv1.EndpointConditions{
			Ready: func() *bool { b := true; return &b }(),
		},
		NodeName:  nodeName,
		TargetRef: targetRef,
	}

	if err := r.Get(ctx, endpointSliceKey, endpointSlice); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get endpoint slice: %w", err)
		}
		// EndpointSlice doesn't exist, create it
		endpointSlice = &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      endpointSliceName,
				Namespace: deployment.Namespace,
				Labels: map[string]string{
					discoveryv1.LabelServiceName: serviceName,
					LabelPool:                     policy.Name,
					"zen-lead.io/managed":        "true",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       deployment.Name,
						UID:        deployment.UID,
						Controller: func() *bool { b := true; return &b }(),
					},
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   []discoveryv1.Endpoint{endpoint},
			Ports:       endpointPorts,
		}

		if err := r.Create(ctx, endpointSlice); err != nil {
			return fmt.Errorf("failed to create endpoint slice: %w", err)
		}
		logger.Info("Created endpoint slice for leader pod", "endpointslice", endpointSliceName, "pod", leaderPod.Name)
		return nil
	}

	// EndpointSlice exists, update it
	originalEndpointSlice := endpointSlice.DeepCopy()
	endpointSlice.Endpoints = []discoveryv1.Endpoint{endpoint}
	endpointSlice.Ports = endpointPorts

	if err := r.Patch(ctx, endpointSlice, client.MergeFrom(originalEndpointSlice)); err != nil {
		return fmt.Errorf("failed to patch endpoint slice: %w", err)
	}

	logger.V(4).Info("Updated endpoint slice for leader pod", "endpointslice", endpointSliceName, "pod", leaderPod.Name)
	return nil
}

// reconcileEndpointSliceForService creates/updates EndpointSlice for an annotated Service
func (r *DirectorReconciler) reconcileEndpointSliceForService(ctx context.Context, svc *corev1.Service, policy *coordinationv1alpha1.LeaderPolicy, leaderServiceName string, leaderPod *corev1.Pod, ports []corev1.ServicePort, logger klog.Logger) error {
	endpointSliceName := leaderServiceName
	endpointSlice := &discoveryv1.EndpointSlice{}
	endpointSliceKey := types.NamespacedName{
		Name:      endpointSliceName,
		Namespace: svc.Namespace,
	}

	// Convert ServicePorts to EndpointPorts
	endpointPorts := make([]discoveryv1.EndpointPort, len(ports))
	for i, port := range ports {
		endpointPorts[i] = discoveryv1.EndpointPort{
			Name:     &port.Name,
			Port:     &port.Port,
			Protocol: &port.Protocol,
		}
	}

	// Build endpoint from pod
	var endpointAddresses []string
	var nodeName *string
	var targetRef *corev1.ObjectReference

	if leaderPod.Status.PodIP != "" {
		endpointAddresses = []string{leaderPod.Status.PodIP}
		if leaderPod.Spec.NodeName != "" {
			nodeName = &leaderPod.Spec.NodeName
		}
		targetRef = &corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: leaderPod.Namespace,
			Name:      leaderPod.Name,
			UID:       leaderPod.UID,
		}
	}

	endpoint := discoveryv1.Endpoint{
		Addresses: endpointAddresses,
		Conditions: discoveryv1.EndpointConditions{
			Ready: func() *bool { b := true; return &b }(),
		},
		NodeName:  nodeName,
		TargetRef: targetRef,
	}

	if err := r.Get(ctx, endpointSliceKey, endpointSlice); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get endpoint slice: %w", err)
		}
		// Create EndpointSlice
		endpointSlice = &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      endpointSliceName,
				Namespace: svc.Namespace,
				Labels: map[string]string{
					discoveryv1.LabelServiceName: leaderServiceName,
					LabelPool:                     policy.Name,
					"zen-lead.io/managed":        "true",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       svc.Name,
						UID:        svc.UID,
						Controller: func() *bool { b := true; return &b }(),
					},
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   []discoveryv1.Endpoint{endpoint},
			Ports:       endpointPorts,
		}

		if err := r.Create(ctx, endpointSlice); err != nil {
			return fmt.Errorf("failed to create endpoint slice: %w", err)
		}
		logger.Info("Created endpoint slice for leader pod", "endpointslice", endpointSliceName, "pod", leaderPod.Name)
		return nil
	}

	// Update existing EndpointSlice
	originalEndpointSlice := endpointSlice.DeepCopy()
	endpointSlice.Endpoints = []discoveryv1.Endpoint{endpoint}
	endpointSlice.Ports = endpointPorts

	if err := r.Patch(ctx, endpointSlice, client.MergeFrom(originalEndpointSlice)); err != nil {
		return fmt.Errorf("failed to patch endpoint slice: %w", err)
	}

	return nil
}

// clearLeaderService clears the leader service endpoints when no leader is available
func (r *DirectorReconciler) clearLeaderService(ctx context.Context, deployment *appsv1.Deployment, policy *coordinationv1alpha1.LeaderPolicy, logger klog.Logger) error {
	leaderServiceName := r.getLeaderServiceName(deployment, policy)
	endpointSliceName := leaderServiceName
	endpointSlice := &discoveryv1.EndpointSlice{}
	endpointSliceKey := types.NamespacedName{
		Name:      endpointSliceName,
		Namespace: deployment.Namespace,
	}

	if err := r.Get(ctx, endpointSliceKey, endpointSlice); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil // EndpointSlice doesn't exist, nothing to clear
		}
		return err
	}

	// Clear endpoints
	originalEndpointSlice := endpointSlice.DeepCopy()
	endpointSlice.Endpoints = []discoveryv1.Endpoint{}

	if err := r.Patch(ctx, endpointSlice, client.MergeFrom(originalEndpointSlice)); err != nil {
		return fmt.Errorf("failed to clear endpoint slice: %w", err)
	}

	logger.Info("Cleared leader service endpoints", "endpointslice", endpointSliceName)
	return nil
}

// clearLeaderServiceForService clears endpoints for a Service-based leader service
func (r *DirectorReconciler) clearLeaderServiceForService(ctx context.Context, svc *corev1.Service, policy *coordinationv1alpha1.LeaderPolicy, logger klog.Logger) error {
	leaderServiceName := r.getLeaderServiceNameForService(svc)
	endpointSliceName := leaderServiceName
	endpointSlice := &discoveryv1.EndpointSlice{}
	endpointSliceKey := types.NamespacedName{
		Name:      endpointSliceName,
		Namespace: svc.Namespace,
	}

	if err := r.Get(ctx, endpointSliceKey, endpointSlice); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil
		}
		return err
	}

	originalEndpointSlice := endpointSlice.DeepCopy()
	endpointSlice.Endpoints = []discoveryv1.Endpoint{}

	if err := r.Patch(ctx, endpointSlice, client.MergeFrom(originalEndpointSlice)); err != nil {
		return fmt.Errorf("failed to clear endpoint slice: %w", err)
	}

	return nil
}

// getLeaderServiceName determines the leader service name
func (r *DirectorReconciler) getLeaderServiceName(deployment *appsv1.Deployment, policy *coordinationv1alpha1.LeaderPolicy) string {
	// Check for custom service name annotation
	if deployment.Annotations != nil {
		if customName := deployment.Annotations[AnnotationLeaderServiceName]; customName != "" {
			return customName
		}
	}
	// Default: <deployment-name>-leader
	return deployment.Name + ServiceSuffix
}

// getLeaderServiceNameForService determines leader service name for an annotated Service
func (r *DirectorReconciler) getLeaderServiceNameForService(svc *corev1.Service) string {
	if svc.Annotations != nil {
		if customName := svc.Annotations[AnnotationLeaderServiceName]; customName != "" {
			return customName
		}
	}
	// Default: <service-name>-leader
	return svc.Name + ServiceSuffix
}

// getServicePorts extracts ports from source service or deployment
func (r *DirectorReconciler) getServicePorts(sourceService *corev1.Service, deployment *appsv1.Deployment) []corev1.ServicePort {
	// Prefer ports from source service
	if sourceService != nil && len(sourceService.Spec.Ports) > 0 {
		return sourceService.Spec.Ports
	}

	// Fallback to deployment container ports
	ports := []corev1.ServicePort{}
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		container := deployment.Spec.Template.Spec.Containers[0]
		for _, containerPort := range container.Ports {
			port := corev1.ServicePort{
				Name:       containerPort.Name,
				Port:       containerPort.ContainerPort,
				TargetPort: intstr.FromInt32(containerPort.ContainerPort),
				Protocol:   containerPort.Protocol,
			}
			if port.Name == "" {
				port.Name = fmt.Sprintf("port-%d", containerPort.ContainerPort)
			}
			ports = append(ports, port)
		}
	}

	// Default port if none found
	if len(ports) == 0 {
		ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromInt32(8080),
				Protocol:   corev1.ProtocolTCP,
			},
		}
	}

	return ports
}

// labelsMatch checks if two label maps match
func labelsMatch(selector map[string]string, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// SetupWithManager sets up the DirectorReconciler with the manager
func (r *DirectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&coordinationv1alpha1.LeaderPolicy{}).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.mapDeploymentToPolicy),
		).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(r.mapServiceToPolicy),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.mapPodToPolicy),
		).
		Complete(r)
}

// mapDeploymentToPolicy maps a Deployment to LeaderPolicy requests
func (r *DirectorReconciler) mapDeploymentToPolicy(ctx context.Context, obj client.Object) []reconcile.Request {
	deployment := obj.(*appsv1.Deployment)
	poolName, exists := deployment.Labels[LabelPool]
	if !exists {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      poolName,
				Namespace: deployment.Namespace,
			},
		},
	}
}

// mapServiceToPolicy maps a Service with zen-lead.io/enabled annotation to LeaderPolicy requests
func (r *DirectorReconciler) mapServiceToPolicy(ctx context.Context, obj client.Object) []reconcile.Request {
	svc := obj.(*corev1.Service)
	if svc.Annotations == nil || svc.Annotations[AnnotationEnabled] != "true" {
		return nil
	}

	poolName := ""
	if svc.Annotations != nil {
		poolName = svc.Annotations[LabelPool]
	}
	if poolName == "" && svc.Labels != nil {
		poolName = svc.Labels[LabelPool]
	}

	if poolName == "" {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      poolName,
				Namespace: svc.Namespace,
			},
		},
	}
}

// mapPodToPolicy maps Pod changes to LeaderPolicy requests (for failover detection)
func (r *DirectorReconciler) mapPodToPolicy(ctx context.Context, obj client.Object) []reconcile.Request {
	pod := obj.(*corev1.Pod)

	// Check if pod belongs to a deployment with zen-lead/pool label
	// We need to find the deployment that owns this pod
	deploymentList := &appsv1.DeploymentList{}
	if err := r.List(ctx, deploymentList, client.InNamespace(pod.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for i := range deploymentList.Items {
		deployment := &deploymentList.Items[i]
		if deployment.Labels != nil {
			poolName, exists := deployment.Labels[LabelPool]
			if !exists {
				continue
			}
			// Check if pod matches deployment selector
			selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
			if err != nil {
				continue
			}
			if selector.Matches(labels.Set(pod.Labels)) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      poolName,
						Namespace: pod.Namespace,
					},
				})
			}
		}
	}

	return requests
}

