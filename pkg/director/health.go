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
	"errors"
	"net/http"
)

var (
	// ErrNotReady indicates the controller is not ready
	ErrNotReady = errors.New("controller not ready")
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
	// Controller is healthy if reconciler is properly initialized
	return nil
}
