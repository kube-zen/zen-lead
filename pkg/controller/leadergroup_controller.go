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
	"fmt"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	leadershipv1alpha1 "github.com/kube-zen/zen-lead/pkg/apis/leadership.kube-zen.io/v1alpha1"
)

// LeaderGroupReconciler reconciles a LeaderGroup object
// For Profile C: zen-lead manages Leases for controller HA.
type LeaderGroupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=leadership.kube-zen.io,resources=leadergroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=leadership.kube-zen.io,resources=leadergroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=leadership.kube-zen.io,resources=leadergroups/finalizers,verbs=update
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

// Reconcile processes LeaderGroup resources.
// For controller type: ensures a Lease exists and updates LeaderGroup status from Lease.
// For routing type: optionally drives Service-based routing (but Service annotation is preferred).
func (r *LeaderGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch LeaderGroup
	lg := &leadershipv1alpha1.LeaderGroup{}
	if err := r.Get(ctx, req.NamespacedName, lg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !lg.DeletionTimestamp.IsZero() {
		// Cleanup Lease if it exists
		if err := r.cleanupLease(ctx, lg); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Process based on type
	switch lg.Spec.Type {
	case leadershipv1alpha1.LeaderGroupTypeController:
		return r.reconcileControllerType(ctx, lg)
	case leadershipv1alpha1.LeaderGroupTypeRouting:
		// Routing type: optionally drive Service-based routing
		// But Service annotation is the preferred path (Profile A)
		logger.Info("Routing type LeaderGroup - prefer Service annotation for Profile A")
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, fmt.Errorf("unknown LeaderGroup type: %q", lg.Spec.Type)
	}
}

// reconcileControllerType handles controller type LeaderGroups.
// Ensures a Lease exists with deterministic name and updates status from Lease.
func (r *LeaderGroupReconciler) reconcileControllerType(ctx context.Context, lg *leadershipv1alpha1.LeaderGroup) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate component name
	if lg.Spec.Component == "" {
		return ctrl.Result{}, fmt.Errorf("component name is required for controller type")
	}

	// Derive Lease name deterministically (matches zen-sdk/pkg/zenlead)
	leaseName := deriveLeaseName(lg.Spec.Component)

	// Fetch or create Lease
	lease := &coordinationv1.Lease{}
	leaseKey := types.NamespacedName{
		Namespace: lg.Namespace,
		Name:      leaseName,
	}

	err := r.Get(ctx, leaseKey, lease)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create Lease
			lease = r.buildLease(lg, leaseName)
			if err := r.Create(ctx, lease); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create Lease: %w", err)
			}
			logger.Info("Created Lease for LeaderGroup", "lease", leaseName)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// Update Lease ownerRef and labels if needed
		if err := r.updateLeaseMetadata(ctx, lease, lg); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update LeaderGroup status from Lease
	return r.updateStatusFromLease(ctx, lg, lease)
}

// buildLease creates a Lease object for a LeaderGroup.
func (r *LeaderGroupReconciler) buildLease(lg *leadershipv1alpha1.LeaderGroup, leaseName string) *coordinationv1.Lease {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: lg.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "zen-lead",
				"leadership.kube-zen.io/leadergroup": lg.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: lg.APIVersion,
					Kind:       lg.Kind,
					Name:       lg.Name,
					UID:        lg.UID,
					Controller: func() *bool { b := true; return &b }(),
				},
			},
		},
		Spec: coordinationv1.LeaseSpec{},
	}

	// Apply lease settings if provided
	if lg.Spec.Lease != nil {
		if lg.Spec.Lease.Duration != nil {
			leaseDuration := int32(lg.Spec.Lease.Duration.Seconds())
			lease.Spec.LeaseDurationSeconds = &leaseDuration
		}
		if lg.Spec.Lease.RenewDeadline != nil {
			renewDeadline := int32(lg.Spec.Lease.RenewDeadline.Seconds())
			lease.Spec.RenewDeadlineSeconds = &renewDeadline
		}
		if lg.Spec.Lease.RetryPeriod != nil {
			retryPeriod := int32(lg.Spec.Lease.RetryPeriod.Seconds())
			lease.Spec.LeaseTransitions = &retryPeriod
		}
	}

	return lease
}

// updateLeaseMetadata updates Lease ownerRef and labels to match LeaderGroup.
func (r *LeaderGroupReconciler) updateLeaseMetadata(ctx context.Context, lease *coordinationv1.Lease, lg *leadershipv1alpha1.LeaderGroup) error {
	needsUpdate := false

	// Check ownerRef
	if len(lease.OwnerReferences) == 0 {
		needsUpdate = true
		lease.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: lg.APIVersion,
				Kind:       lg.Kind,
				Name:       lg.Name,
				UID:        lg.UID,
				Controller: func() *bool { b := true; return &b }(),
			},
		}
	}

	// Check labels
	if lease.Labels == nil {
		lease.Labels = make(map[string]string)
	}
	if lease.Labels["app.kubernetes.io/managed-by"] != "zen-lead" {
		lease.Labels["app.kubernetes.io/managed-by"] = "zen-lead"
		needsUpdate = true
	}
	if lease.Labels["leadership.kube-zen.io/leadergroup"] != lg.Name {
		lease.Labels["leadership.kube-zen.io/leadergroup"] = lg.Name
		needsUpdate = true
	}

	if needsUpdate {
		return r.Update(ctx, lease)
	}
	return nil
}

// updateStatusFromLease updates LeaderGroup status from Lease.
func (r *LeaderGroupReconciler) updateStatusFromLease(ctx context.Context, lg *leadershipv1alpha1.LeaderGroup, lease *coordinationv1.Lease) (ctrl.Result, error) {
	status := lg.Status.DeepCopy()

	// Update from Lease
	status.HolderIdentity = lease.Spec.HolderIdentity
	if lease.Spec.RenewTime != nil {
		status.RenewTime = &metav1.Time{Time: lease.Spec.RenewTime.Time}
	}
	if lease.Spec.LeaseDurationSeconds != nil {
		status.LeaseDurationSeconds = lease.Spec.LeaseDurationSeconds
	}
	if lease.Spec.LeaseTransitions != nil {
		status.FencingToken = lease.Spec.LeaseTransitions
	}
	status.ObservedLeaseResourceVersion = lease.ResourceVersion

	// Update conditions
	condition := metav1.Condition{
		Type:               "LeaseReady",
		Status:             metav1.ConditionTrue,
		Reason:             "LeaseExists",
		Message:            fmt.Sprintf("Lease %q exists", lease.Name),
		LastTransitionTime: metav1.Now(),
	}
	if lease.Spec.HolderIdentity == "" {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NoHolder"
		condition.Message = "Lease exists but no holder"
	}

	// Update condition
	found := false
	for i, c := range status.Conditions {
		if c.Type == condition.Type {
			status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		status.Conditions = append(status.Conditions, condition)
	}

	// Update status if changed
	if !statusEqual(lg.Status, *status) {
		lg.Status = *status
		if err := r.Status().Update(ctx, lg); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Requeue to watch Lease changes
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// cleanupLease removes the Lease when LeaderGroup is deleted.
func (r *LeaderGroupReconciler) cleanupLease(ctx context.Context, lg *leadershipv1alpha1.LeaderGroup) error {
	if lg.Spec.Type != leadershipv1alpha1.LeaderGroupTypeController {
		return nil
	}

	leaseName := deriveLeaseName(lg.Spec.Component)
	lease := &coordinationv1.Lease{}
	leaseKey := types.NamespacedName{
		Namespace: lg.Namespace,
		Name:      leaseName,
	}

	if err := r.Get(ctx, leaseKey, lease); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return r.Delete(ctx, lease)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LeaderGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&leadershipv1alpha1.LeaderGroup{}).
		Owns(&coordinationv1.Lease{}).
		Complete(r)
}

// deriveLeaseName derives the Lease name from component name.
// This matches zen-sdk/pkg/zenlead.deriveElectionIDFromLeaseName logic.
// Format: "<component-name>-lease"
func deriveLeaseName(component string) string {
	return fmt.Sprintf("%s-lease", component)
}

// statusEqual compares two LeaderGroupStatus for equality.
func statusEqual(a, b leadershipv1alpha1.LeaderGroupStatus) bool {
	if a.HolderIdentity != b.HolderIdentity {
		return false
	}
	if a.ObservedLeaseResourceVersion != b.ObservedLeaseResourceVersion {
		return false
	}
	// Compare times (nil-safe)
	if (a.RenewTime == nil) != (b.RenewTime == nil) {
		return false
	}
	if a.RenewTime != nil && !a.RenewTime.Equal(b.RenewTime) {
		return false
	}
	// Compare durations (nil-safe)
	if (a.LeaseDurationSeconds == nil) != (b.LeaseDurationSeconds == nil) {
		return false
	}
	if a.LeaseDurationSeconds != nil && *a.LeaseDurationSeconds != *b.LeaseDurationSeconds {
		return false
	}
	return true
}

