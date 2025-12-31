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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kube-zen/zen-lead/pkg/metrics"
)

const (
	// AnnotationEnabledService enables zen-lead for a Service
	AnnotationEnabledService = "zen-lead.io/enabled"
	// AnnotationLeaderServiceNameService allows specifying custom leader service name
	AnnotationLeaderServiceNameService = "zen-lead.io/leader-service-name"
	// AnnotationStrategyService specifies leader selection strategy
	AnnotationStrategyService = "zen-lead.io/strategy"
	// AnnotationStickyService enables sticky leader (keep current leader if Ready)
	AnnotationStickyService = "zen-lead.io/sticky"
	// AnnotationFailoverMinDelayService specifies minimum delay before failover
	AnnotationFailoverMinDelayService = "zen-lead.io/failover-min-delay"
	// AnnotationPortsModeService specifies how to handle ports
	AnnotationPortsModeService = "zen-lead.io/ports-mode"
	// AnnotationMinReadyDurationService specifies minimum duration pod must be Ready before becoming leader
	AnnotationMinReadyDurationService = "zen-lead.io/min-ready-duration"

	// ServiceSuffixService is the suffix for the leader service name
	ServiceSuffixService = "-leader"

	// AnnotationLeaderPodName is set on leader Service to track current leader pod name
	AnnotationLeaderPodName = "zen-lead.io/leader-pod-name"
	// AnnotationLeaderPodUID is set on leader Service to track current leader pod UID
	AnnotationLeaderPodUID = "zen-lead.io/leader-pod-uid"
	// AnnotationLeaderLastSwitchTime is set on leader Service to track when leader last changed
	AnnotationLeaderLastSwitchTime = "zen-lead.io/leader-last-switch-time"

	// LabelManagedBy marks resources managed by zen-lead
	LabelManagedBy      = "app.kubernetes.io/managed-by"
	LabelManagedByValue = "zen-lead"
	// LabelSourceService marks the source Service
	LabelSourceService = "zen-lead.io/source-service"
	// LabelEndpointSliceManagedBy marks EndpointSlice as managed by zen-lead
	LabelEndpointSliceManagedBy      = "endpointslice.kubernetes.io/managed-by"
	LabelEndpointSliceManagedByValue = "zen-lead"
)

// GitOps tracking labels/annotations that should NOT be copied to generated resources
// These are common GitOps tool labels that would cause ownership/prune conflicts
var gitOpsTrackingLabels = []string{
	"app.kubernetes.io/instance",
	"app.kubernetes.io/managed-by", // We set our own value
	"app.kubernetes.io/part-of",
	"app.kubernetes.io/version",
	"argocd.argoproj.io/instance",
	"fluxcd.io/part-of",
	"kustomize.toolkit.fluxcd.io/name",
	"kustomize.toolkit.fluxcd.io/namespace",
	"kustomize.toolkit.fluxcd.io/revision",
}

var gitOpsTrackingAnnotations = []string{
	"argocd.argoproj.io/sync-wave",
	"argocd.argoproj.io/sync-options",
	"fluxcd.io/sync-checksum",
	"kustomize.toolkit.fluxcd.io/checksum",
}

// filterGitOpsLabels removes GitOps tracking labels from a label map
func filterGitOpsLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return make(map[string]string)
	}
	filtered := make(map[string]string)
	for k, v := range labels {
		skip := false
		for _, gitOpsLabel := range gitOpsTrackingLabels {
			if k == gitOpsLabel {
				skip = true
				break
			}
		}
		if !skip {
			filtered[k] = v
		}
	}
	return filtered
}

// filterGitOpsAnnotations removes GitOps tracking annotations from an annotation map
func filterGitOpsAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		return make(map[string]string)
	}
	filtered := make(map[string]string)
	for k, v := range annotations {
		skip := false
		for _, gitOpsAnnotation := range gitOpsTrackingAnnotations {
			if k == gitOpsAnnotation {
				skip = true
				break
			}
		}
		if !skip {
			filtered[k] = v
		}
	}
	return filtered
}

// ServiceDirectorReconciler reconciles Services with zen-lead.io/enabled annotation
// to route traffic to leader pods via selector-less Service + EndpointSlice.
// This is the day-0 non-invasive approach: no CRD required, no pod mutation.
type ServiceDirectorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Metrics  *metrics.Recorder

	// optedInServicesCache caches opted-in Services per namespace for efficient pod-to-service mapping
	// key: namespace, value: list of Service names with compiled selectors
	optedInServicesCache map[string][]*cachedService
}

// cachedService holds a Service's selector for efficient matching
type cachedService struct {
	name     string
	selector labels.Selector
}

// NewServiceDirectorReconciler creates a new ServiceDirectorReconciler
func NewServiceDirectorReconciler(client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder) *ServiceDirectorReconciler {
	return &ServiceDirectorReconciler{
		Client:               client,
		Scheme:               scheme,
		Recorder:             recorder,
		Metrics:              metrics.NewRecorder(),
		optedInServicesCache: make(map[string][]*cachedService),
	}
}

// Reconcile reconciles a Service with zen-lead.io/enabled annotation
func (r *ServiceDirectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	logger := klog.FromContext(ctx)
	logger = logger.WithValues("service", req.NamespacedName)

	// Fetch the Service
	svc := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, svc); err != nil {
		// Update cache on Service deletion
		r.updateOptedInServicesCache(ctx, req.Namespace, logger)
		// Service not found - cleanup leader resources
		if client.IgnoreNotFound(err) == nil {
			result, err := r.cleanupLeaderResources(ctx, req.NamespacedName, logger)
			duration := time.Since(startTime).Seconds()
			if r.Metrics != nil {
				r.Metrics.RecordReconciliationDuration(req.Namespace, req.Name, "success", duration)
			}
			return result, err
		}
		duration := time.Since(startTime).Seconds()
		if r.Metrics != nil {
			r.Metrics.RecordReconciliationDuration(req.Namespace, req.Name, "error", duration)
			r.Metrics.RecordReconciliationError(req.Namespace, req.Name, "service_not_found")
		}
		return ctrl.Result{}, err
	}

	// Check if zen-lead is enabled for this Service
	if svc.Annotations == nil || svc.Annotations[AnnotationEnabledService] != "true" {
		// Annotation removed - cleanup leader resources
		result, err := r.cleanupLeaderResources(ctx, req.NamespacedName, logger)
		duration := time.Since(startTime).Seconds()
		if r.Metrics != nil {
			r.Metrics.RecordReconciliationDuration(req.Namespace, svc.Name, "success", duration)
		}
		return result, err
	}

	// Validate Service has selector (required for finding pods)
	if len(svc.Spec.Selector) == 0 {
		logger.Info("Service has no selector, skipping", "service", svc.Name)
		r.Recorder.Event(svc, corev1.EventTypeWarning, "InvalidService",
			"Service must have a selector to use zen-lead")
		duration := time.Since(startTime).Seconds()
		if r.Metrics != nil {
			r.Metrics.RecordReconciliationDuration(svc.Namespace, svc.Name, "success", duration)
		}
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("service", svc.Name, "namespace", svc.Namespace)

	// Find pods matching the Service selector
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(svc.Namespace), client.MatchingLabels(svc.Spec.Selector)); err != nil {
		logger.Error(err, "Failed to list pods for service")
		duration := time.Since(startTime).Seconds()
		if r.Metrics != nil {
			r.Metrics.RecordReconciliationDuration(svc.Namespace, svc.Name, "error", duration)
			r.Metrics.RecordReconciliationError(svc.Namespace, svc.Name, "list_pods_failed")
		}
		return ctrl.Result{}, err
	}

	// Count Ready pods for metrics
	readyPods := 0
	for i := range podList.Items {
		if isPodReady(&podList.Items[i]) {
			readyPods++
		}
	}
	if r.Metrics != nil {
		r.Metrics.RecordPodsAvailable(svc.Namespace, svc.Name, readyPods)
	}

	if len(podList.Items) == 0 {
		logger.Info("No pods found for service")
		if err := r.reconcileLeaderService(ctx, svc, nil, logger); err != nil {
			duration := time.Since(startTime).Seconds()
			if r.Metrics != nil {
				r.Metrics.RecordReconciliationDuration(svc.Namespace, svc.Name, "error", duration)
				r.Metrics.RecordReconciliationError(svc.Namespace, svc.Name, "reconcile_service_failed")
			}
			return ctrl.Result{}, err
		}
		duration := time.Since(startTime).Seconds()
		if r.Metrics != nil {
			r.Metrics.RecordReconciliationDuration(svc.Namespace, svc.Name, "success", duration)
		}
		return ctrl.Result{}, nil
	}

	// Get current leader from EndpointSlice (for failover detection)
	currentLeaderPod := r.getCurrentLeaderPod(ctx, svc, logger)

	// Leader-fast-path - immediately failover if current leader is unhealthy
	bypassStickiness := false
	if currentLeaderPod != nil {
		// Check if current leader is terminating, not Ready, or has no PodIP
		if currentLeaderPod.DeletionTimestamp != nil ||
			!isPodReady(currentLeaderPod) ||
			currentLeaderPod.Status.PodIP == "" {
			logger.Info("Current leader unhealthy, triggering immediate failover",
				"leader", currentLeaderPod.Name,
				"terminating", currentLeaderPod.DeletionTimestamp != nil,
				"ready", isPodReady(currentLeaderPod),
				"hasIP", currentLeaderPod.Status.PodIP != "")
			// Force new leader selection (bypass stickiness)
			bypassStickiness = true
			currentLeaderPod = nil
		}
	}

	// Record leader selection attempt
	if r.Metrics != nil {
		r.Metrics.RecordLeaderSelectionAttempt(svc.Namespace, svc.Name)
		r.Metrics.RecordReconciliation(svc.Namespace, svc.Name, "success")
	}

	// Select leader pod (with stickiness, unless current leader was unhealthy)
	leaderPod := r.selectLeaderPod(ctx, svc, podList.Items, bypassStickiness, logger)

	// Detect failover (leader changed) - track leader switch time
	leaderChanged := false
	if currentLeaderPod != nil && leaderPod != nil {
		// Compare by UID for restart safety
		if string(currentLeaderPod.UID) != string(leaderPod.UID) {
			leaderChanged = true
		}
	} else if (currentLeaderPod == nil) != (leaderPod == nil) {
		leaderChanged = true
	}

	if leaderChanged {
		logger.Info("Leader changed", "old_leader", func() string {
			if currentLeaderPod != nil {
				return currentLeaderPod.Name
			}
			return "none"
		}(), "new_leader", func() string {
			if leaderPod != nil {
				return leaderPod.Name
			}
			return "none"
		}())
		if r.Metrics != nil {
			// Record failover with reason
			reason := "noneReady"
			if currentLeaderPod != nil {
				if currentLeaderPod.DeletionTimestamp != nil {
					reason = "terminating"
				} else if !isPodReady(currentLeaderPod) {
					reason = "notReady"
				} else if currentLeaderPod.Status.PodIP == "" {
					reason = "noIP"
				}
			}
			r.Metrics.RecordFailover(svc.Namespace, svc.Name, reason)
			// Reset leader duration (no pod label - leader identity in annotations)
			r.Metrics.ResetLeaderDuration(svc.Namespace, svc.Name)
		}
	}

	// Reconcile leader Service and EndpointSlice
	if err := r.reconcileLeaderService(ctx, svc, leaderPod, logger); err != nil {
		logger.Error(err, "Failed to reconcile leader service")
		duration := time.Since(startTime).Seconds()
		if r.Metrics != nil {
			r.Metrics.RecordReconciliationDuration(svc.Namespace, svc.Name, "error", duration)
			r.Metrics.RecordReconciliationError(svc.Namespace, svc.Name, "reconcile_service_failed")
		}
		return ctrl.Result{}, err
	}

	// Record leader duration and pod age (if leader exists)
	if leaderPod != nil {
		// Calculate duration since pod creation (or since it became leader)
		// For simplicity, use pod creation time. In future, could track leader acquisition time.
		duration := time.Since(leaderPod.CreationTimestamp.Time).Seconds()
		if r.Metrics != nil {
			// Record metrics without pod label (leader identity in annotations)
			r.Metrics.RecordLeaderDuration(svc.Namespace, svc.Name, duration)
			r.Metrics.RecordLeaderPodAge(svc.Namespace, svc.Name, duration)
		}
	} else {
		// No leader - record that service has no endpoints
		if r.Metrics != nil {
			r.Metrics.RecordLeaderServiceWithoutEndpoints(svc.Namespace, svc.Name, true)
		}
	}

	duration := time.Since(startTime).Seconds()
	if r.Metrics != nil {
		r.Metrics.RecordReconciliationDuration(svc.Namespace, svc.Name, "success", duration)
	}
	return ctrl.Result{}, nil
}

// getCurrentLeaderPod gets the current leader pod from the EndpointSlice (if it exists)
func (r *ServiceDirectorReconciler) getCurrentLeaderPod(ctx context.Context, svc *corev1.Service, logger klog.Logger) *corev1.Pod {
	leaderServiceName := r.getLeaderServiceName(svc)
	endpointSlice := &discoveryv1.EndpointSlice{}
	endpointSliceKey := types.NamespacedName{
		Name:      leaderServiceName,
		Namespace: svc.Namespace,
	}

	if err := r.Get(ctx, endpointSliceKey, endpointSlice); err != nil {
		return nil
	}

	// Find the pod referenced in the EndpointSlice (match by UID for restart safety)
	for _, endpoint := range endpointSlice.Endpoints {
		if endpoint.TargetRef != nil && endpoint.TargetRef.Kind == "Pod" && endpoint.TargetRef.UID != "" {
			pod := &corev1.Pod{}
			podKey := types.NamespacedName{
				Name:      endpoint.TargetRef.Name,
				Namespace: endpoint.TargetRef.Namespace,
			}
			if err := r.Get(ctx, podKey, pod); err == nil {
				// Verify UID matches (pod may have been recreated with same name)
				if string(pod.UID) == string(endpoint.TargetRef.UID) {
					return pod
				}
			}
		}
	}

	return nil
}

// selectLeaderPod selects the leader pod using controller-driven selection with stickiness
// bypassStickiness: if true, forces new leader selection even if current leader exists (leader-fast-path)
func (r *ServiceDirectorReconciler) selectLeaderPod(ctx context.Context, svc *corev1.Service, pods []corev1.Pod, bypassStickiness bool, logger klog.Logger) *corev1.Pod {
	// Check if sticky is enabled (default: true)
	sticky := true
	if svc.Annotations != nil {
		if val, ok := svc.Annotations[AnnotationStickyService]; ok && val == "false" {
			sticky = false
		}
	}

	// If bypassStickiness is true, skip sticky check (force new leader selection)
	// If sticky, check existing EndpointSlice for current leader
	if sticky && !bypassStickiness {
		leaderServiceName := r.getLeaderServiceName(svc)
		endpointSlice := &discoveryv1.EndpointSlice{}
		endpointSliceKey := types.NamespacedName{
			Name:      leaderServiceName,
			Namespace: svc.Namespace,
		}

		if err := r.Get(ctx, endpointSliceKey, endpointSlice); err == nil {
			// Found existing EndpointSlice - check if current leader is still Ready (match by UID)
			for _, endpoint := range endpointSlice.Endpoints {
				if endpoint.TargetRef != nil && endpoint.TargetRef.Kind == "Pod" && endpoint.TargetRef.UID != "" {
					// Find the pod by UID (restart-safe)
					for i := range pods {
						pod := &pods[i]
						if string(pod.UID) == string(endpoint.TargetRef.UID) {
							if isPodReady(pod) {
								logger.V(4).Info("Keeping sticky leader", "pod", pod.Name, "uid", pod.UID)
								if r.Metrics != nil {
									r.Metrics.RecordStickyLeaderHit(svc.Namespace, svc.Name)
								}
								return pod
							}
							break
						}
					}
				}
			}
		}
		// Sticky leader was not available
		if r.Metrics != nil {
			r.Metrics.RecordStickyLeaderMiss(svc.Namespace, svc.Name)
		}
	}

	// Filter to Ready pods only (apply flap damping if configured)
	var readyPods []corev1.Pod
	minReadyDuration := r.getMinReadyDuration(svc)
	now := time.Now()

	for _, pod := range pods {
		if !isPodReady(&pod) {
			continue
		}

		// Flap damping - pod must be Ready for at least minReadyDuration
		if minReadyDuration > 0 {
			readySince := r.getPodReadySince(&pod)
			if readySince == nil || now.Sub(*readySince) < minReadyDuration {
				logger.V(4).Info("Pod not ready long enough", "pod", pod.Name, "readySince", readySince, "minDuration", minReadyDuration)
				continue
			}
		}

		readyPods = append(readyPods, pod)
	}

	if len(readyPods) == 0 {
		logger.Info("No ready pods found for service")
		// Emit event for no ready pods scenario
		r.Recorder.Event(svc, corev1.EventTypeWarning, "NoReadyPods",
			fmt.Sprintf("No ready pods available for leader selection. Leader Service %s will have no endpoints until at least one pod becomes Ready.", r.getLeaderServiceName(svc)))
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
	leaderPod := &readyPods[0]
	logger.Info("Selected new leader pod", "pod", leaderPod.Name)
	return leaderPod
}

// reconcileLeaderService creates or updates the selector-less leader Service and EndpointSlice
func (r *ServiceDirectorReconciler) reconcileLeaderService(ctx context.Context, svc *corev1.Service, leaderPod *corev1.Pod, logger klog.Logger) error {
	leaderServiceName := r.getLeaderServiceName(svc)

	// Create or update selector-less leader Service
	leaderService := &corev1.Service{}
	leaderServiceKey := types.NamespacedName{
		Name:      leaderServiceName,
		Namespace: svc.Namespace,
	}

	// Resolve ports (handle named targetPort) - fail-closed
	leaderPorts, err := r.resolveServicePorts(svc, leaderPod)
	if err != nil {
		logger.Error(err, "Failed to resolve service ports", "error", err)
		r.Recorder.Event(svc, corev1.EventTypeWarning, "PortResolutionFailed", err.Error())
		// Fail-closed: if port resolution fails, don't create/update EndpointSlice
		// Delete existing EndpointSlice if it exists (clean failure mode)
		endpointSliceKey := types.NamespacedName{
			Name:      leaderServiceName,
			Namespace: svc.Namespace,
		}
		existingSlice := &discoveryv1.EndpointSlice{}
		if err := r.Get(ctx, endpointSliceKey, existingSlice); err == nil {
			if err := r.Delete(ctx, existingSlice); err != nil {
				logger.Error(err, "Failed to delete EndpointSlice after port resolution failure")
			} else {
				logger.Info("Deleted EndpointSlice due to port resolution failure")
				r.Recorder.Event(svc, corev1.EventTypeWarning, "EndpointSliceDeleted",
					"EndpointSlice deleted due to port resolution failure. Fix port configuration and reconciliation will recreate it.")
			}
		}
		// Still create/update the Service (it can exist without endpoints)
		// But don't create EndpointSlice until ports are resolved
		// Use empty ports list for Service (will be empty until ports resolve)
		leaderPorts = []corev1.ServicePort{}
		leaderPod = nil // Prevent EndpointSlice creation
	}

	if err := r.Get(ctx, leaderServiceKey, leaderService); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get leader service: %w", err)
		}
		// Service doesn't exist, create it
		// Filter GitOps labels/annotations to prevent ownership conflicts
		leaderLabels := filterGitOpsLabels(svc.Labels)
		leaderLabels[LabelManagedBy] = LabelManagedByValue
		leaderLabels[LabelSourceService] = svc.Name

		// Build annotations for leader Service (add leader tracking annotations)
		leaderAnnotations := filterGitOpsAnnotations(svc.Annotations)
		if leaderPod != nil {
			leaderAnnotations["zen-lead.io/current-leader"] = leaderPod.Name
			leaderAnnotations[AnnotationLeaderPodName] = leaderPod.Name
			leaderAnnotations[AnnotationLeaderPodUID] = string(leaderPod.UID)
			leaderAnnotations[AnnotationLeaderLastSwitchTime] = time.Now().Format(time.RFC3339)
		}

		leaderService = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        leaderServiceName,
				Namespace:   svc.Namespace,
				Labels:      leaderLabels,
				Annotations: leaderAnnotations,
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
				Selector: nil, // CRITICAL: No selector - we manage endpoints manually
				Ports:    leaderPorts,
				Type:     svc.Spec.Type, // Mirror service type (or default to ClusterIP)
			},
		}

		// Handle headless Services - if source is headless, default leader to ClusterIP
		if svc.Spec.ClusterIP == corev1.ClusterIPNone {
			leaderService.Spec.Type = corev1.ServiceTypeClusterIP
			leaderService.Spec.ClusterIP = "" // Let Kubernetes assign ClusterIP
		}

		// Default to ClusterIP if source service type is not suitable
		if leaderService.Spec.Type == "" {
			leaderService.Spec.Type = corev1.ServiceTypeClusterIP
		}

		if err := r.Create(ctx, leaderService); err != nil {
			return fmt.Errorf("failed to create leader service: %w", err)
		}
		logger.Info("Created selector-less leader service", "service", leaderServiceName)
		r.Recorder.Event(svc, corev1.EventTypeNormal, "LeaderServiceCreated",
			fmt.Sprintf("Created leader service %s. Leader routing available at %s", leaderServiceName, leaderServiceName))

		// Emit informational event about leader routing
		if leaderPod != nil {
			r.Recorder.Event(svc, corev1.EventTypeNormal, "LeaderRoutingAvailable",
				fmt.Sprintf("Leader routing available at %s (current leader: %s)", leaderServiceName, leaderPod.Name))
		} else {
			r.Recorder.Event(svc, corev1.EventTypeNormal, "LeaderRoutingAvailable",
				fmt.Sprintf("Leader routing available at %s (no leader selected yet)", leaderServiceName))
		}

		// Update total leader Services metric
		if r.Metrics != nil {
			r.updateResourceTotals(ctx, svc.Namespace, logger)
		}
	} else {
		// Service exists, update it
		originalService := leaderService.DeepCopy()
		leaderService.Spec.Selector = nil // Ensure no selector
		leaderService.Spec.Ports = leaderPorts
		leaderService.Spec.Type = svc.Spec.Type

		// Handle headless Services - if source is headless, default leader to ClusterIP
		if svc.Spec.ClusterIP == corev1.ClusterIPNone {
			leaderService.Spec.Type = corev1.ServiceTypeClusterIP
		}

		if leaderService.Spec.Type == "" {
			leaderService.Spec.Type = corev1.ServiceTypeClusterIP
		}

		// Update leader annotations (add pod-name, pod-uid, last-switch-time)
		if leaderService.Annotations == nil {
			leaderService.Annotations = make(map[string]string)
		}
		oldLeaderName := leaderService.Annotations[AnnotationLeaderPodName]
		oldLeaderUID := leaderService.Annotations[AnnotationLeaderPodUID]

		if leaderPod != nil {
			leaderService.Annotations["zen-lead.io/current-leader"] = leaderPod.Name
			leaderService.Annotations[AnnotationLeaderPodName] = leaderPod.Name
			leaderService.Annotations[AnnotationLeaderPodUID] = string(leaderPod.UID)

			// Update last switch time if leader changed (by UID)
			if oldLeaderUID != string(leaderPod.UID) {
				leaderService.Annotations[AnnotationLeaderLastSwitchTime] = time.Now().Format(time.RFC3339)
				// Emit event if leader changed
				if oldLeaderName != "" && oldLeaderName != leaderPod.Name {
					r.Recorder.Event(svc, corev1.EventTypeNormal, "LeaderChanged",
						fmt.Sprintf("Leader changed from %s to %s. Routing available at %s", oldLeaderName, leaderPod.Name, leaderServiceName))
				}
			}
		} else {
			delete(leaderService.Annotations, "zen-lead.io/current-leader")
			delete(leaderService.Annotations, AnnotationLeaderPodName)
			delete(leaderService.Annotations, AnnotationLeaderPodUID)
			// Keep last switch time for debugging
		}

		if err := r.Patch(ctx, leaderService, client.MergeFrom(originalService)); err != nil {
			return fmt.Errorf("failed to patch leader service: %w", err)
		}
	}

	// Create or update EndpointSlice
	if err := r.reconcileEndpointSlice(ctx, svc, leaderServiceName, leaderPod, leaderPorts, logger); err != nil {
		return fmt.Errorf("failed to reconcile endpoint slice: %w", err)
	}

	// Record leader stability and endpoint status
	if r.Metrics != nil {
		if leaderPod != nil && isPodReady(leaderPod) {
			r.Metrics.RecordLeaderStable(svc.Namespace, svc.Name, true)
			r.Metrics.RecordLeaderServiceWithoutEndpoints(svc.Namespace, svc.Name, false)
		} else {
			r.Metrics.RecordLeaderStable(svc.Namespace, svc.Name, false)
			r.Metrics.RecordLeaderServiceWithoutEndpoints(svc.Namespace, svc.Name, true)
		}
	}

	return nil
}

// resolveServicePorts resolves Service ports to EndpointSlice ports, handling named targetPort
// Fail-closed: if any named port cannot be resolved, returns error (no fallback)
func (r *ServiceDirectorReconciler) resolveServicePorts(svc *corev1.Service, leaderPod *corev1.Pod) ([]corev1.ServicePort, error) {
	ports := make([]corev1.ServicePort, 0, len(svc.Spec.Ports))

	for _, svcPort := range svc.Spec.Ports {
		resolvedPort := svcPort.DeepCopy()

		// Resolve targetPort
		if svcPort.TargetPort.Type == intstr.String {
			// Named port - resolve against leader pod (fail-closed)
			if leaderPod == nil {
				return nil, fmt.Errorf("cannot resolve named port %s: no leader pod available", svcPort.TargetPort.StrVal)
			}
			targetPort, err := r.resolveNamedPort(leaderPod, svcPort.TargetPort.StrVal)
			if err != nil {
				// Fail-closed: return error instead of fallback
				r.Recorder.Event(svc, corev1.EventTypeWarning, "NamedPortResolutionFailed",
					fmt.Sprintf("Failed to resolve named port %s in pod %s: %v. EndpointSlice will have no endpoints until port is resolved.", svcPort.TargetPort.StrVal, leaderPod.Name, err))
				// Record metric
				if r.Metrics != nil {
					r.Metrics.RecordPortResolutionFailure(svc.Namespace, svc.Name, svcPort.TargetPort.StrVal)
				}
				return nil, fmt.Errorf("failed to resolve named port %s: %w", svcPort.TargetPort.StrVal, err)
			}
			resolvedPort.TargetPort = intstr.FromInt32(targetPort)
		}
		// If targetPort is int, keep it as-is

		ports = append(ports, *resolvedPort)
	}

	return ports, nil
}

// resolveNamedPort resolves a named port against a pod's container ports
func (r *ServiceDirectorReconciler) resolveNamedPort(pod *corev1.Pod, portName string) (int32, error) {
	for _, container := range pod.Spec.Containers {
		for _, containerPort := range container.Ports {
			if containerPort.Name == portName {
				return containerPort.ContainerPort, nil
			}
		}
	}
	return 0, fmt.Errorf("named port %s not found in pod %s", portName, pod.Name)
}

// reconcileEndpointSlice creates or updates EndpointSlice pointing to leader pod
func (r *ServiceDirectorReconciler) reconcileEndpointSlice(ctx context.Context, svc *corev1.Service, leaderServiceName string, leaderPod *corev1.Pod, servicePorts []corev1.ServicePort, logger klog.Logger) error {
	endpointSliceName := leaderServiceName
	endpointSlice := &discoveryv1.EndpointSlice{}
	endpointSliceKey := types.NamespacedName{
		Name:      endpointSliceName,
		Namespace: svc.Namespace,
	}

	// Convert ServicePorts to EndpointPorts (using resolved targetPort)
	// Note: resolveServicePorts already resolved named ports, so TargetPort should be int here
	endpointPorts := make([]discoveryv1.EndpointPort, len(servicePorts))
	for i, port := range servicePorts {
		var portNum *int32
		var portName *string

		// Determine the actual backend port (should be resolved by resolveServicePorts)
		var backendPort int32
		if port.TargetPort.Type == intstr.Int {
			backendPort = int32(port.TargetPort.IntVal)
		} else {
			// This should not happen if resolveServicePorts worked correctly
			// But if it does, fail-closed: don't create endpoint
			return fmt.Errorf("port %s has unresolved named targetPort %s", port.Name, port.TargetPort.StrVal)
		}

		portNum = &backendPort
		if port.Name != "" {
			portName = &port.Name
		}

		endpointPorts[i] = discoveryv1.EndpointPort{
			Name:     portName,
			Port:     portNum,
			Protocol: &port.Protocol,
		}
	}

	// Build endpoint from pod
	var endpointAddresses []string
	var nodeName *string
	var targetRef *corev1.ObjectReference

	if leaderPod != nil && leaderPod.Status.PodIP != "" {
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

	// Determine address type from pod IP
	addressType := discoveryv1.AddressTypeIPv4
	if leaderPod != nil && leaderPod.Status.PodIP != "" {
		// Simple heuristic: if IP contains ":", it's IPv6
		if strings.Contains(leaderPod.Status.PodIP, ":") {
			addressType = discoveryv1.AddressTypeIPv6
		}
	}

	// Determine ready condition from pod readiness
	var ready *bool
	if leaderPod != nil {
		for _, condition := range leaderPod.Status.Conditions {
			if condition.Type == corev1.PodReady {
				b := condition.Status == corev1.ConditionTrue
				ready = &b
				break
			}
		}
		if ready == nil {
			b := false
			ready = &b
		}
	} else {
		b := false
		ready = &b
	}

	endpoint := discoveryv1.Endpoint{
		Addresses: endpointAddresses,
		Conditions: discoveryv1.EndpointConditions{
			Ready: ready,
		},
		NodeName:  nodeName,
		TargetRef: targetRef,
	}

	if err := r.Get(ctx, endpointSliceKey, endpointSlice); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get endpoint slice: %w", err)
		}
		// EndpointSlice doesn't exist, create it
		// Filter GitOps labels to prevent ownership conflicts
		endpointSliceLabels := filterGitOpsLabels(svc.Labels)
		endpointSliceLabels[discoveryv1.LabelServiceName] = leaderServiceName
		endpointSliceLabels[LabelManagedBy] = LabelManagedByValue
		endpointSliceLabels[LabelSourceService] = svc.Name
		endpointSliceLabels[LabelEndpointSliceManagedBy] = LabelEndpointSliceManagedByValue

		endpointSlice = &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      endpointSliceName,
				Namespace: svc.Namespace,
				Labels:    endpointSliceLabels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       leaderServiceName,
						UID: func() types.UID {
							leaderSvc := &corev1.Service{}
							if err := r.Get(ctx, types.NamespacedName{Name: leaderServiceName, Namespace: svc.Namespace}, leaderSvc); err == nil {
								return leaderSvc.UID
							}
							return ""
						}(),
						Controller: func() *bool { b := true; return &b }(),
					},
				},
			},
			AddressType: addressType,
			Endpoints:   []discoveryv1.Endpoint{endpoint},
			Ports:       endpointPorts,
		}

		if err := r.Create(ctx, endpointSlice); err != nil {
			// Record endpoint write error
			if r.Metrics != nil {
				r.Metrics.RecordEndpointWriteError(svc.Namespace, svc.Name)
			}
			return fmt.Errorf("failed to create endpoint slice: %w", err)
		}
		logger.Info("Created endpoint slice for leader pod", "endpointslice", endpointSliceName, "pod", func() string {
			if leaderPod != nil {
				return leaderPod.Name
			}
			return "none"
		}())

		// Update total EndpointSlices metric
		if r.Metrics != nil {
			r.updateResourceTotals(ctx, svc.Namespace, logger)
		}
		return nil
	}

	// EndpointSlice exists, update it
	originalEndpointSlice := endpointSlice.DeepCopy()
	endpointSlice.Endpoints = []discoveryv1.Endpoint{endpoint}
	endpointSlice.Ports = endpointPorts
	endpointSlice.AddressType = addressType

	if err := r.Patch(ctx, endpointSlice, client.MergeFrom(originalEndpointSlice)); err != nil {
		// Record endpoint write error
		if r.Metrics != nil {
			r.Metrics.RecordEndpointWriteError(svc.Namespace, svc.Name)
		}
		return fmt.Errorf("failed to patch endpoint slice: %w", err)
	}

	logger.V(4).Info("Updated endpoint slice for leader pod", "endpointslice", endpointSliceName, "pod", func() string {
		if leaderPod != nil {
			return leaderPod.Name
		}
		return "none"
	}())
	return nil
}

// updateResourceTotals updates the total count metrics for leader Services and EndpointSlices
func (r *ServiceDirectorReconciler) updateResourceTotals(ctx context.Context, namespace string, logger klog.Logger) {
	if r.Metrics == nil {
		return
	}

	// Count leader Services (selector-less Services with zen-lead managed-by label)
	leaderServiceList := &corev1.ServiceList{}
	if err := r.List(ctx, leaderServiceList, client.InNamespace(namespace), client.MatchingLabels{
		LabelManagedBy: LabelManagedByValue,
	}); err == nil {
		r.Metrics.RecordLeaderServicesTotal(namespace, len(leaderServiceList.Items))
	} else {
		logger.V(4).Info("Failed to list leader services for metrics", "error", err)
	}

	// Count EndpointSlices (with zen-lead managed-by label)
	endpointSliceList := &discoveryv1.EndpointSliceList{}
	if err := r.List(ctx, endpointSliceList, client.InNamespace(namespace), client.MatchingLabels{
		LabelEndpointSliceManagedBy: LabelEndpointSliceManagedByValue,
	}); err == nil {
		r.Metrics.RecordEndpointSlicesTotal(namespace, len(endpointSliceList.Items))
	} else {
		logger.V(4).Info("Failed to list endpoint slices for metrics", "error", err)
	}
}

// cleanupLeaderResources removes leader Service and EndpointSlice when annotation is removed
func (r *ServiceDirectorReconciler) cleanupLeaderResources(ctx context.Context, svcName types.NamespacedName, logger klog.Logger) (ctrl.Result, error) {
	// Try to determine leader service name (best effort)
	svc := &corev1.Service{}
	if err := r.Get(ctx, svcName, svc); err == nil {
		leaderServiceName := r.getLeaderServiceName(svc)
		leaderServiceKey := types.NamespacedName{
			Name:      leaderServiceName,
			Namespace: svcName.Namespace,
		}

		// Delete leader Service (GC will delete EndpointSlice via ownerRef)
		leaderService := &corev1.Service{}
		if err := r.Get(ctx, leaderServiceKey, leaderService); err == nil {
			if err := r.Delete(ctx, leaderService); err != nil {
				logger.Error(err, "Failed to delete leader service", "service", leaderServiceName)
				return ctrl.Result{}, err
			}
			logger.Info("Deleted leader service", "service", leaderServiceName)
		}
	} else {
		// Service doesn't exist - try to find and delete leader service by label
		leaderServiceList := &corev1.ServiceList{}
		if err := r.List(ctx, leaderServiceList, client.InNamespace(svcName.Namespace), client.MatchingLabels{
			LabelSourceService: svcName.Name,
			LabelManagedBy:     LabelManagedByValue,
		}); err == nil {
			for i := range leaderServiceList.Items {
				if err := r.Delete(ctx, &leaderServiceList.Items[i]); err != nil {
					logger.Error(err, "Failed to delete leader service", "service", leaderServiceList.Items[i].Name)
				}
			}
		}
	}

	return ctrl.Result{}, nil
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

// getPodReadySince returns the time when the pod became Ready (LastTransitionTime of Ready condition)
// Returns nil if pod is not currently Ready (flap damping)
func (r *ServiceDirectorReconciler) getPodReadySince(pod *corev1.Pod) *time.Time {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return &condition.LastTransitionTime.Time
		}
	}
	return nil
}

// getMinReadyDuration parses the min-ready-duration annotation (flap damping)
func (r *ServiceDirectorReconciler) getMinReadyDuration(svc *corev1.Service) time.Duration {
	if svc.Annotations == nil {
		return 0 // Default: no flap damping
	}
	durationStr := svc.Annotations[AnnotationMinReadyDurationService]
	if durationStr == "" || durationStr == "0s" {
		return 0
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		// Invalid duration - log warning but don't fail
		return 0
	}
	return duration
}

// getLeaderServiceName determines the leader service name
func (r *ServiceDirectorReconciler) getLeaderServiceName(svc *corev1.Service) string {
	if svc.Annotations != nil {
		if customName := svc.Annotations[AnnotationLeaderServiceNameService]; customName != "" {
			return customName
		}
	}
	// Default: <service-name>-leader
	return svc.Name + ServiceSuffixService
}

// SetupWithManager sets up the ServiceDirectorReconciler with the manager
// Pod watch predicates filter to meaningful transitions only (Ready, deletionTimestamp, podIP, phase)
func (r *ServiceDirectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Pod watch predicate - only react to meaningful transitions
	podPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Always reconcile on pod creation (may become leader candidate)
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, okOld := e.ObjectOld.(*corev1.Pod)
			newPod, okNew := e.ObjectNew.(*corev1.Pod)
			if !okOld || !okNew {
				return false
			}

			// Check for meaningful transitions
			// 1. Ready condition changed
			oldReady := isPodReady(oldPod)
			newReady := isPodReady(newPod)
			if oldReady != newReady {
				return true
			}

			// 2. DeletionTimestamp became non-nil
			oldDeleting := oldPod.DeletionTimestamp != nil
			newDeleting := newPod.DeletionTimestamp != nil
			if oldDeleting != newDeleting {
				return true
			}

			// 3. PodIP changed (empty ↔ assigned)
			oldIP := oldPod.Status.PodIP
			newIP := newPod.Status.PodIP
			if oldIP != newIP {
				return true
			}

			// 4. Phase changed to Failed/Succeeded
			if (oldPod.Status.Phase == corev1.PodFailed || oldPod.Status.Phase == corev1.PodSucceeded) &&
				(newPod.Status.Phase != oldPod.Status.Phase) {
				return true
			}
			if (newPod.Status.Phase == corev1.PodFailed || newPod.Status.Phase == corev1.PodSucceeded) &&
				(oldPod.Status.Phase != newPod.Status.Phase) {
				return true
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Always reconcile on pod deletion (leader may be deleted)
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			// Don't reconcile on generic events
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.mapPodToService),
			builder.WithPredicates(podPredicate),
		).
		Watches(
			&discoveryv1.EndpointSlice{},
			handler.EnqueueRequestsFromMapFunc(r.mapEndpointSliceToService),
		).
		// Bound reconcile concurrency + Safety resync handled by informer cache (default 10m)
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10, // Bound reconcile concurrency to prevent starvation
		}).
		Complete(r)
}

// mapPodToService maps Pod changes to Service reconciles (for failover detection)
// Uses cache/index for efficient pod-to-service mapping
func (r *ServiceDirectorReconciler) mapPodToService(ctx context.Context, obj client.Object) []reconcile.Request {
	pod := obj.(*corev1.Pod)

	// Use cache to only check opted-in Services in this namespace
	cachedServices := r.optedInServicesCache[pod.Namespace]
	if len(cachedServices) == 0 {
		// Cache miss - refresh cache for this namespace
		logger := klog.FromContext(ctx)
		r.updateOptedInServicesCache(ctx, pod.Namespace, logger)
		cachedServices = r.optedInServicesCache[pod.Namespace]
	}

	var requests []reconcile.Request
	for _, cachedSvc := range cachedServices {
		// Check if pod matches service selector
		if cachedSvc.selector.Matches(labels.Set(pod.Labels)) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cachedSvc.name,
					Namespace: pod.Namespace,
				},
			})
		}
	}

	return requests
}

// mapEndpointSliceToService maps EndpointSlice changes to Service reconciles (for drift detection)
func (r *ServiceDirectorReconciler) mapEndpointSliceToService(ctx context.Context, obj client.Object) []reconcile.Request {
	endpointSlice := obj.(*discoveryv1.EndpointSlice)

	// Only process EndpointSlices managed by zen-lead
	if endpointSlice.Labels == nil || endpointSlice.Labels[LabelEndpointSliceManagedBy] != LabelEndpointSliceManagedByValue {
		return nil
	}

	// Find the source Service
	sourceServiceName, ok := endpointSlice.Labels[LabelSourceService]
	if !ok {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      sourceServiceName,
				Namespace: endpointSlice.Namespace,
			},
		},
	}
}

// updateOptedInServicesCache updates the cache for a specific namespace
func (r *ServiceDirectorReconciler) updateOptedInServicesCache(ctx context.Context, namespace string, logger klog.Logger) {
	serviceList := &corev1.ServiceList{}
	if err := r.List(ctx, serviceList, client.InNamespace(namespace)); err != nil {
		logger.V(4).Info("Failed to list services for cache update", "error", err)
		return
	}

	var cached []*cachedService
	for i := range serviceList.Items {
		svc := &serviceList.Items[i]
		// Only cache opted-in Services with selectors
		if svc.Annotations == nil || svc.Annotations[AnnotationEnabledService] != "true" {
			continue
		}
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		selector := labels.SelectorFromSet(svc.Spec.Selector)
		cached = append(cached, &cachedService{
			name:     svc.Name,
			selector: selector,
		})
	}
	r.optedInServicesCache[namespace] = cached
}

// updateOptedInServicesCacheForService updates cache for a single Service
func (r *ServiceDirectorReconciler) updateOptedInServicesCacheForService(svc *corev1.Service, logger klog.Logger) {
	if svc.Annotations == nil || svc.Annotations[AnnotationEnabledService] != "true" {
		// Not opted in - remove from cache if present
		cached := r.optedInServicesCache[svc.Namespace]
		for i, cachedSvc := range cached {
			if cachedSvc.name == svc.Name {
				// Remove from cache
				r.optedInServicesCache[svc.Namespace] = append(cached[:i], cached[i+1:]...)
				return
			}
		}
		return
	}

	if len(svc.Spec.Selector) == 0 {
		// No selector - remove from cache if present
		cached := r.optedInServicesCache[svc.Namespace]
		for i, cachedSvc := range cached {
			if cachedSvc.name == svc.Name {
				r.optedInServicesCache[svc.Namespace] = append(cached[:i], cached[i+1:]...)
				return
			}
		}
		return
	}

	// Add or update in cache
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	cached := r.optedInServicesCache[svc.Namespace]
	for i, cachedSvc := range cached {
		if cachedSvc.name == svc.Name {
			// Update existing
			cached[i].selector = selector
			return
		}
	}
	// Add new
	r.optedInServicesCache[svc.Namespace] = append(cached, &cachedService{
		name:     svc.Name,
		selector: selector,
	})
}
