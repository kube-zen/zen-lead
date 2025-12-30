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
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AnnotationPool is the annotation key for the pool name
	AnnotationPool = "zen-lead/pool"
	// AnnotationRole is the annotation key for the role (leader/follower)
	AnnotationRole = "zen-lead/role"
	// RoleLeader indicates this pod is the leader
	RoleLeader = "leader"
	// ServiceSuffix is the suffix for the active service name
	ServiceSuffix = "-active"
)

// DirectorReconciler reconciles Deployments to route traffic to leader pods
// via Service endpoints. It watches Deployments with zen-lead/pool annotation
// and updates a corresponding Service to route traffic exclusively to the Leader Pod.
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

// Reconcile watches a Deployment and updates a Service to route traffic to the leader pod
func (r *DirectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := klog.FromContext(ctx)
	logger = logger.WithValues("deployment", req.NamespacedName)

	// Fetch the Deployment
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deployment); err != nil {
		// Deployment not found, ignore
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if this deployment participates in leader election
	poolName, hasPool := deployment.Annotations[AnnotationPool]
	if !hasPool {
		// Not participating in leader election, skip
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("pool", poolName)

	// List all Pods belonging to this Deployment
	podList := &corev1.PodList{}
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		logger.Error(err, "Failed to create pod selector")
		return ctrl.Result{}, err
	}

	if err := r.List(ctx, podList, client.InNamespace(req.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		logger.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	if len(podList.Items) == 0 {
		logger.Info("No pods found for deployment")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Find the leader pod
	leaderPod := r.selectLeaderPod(podList.Items)
	if leaderPod == nil {
		logger.Info("No leader pod found, waiting for leader election")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger = logger.WithValues("leader_pod", leaderPod.Name)

	// Find or create the Service
	serviceName := deployment.Name + ServiceSuffix
	service := &corev1.Service{}
	serviceKey := types.NamespacedName{
		Name:      serviceName,
		Namespace: req.Namespace,
	}

	if err := r.Get(ctx, serviceKey, service); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get service")
			return ctrl.Result{}, err
		}
		// Service doesn't exist, create it
		service = r.createService(deployment, serviceName, leaderPod)
		if err := r.Create(ctx, service); err != nil {
			logger.Error(err, "Failed to create service")
			return ctrl.Result{}, err
		}
		logger.Info("Created service for leader pod")
		// Continue to create/update endpoints
	}

	// Update Endpoints resource to point only to leader pod
	endpoints := &corev1.Endpoints{}
	endpointsKey := types.NamespacedName{
		Name:      serviceName,
		Namespace: req.Namespace,
	}

	if err := r.Get(ctx, endpointsKey, endpoints); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get endpoints")
			return ctrl.Result{}, err
		}
		// Endpoints don't exist, create them
		endpoints = r.createEndpoints(serviceName, req.Namespace, leaderPod, service.Spec.Ports)
		if err := r.Create(ctx, endpoints); err != nil {
			logger.Error(err, "Failed to create endpoints")
			return ctrl.Result{}, err
		}
		logger.Info("Created endpoints for leader pod")
	} else {
		// Update existing endpoints
		originalEndpoints := endpoints.DeepCopy()
		endpoints.Subsets = []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: leaderPod.Status.PodIP,
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Namespace: leaderPod.Namespace,
							Name:      leaderPod.Name,
							UID:       leaderPod.UID,
						},
					},
				},
				Ports: r.getEndpointPorts(service.Spec.Ports),
			},
		}
		if err := r.Patch(ctx, endpoints, client.MergeFrom(originalEndpoints)); err != nil {
			logger.Error(err, "Failed to patch endpoints")
			return ctrl.Result{}, err
		}
		logger.Info("Updated endpoints to route traffic to leader pod")
	}

	logger.Info("Updated service to route traffic to leader pod",
		"leader_pod", leaderPod.Name,
		"service", serviceName,
	)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// selectLeaderPod selects the leader pod from a list of pods
// Uses a deterministic algorithm: picks the pod with zen-lead/role=leader annotation,
// or if none found, picks the one with the oldest creation timestamp.
func (r *DirectorReconciler) selectLeaderPod(pods []corev1.Pod) *corev1.Pod {
	// First, try to find a pod with leader annotation
	for i := range pods {
		if role, exists := pods[i].Annotations[AnnotationRole]; exists && role == RoleLeader {
			return &pods[i]
		}
	}

	// No leader annotation found, use deterministic selection (oldest pod)
	if len(pods) == 0 {
		return nil
	}

	// Sort pods by creation timestamp (oldest first)
	sortedPods := make([]corev1.Pod, len(pods))
	copy(sortedPods, pods)
	sort.Slice(sortedPods, func(i, j int) bool {
		return sortedPods[i].CreationTimestamp.Before(&sortedPods[j].CreationTimestamp)
	})

	return &sortedPods[0]
}

// createService creates a new Service for the deployment
func (r *DirectorReconciler) createService(deployment *appsv1.Deployment, serviceName string, leaderPod *corev1.Pod) *corev1.Service {
	// Determine service port (default to 8080 if not specified)
	port := int32(8080)
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		if len(deployment.Spec.Template.Spec.Containers[0].Ports) > 0 {
			port = deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort
		}
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: deployment.Namespace,
			Labels:    deployment.Labels,
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
			Selector: deployment.Spec.Selector.MatchLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return service
}

// createEndpoints creates a new Endpoints resource for the service
func (r *DirectorReconciler) createEndpoints(serviceName, namespace string, leaderPod *corev1.Pod, servicePorts []corev1.ServicePort) *corev1.Endpoints {
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: leaderPod.Status.PodIP,
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Namespace: leaderPod.Namespace,
							Name:      leaderPod.Name,
							UID:       leaderPod.UID,
						},
					},
				},
				Ports: r.getEndpointPorts(servicePorts),
			},
		},
	}
	return endpoints
}

// getEndpointPorts converts ServicePorts to EndpointPorts
func (r *DirectorReconciler) getEndpointPorts(servicePorts []corev1.ServicePort) []corev1.EndpointPort {
	endpointPorts := make([]corev1.EndpointPort, 0, len(servicePorts))
	for _, sp := range servicePorts {
		ep := corev1.EndpointPort{
			Name:     sp.Name,
			Port:     sp.Port,
			Protocol: sp.Protocol,
		}
		endpointPorts = append(endpointPorts, ep)
	}
	return endpointPorts
}

// SetupWithManager sets up the DirectorReconciler with the manager
func (r *DirectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}
