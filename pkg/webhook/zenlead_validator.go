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

package webhook

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// LabelPool is the label key for the pool name
	LabelPool = "zen-lead/pool"
	// AnnotationPolicyOverride allows users to override auto-detected policy
	AnnotationPolicyOverride = "zen-lead/policy"
)

// ZenLeadValidatingWebhook validates Pod creation requests to ensure only leader pods are allowed
// This implements the "Gatekeeper" pattern where zen-lead actively rejects non-leader Pod creation
type ZenLeadValidatingWebhook struct {
	Client  client.Client
	decoder admission.Decoder
}

// NewZenLeadValidatingWebhook creates a new ZenLeadValidatingWebhook
func NewZenLeadValidatingWebhook(client client.Client, scheme *runtime.Scheme) (*ZenLeadValidatingWebhook, error) {
	decoder := admission.NewDecoder(scheme)
	return &ZenLeadValidatingWebhook{
		Client:  client,
		decoder: decoder,
	}, nil
}

// Handle processes admission requests for Pod creation
// It acts as a "Gatekeeper" by rejecting Pod creation requests from non-leader replicas
func (w *ZenLeadValidatingWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := klog.FromContext(ctx)
	logger = logger.WithValues(
		"operation", req.Operation,
		"kind", req.Kind.Kind,
		"name", req.Name,
		"namespace", req.Namespace,
	)

	// Only validate Pod creation requests
	if req.Kind.Kind != "Pod" || req.Operation != "CREATE" {
		// Allow all other operations (UPDATE, DELETE) and non-Pod resources
		return admission.Allowed("not a Pod CREATE request")
	}

	// Decode Pod
	pod := &corev1.Pod{}
	if err := w.decoder.Decode(req, pod); err != nil {
		logger.Error(err, "Failed to decode Pod")
		return admission.Errored(400, fmt.Errorf("failed to decode Pod: %w", err))
	}

	// Check if Pod belongs to a Deployment/StatefulSet with zen-lead/pool label
	poolName, ownerKind, err := w.findPoolFromOwner(ctx, pod)
	if err != nil {
		logger.V(4).Info("Pod does not belong to a zen-lead pool", "error", err)
		// Allow Pod creation if it's not part of a zen-lead pool
		return admission.Allowed("not part of zen-lead pool")
	}

	if poolName == "" {
		// Pod is not part of any zen-lead pool, allow it
		return admission.Allowed("not part of zen-lead pool")
	}

	logger = logger.WithValues("pool", poolName, "owner_kind", ownerKind)

	// Auto-detect policy based on owner Kind
	policy := w.autoDetectPolicy(ownerKind, pod)
	if policy == "allow-all" {
		// User override: allow all pods
		logger.Info("Policy override detected, allowing all pods", "policy", policy)
		return admission.Allowed("policy override: allow-all")
	}

	// Get the current leader from Lease
	leaderIdentity, err := w.getLeaderIdentity(ctx, poolName, req.Namespace)
	if err != nil {
		logger.Error(err, "Failed to get leader identity, allowing request (fail-safe)")
		// Fail-safe: if we can't determine leader, allow the request
		// This prevents blocking all pods if zen-lead is misconfigured
		return admission.Allowed("failed to determine leader (fail-safe)")
	}

	if leaderIdentity == "" {
		logger.Info("No leader elected yet, allowing request (leader election in progress)")
		// No leader yet, allow the request (leader election will happen)
		return admission.Allowed("no leader elected yet")
	}

	// Compare Pod identity against leader identity
	podIdentity := w.extractPodIdentity(pod)
	isLeader := w.isLeaderPod(podIdentity, leaderIdentity)

	if isLeader {
		logger.Info("Traffic directed to leader pod for pool",
			"pod", pod.Name,
			"pool", poolName,
			"leader_identity", leaderIdentity,
		)
		return admission.Allowed("leader pod allowed")
	}

	// This is a follower pod, reject the request
	logger.Info("Request from follower pod blocked",
		"pod", pod.Name,
		"pool", poolName,
		"pod_identity", podIdentity,
		"leader_identity", leaderIdentity,
		"policy", policy,
	)

	return admission.Denied(fmt.Sprintf(
		"Only the leader replica is allowed to reconcile active workloads. "+
			"Pod %s is not the leader for pool %s. Current leader: %s",
		pod.Name, poolName, leaderIdentity,
	))
}

// findPoolFromOwner finds the pool name from the Pod's owner (Deployment, StatefulSet, etc.)
func (w *ZenLeadValidatingWebhook) findPoolFromOwner(ctx context.Context, pod *corev1.Pod) (poolName string, ownerKind string, err error) {
	// Check Pod labels first (direct label)
	if poolName, exists := pod.Labels[LabelPool]; exists {
		// Try to determine owner kind from owner references
		if len(pod.OwnerReferences) > 0 {
			ownerKind = pod.OwnerReferences[0].Kind
		}
		return poolName, ownerKind, nil
	}

	// If not found in Pod labels, check owner references
	for _, ownerRef := range pod.OwnerReferences {
		switch ownerRef.Kind {
		case "ReplicaSet":
			// ReplicaSet is owned by Deployment - need to check Deployment
			rs := &appsv1.ReplicaSet{}
			rsKey := types.NamespacedName{
				Name:      ownerRef.Name,
				Namespace: pod.Namespace,
			}
			if err := w.Client.Get(ctx, rsKey, rs); err != nil {
				continue
			}

			// Check ReplicaSet labels
			if poolName, exists := rs.Labels[LabelPool]; exists {
				return poolName, "Deployment", nil
			}

			// Check ReplicaSet's owner (Deployment)
			for _, rsOwnerRef := range rs.OwnerReferences {
				if rsOwnerRef.Kind == "Deployment" {
					deployment := &appsv1.Deployment{}
					deployKey := types.NamespacedName{
						Name:      rsOwnerRef.Name,
						Namespace: pod.Namespace,
					}
					if err := w.Client.Get(ctx, deployKey, deployment); err == nil {
						if poolName, exists := deployment.Labels[LabelPool]; exists {
							return poolName, "Deployment", nil
						}
					}
				}
			}

		case "StatefulSet":
			// StatefulSet directly owns Pods
			ss := &appsv1.StatefulSet{}
			ssKey := types.NamespacedName{
				Name:      ownerRef.Name,
				Namespace: pod.Namespace,
			}
			if err := w.Client.Get(ctx, ssKey, ss); err == nil {
				if poolName, exists := ss.Labels[LabelPool]; exists {
					return poolName, "StatefulSet", nil
				}
			}

		case "Job":
			// Job directly owns Pods
			job := &batchv1.Job{}
			jobKey := types.NamespacedName{
				Name:      ownerRef.Name,
				Namespace: pod.Namespace,
			}
			if err := w.Client.Get(ctx, jobKey, job); err == nil {
				if poolName, exists := job.Labels[LabelPool]; exists {
					return poolName, "Job", nil
				}
			}

		case "CronJob":
			// CronJob creates Jobs, which create Pods
			// Check if Pod has pool label
			if poolName, exists := pod.Labels[LabelPool]; exists {
				return poolName, "CronJob", nil
			}
		}
	}

	return "", "", fmt.Errorf("no pool found for pod")
}

// autoDetectPolicy automatically detects the HA policy based on workload Kind
// This implements the "Smart Auto-Detect" feature
func (w *ZenLeadValidatingWebhook) autoDetectPolicy(ownerKind string, pod *corev1.Pod) string {
	// Check for user override annotation
	if policyOverride, exists := pod.Annotations[AnnotationPolicyOverride]; exists {
		return policyOverride
	}

	// Auto-detect based on Kind
	switch ownerKind {
	case "Deployment", "ReplicaSet":
		return "TrafficDirector" // Route traffic to leader, reject followers
	case "StatefulSet":
		return "TrafficDirector" // Route traffic to leader, reject followers
	case "Job":
		return "StateGuard" // Ensure only one pod active
	case "CronJob":
		return "StateGuard" // Ensure only one pod active
	default:
		// Default to TrafficDirector for safety
		return "TrafficDirector"
	}
}

// getLeaderIdentity gets the current leader identity from the Lease resource
func (w *ZenLeadValidatingWebhook) getLeaderIdentity(ctx context.Context, poolName, namespace string) (string, error) {
	lease := &coordinationv1.Lease{}
	leaseKey := types.NamespacedName{
		Name:      poolName,
		Namespace: namespace,
	}

	if err := w.Client.Get(ctx, leaseKey, lease); err != nil {
		return "", fmt.Errorf("failed to get lease: %w", err)
	}

	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity != "" {
		return *lease.Spec.HolderIdentity, nil
	}

	return "", nil // No leader yet
}

// extractPodIdentity extracts the identity of the Pod for comparison
func (w *ZenLeadValidatingWebhook) extractPodIdentity(pod *corev1.Pod) string {
	// Try to match against pod name or pod-name-uid format
	// This matches the identity format used by zen-lead election
	return fmt.Sprintf("%s-%s", pod.Name, string(pod.UID))
}

// isLeaderPod checks if the Pod identity matches the leader identity
func (w *ZenLeadValidatingWebhook) isLeaderPod(podIdentity, leaderIdentity string) bool {
	// Leader identity can be:
	// - Pod name (e.g., "zen-flow-controller-abc123")
	// - Pod name-uid (e.g., "zen-flow-controller-abc123-xyz789")
	// - Just the prefix (e.g., "zen-flow-controller-abc123-")

	// Extract pod name prefix (before the UID)
	podNamePrefix := strings.Split(podIdentity, "-")[0]
	if len(strings.Split(podIdentity, "-")) > 1 {
		// Reconstruct without UID
		parts := strings.Split(podIdentity, "-")
		podNamePrefix = strings.Join(parts[:len(parts)-1], "-")
	}

	// Check exact match
	if podIdentity == leaderIdentity {
		return true
	}

	// Check if leader identity starts with pod name prefix
	if strings.HasPrefix(leaderIdentity, podNamePrefix+"-") {
		return true
	}

	// Check if pod identity starts with leader identity
	if strings.HasPrefix(podIdentity, leaderIdentity+"-") {
		return true
	}

	return false
}

