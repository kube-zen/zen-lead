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

// Package client provides a simple API for checking leader status in zen-lead.
//
// This is the "Simple Query" API that tools use to ask "Am I the leader?"
// without needing to understand the underlying Lease resources or leader election logic.
//
// Usage:
//
//	import "github.com/kube-zen/zen-lead/pkg/client"
//
//	// Create client
//	zenleadClient, err := client.NewClient(mgr.GetClient())
//	if err != nil {
//		// handle error
//	}
//
//	// Check if this pod is the leader
//	isLeader, err := zenleadClient.IsLeader(ctx, "zen-flow-pool")
//	if err != nil {
//		// handle error (or use fail-safe: assume leader)
//	}
//
//	if !isLeader {
//		// Skip processing - not the leader
//		return reconcile.Result{}, nil
//	}
//
//	// Proceed with leader-only logic
//
// Fail-Safe Behavior:
//
// If zen-lead is not installed (Lease doesn't exist), IsLeader() returns true.
// This allows applications to work without zen-lead, assuming single-replica mode.
//
// If pod name cannot be determined (local development), IsLeader() returns true.
// This allows local development without Kubernetes.
package client

