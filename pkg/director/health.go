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
	"errors"
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// ErrNotReady indicates the controller is not ready
	ErrNotReady = errors.New("controller not ready")
	// ErrAPIConnectionFailed indicates API server connectivity issue
	ErrAPIConnectionFailed = errors.New("API server connection failed")
)

// ControllerHealthChecker provides health check functionality for the ServiceDirector controller
type ControllerHealthChecker struct {
	reconciler *ServiceDirectorReconciler
}

// NewControllerHealthChecker creates a new health checker for the ServiceDirector controller
func NewControllerHealthChecker(reconciler *ServiceDirectorReconciler) *ControllerHealthChecker {
	return &ControllerHealthChecker{
		reconciler: reconciler,
	}
}

// Check verifies that the controller can reconcile Services
// Returns nil if healthy, error if unhealthy
func (c *ControllerHealthChecker) Check(req *http.Request) error {
	// Basic health check: verify reconciler is initialized
	if c.reconciler == nil {
		return ErrNotReady
	}
	if c.reconciler.Client == nil {
		return ErrNotReady
	}
	if c.reconciler.Metrics == nil {
		return ErrNotReady
	}

	// Enhanced health check: verify API server connectivity
	// Use a short timeout to avoid blocking the health check
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()

	// Test API connectivity by listing namespaces (lightweight operation)
	nsList := &corev1.NamespaceList{}
	if err := c.reconciler.Client.List(ctx, nsList, client.Limit(1)); err != nil {
		return fmt.Errorf("%w: %v", ErrAPIConnectionFailed, err)
	}

	// Verify cache is initialized (if enabled)
	if c.reconciler.leaderPodCache != nil {
		// Cache is initialized, that's sufficient
		// We don't check cache size here as it's dynamic
	}

	// Controller is healthy if reconciler is properly initialized and API is reachable
	return nil
}
