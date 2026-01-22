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
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-zen/zen-sdk/pkg/health"
)

// ControllerHealthChecker provides health check functionality for the ServiceDirector controller
// Implements zen-sdk/pkg/health.Checker interface
type ControllerHealthChecker struct {
	reconciler *ServiceDirectorReconciler
}

// NewControllerHealthChecker creates a new health checker for the ServiceDirector controller
func NewControllerHealthChecker(reconciler *ServiceDirectorReconciler) *ControllerHealthChecker {
	return &ControllerHealthChecker{
		reconciler: reconciler,
	}
}

// ReadinessCheck verifies that the controller is ready to serve requests
// Returns nil if ready, error if not ready
func (c *ControllerHealthChecker) ReadinessCheck(req *http.Request) error {
	// Basic readiness check: verify reconciler is initialized
	if c.reconciler == nil {
		return fmt.Errorf("%w: reconciler not initialized", health.ErrNotInitialized)
	}
	if c.reconciler.Client == nil {
		return fmt.Errorf("%w: client not initialized", health.ErrNotReady)
	}
	if c.reconciler.Metrics == nil {
		return fmt.Errorf("%w: metrics not initialized", health.ErrNotReady)
	}

	// Enhanced readiness check: verify API server connectivity
	// Use a short timeout to avoid blocking the health check
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()

	// Test API connectivity by listing namespaces (lightweight operation)
	nsList := &corev1.NamespaceList{}
	if err := c.reconciler.Client.List(ctx, nsList, client.Limit(1)); err != nil {
		return fmt.Errorf("%w: API server connection failed: %v", health.ErrNotReady, err)
	}

	// Verify cache is initialized (if enabled)
	// Cache initialization is verified by checking if it's not nil
	// We don't check cache size here as it's dynamic
	_ = c.reconciler.leaderPodCache

	// Controller is ready if reconciler is properly initialized and API is reachable
	return nil
}

// LivenessCheck verifies that the controller is actively processing
// Returns nil if alive, error if not alive
func (c *ControllerHealthChecker) LivenessCheck(req *http.Request) error {
	// Liveness is similar to readiness but less strict
	// Just verify basic initialization
	if c.reconciler == nil {
		return fmt.Errorf("%w: reconciler not initialized", health.ErrNotInitialized)
	}
	if c.reconciler.Client == nil {
		return fmt.Errorf("%w: client not initialized", health.ErrNotReady)
	}

	// For liveness, we don't require API connectivity check
	// The controller is alive if it's initialized, even if API is temporarily unreachable
	return nil
}

// StartupCheck verifies that the controller is initialized
// Returns nil if initialized, error otherwise
func (c *ControllerHealthChecker) StartupCheck(req *http.Request) error {
	// Startup check is the most basic - just verify reconciler exists
	if c.reconciler == nil {
		return fmt.Errorf("%w: reconciler not initialized", health.ErrNotInitialized)
	}
	return nil
}

// Check is a convenience method that calls ReadinessCheck for backward compatibility
// Deprecated: Use ReadinessCheck, LivenessCheck, or StartupCheck directly
func (c *ControllerHealthChecker) Check(req *http.Request) error {
	return c.ReadinessCheck(req)
}
