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
	"strings"
	"time"

	coordinationv1alpha1 "github.com/kube-zen/zen-lead/pkg/apis/coordination.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lead/pkg/pool"
	corev1 "k8s.io/api/core/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// LeaderPolicyReconciler reconciles a LeaderPolicy object
type LeaderPolicyReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	PoolMgr   *pool.Manager
}

//+kubebuilder:rbac:groups=coordination.kube-zen.io,resources=leaderpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=coordination.kube-zen.io,resources=leaderpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=coordination.kube-zen.io,resources=leaderpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *LeaderPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the LeaderPolicy
	policy := &coordinationv1alpha1.LeaderPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Set defaults if not specified
	spec := policy.Spec
	if spec.LeaseDurationSeconds == 0 {
		spec.LeaseDurationSeconds = 15
	}
	if spec.RenewDeadlineSeconds == 0 {
		spec.RenewDeadlineSeconds = 10
	}
	if spec.RetryPeriodSeconds == 0 {
		spec.RetryPeriodSeconds = 2
	}
	if spec.IdentityStrategy == "" {
		spec.IdentityStrategy = "pod"
	}
	if spec.FollowerMode == "" {
		spec.FollowerMode = "standby"
	}

	// Find all candidates for this pool
	candidates, err := r.PoolMgr.FindCandidates(ctx, req.Namespace, policy.Name)
	if err != nil {
		logger.Error(err, "Failed to find candidates")
		return ctrl.Result{}, err
	}

	// Get the current lease
	lease := &coordinationv1.Lease{}
	leaseName := types.NamespacedName{
		Name:      policy.Name,
		Namespace: req.Namespace,
	}

	err = r.Get(ctx, leaseName, lease)
	if err != nil && client.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to get lease")
		return ctrl.Result{}, err
	}

	// Determine current leader from lease
	var currentLeader *coordinationv1alpha1.LeaderHolder
	phase := "Electing"

	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity != "" {
		// Find the leader pod
		leaderIdentity := *lease.Spec.HolderIdentity
		for i := range candidates {
			candidate := &candidates[i]
			// Match identity - check if identity matches pod name or pod-name-uid format
			candidateIdentity := candidate.Name
			if candidateIdentity == leaderIdentity || 
			   strings.HasPrefix(leaderIdentity, candidateIdentity+"-") ||
			   fmt.Sprintf("%s-%s", candidate.Name, string(candidate.UID)) == leaderIdentity {
				currentLeader = &coordinationv1alpha1.LeaderHolder{
					Name:      candidate.Name,
					UID:       string(candidate.UID),
					Namespace: candidate.Namespace,
					StartTime: func() metav1.Time {
						if lease.Spec.AcquireTime != nil {
							return *lease.Spec.AcquireTime
						}
						return metav1.Time{Time: time.Now()}
					}(),
				}
				phase = "Stable"

				// Update pod role annotation
				if err := r.PoolMgr.UpdatePodRole(ctx, candidate, pool.RoleLeader); err != nil {
					logger.V(1).Info("Failed to update pod role", "error", err)
				}
				break
			}
		}

		// Mark all other candidates as followers
		for i := range candidates {
			candidate := &candidates[i]
			if currentLeader == nil || candidate.Name != currentLeader.Name {
				if err := r.PoolMgr.UpdatePodRole(ctx, candidate, pool.RoleFollower); err != nil {
					logger.V(1).Info("Failed to update pod role", "error", err)
				}
			}
		}
	} else {
		// No leader yet - mark all as followers
		for i := range candidates {
			candidate := &candidates[i]
			if err := r.PoolMgr.UpdatePodRole(ctx, candidate, pool.RoleFollower); err != nil {
				logger.V(1).Info("Failed to update pod role", "error", err)
			}
		}
	}

	// Update status
	policy.Status.Phase = phase
	policy.Status.CurrentHolder = currentLeader
	policy.Status.Candidates = int32(len(candidates))
	policy.Status.LastTransitionTime = metav1.Now()

	// Update conditions
	conditions := []metav1.Condition{
		{
			Type:               "LeaderElected",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "LeaderActive",
			Message:            fmt.Sprintf("Leader is %s", func() string {
				if currentLeader != nil {
					return currentLeader.Name
				}
				return "none"
			}()),
		},
	}

	if len(candidates) == 0 {
		conditions = append(conditions, metav1.Condition{
			Type:               "CandidatesAvailable",
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             "NoCandidates",
			Message:            "No pods found with zen-lead/pool annotation",
		})
	} else {
		conditions = append(conditions, metav1.Condition{
			Type:               "CandidatesAvailable",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "CandidatesFound",
			Message:            fmt.Sprintf("%d candidates found", len(candidates)),
		})
	}

	policy.Status.Conditions = conditions

	// Update status
	if err := r.Status().Update(ctx, policy); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Requeue to keep status updated
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *LeaderPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&coordinationv1alpha1.LeaderPolicy{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return nil
				}

				poolName, ok := pool.GetPoolFromPod(pod)
				if !ok {
					return nil
				}

				// Only watch pods that are participating
				if !pool.IsParticipating(pod) {
					return nil
				}

				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: pod.Namespace,
							Name:      poolName,
						},
					},
				}
			}),
		).
		Complete(r)
}

