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

package metrics

import (
	"testing"
)

// resetGlobalRecorder resets the global recorder for testing
func resetGlobalRecorder() {
	globalRecorder = nil
}

func TestNewRecorder(t *testing.T) {
	recorder := NewRecorder()
	if recorder == nil {
		t.Fatal("NewRecorder returned nil")
	}

	// Verify it returns the same instance on subsequent calls
	recorder2 := NewRecorder()
	if recorder != recorder2 {
		t.Error("NewRecorder should return the same instance")
	}
}

func TestRecordLeaderDuration(t *testing.T) {
	// Note: This test verifies the function doesn't panic
	// Due to promauto's global registration, we can't easily test exact values
	// without using a custom registry. For now, we verify the function works.
	recorder := NewRecorder()

	// Record leader duration - should not panic (no pod label for cardinality)
	recorder.RecordLeaderDuration("default", "my-service", 125.5)
	recorder.RecordLeaderDuration("default", "my-service", 250.0)

	// Function executed without panic - test passes
}

func TestRecordFailover(t *testing.T) {
	recorder := NewRecorder()

	// Record failover - should not panic (with reason label)
	recorder.RecordFailover("default", "my-service", "notReady")
	recorder.RecordFailover("default", "my-service", "terminating")

	// Function executed without panic - test passes
}

func TestRecordReconciliationDuration(t *testing.T) {
	recorder := NewRecorder()

	// Record successful reconciliation
	recorder.RecordReconciliationDuration("default", "my-service", "success", 0.5)

	// Record failed reconciliation
	recorder.RecordReconciliationDuration("default", "my-service", "error", 1.0)

	// Verify metrics were recorded (histogram observations)
	// We can't easily check exact values without exposing internals, but we can verify it doesn't panic
}

func TestRecordPodsAvailable(t *testing.T) {
	recorder := NewRecorder()

	// Record pods available - should not panic
	recorder.RecordPodsAvailable("default", "my-service", 3)
	recorder.RecordPodsAvailable("default", "my-service", 5)

	// Function executed without panic - test passes
}

func TestRecordPortResolutionFailure(t *testing.T) {
	recorder := NewRecorder()

	// Record port resolution failure - should not panic
	recorder.RecordPortResolutionFailure("default", "my-service", "http")
	recorder.RecordPortResolutionFailure("default", "my-service", "http")

	// Function executed without panic - test passes
}

func TestRecordReconciliationError(t *testing.T) {
	recorder := NewRecorder()

	// Record reconciliation error - should not panic
	recorder.RecordReconciliationError("default", "my-service", "list_pods_failed")

	// Function executed without panic - test passes
}

func TestResetLeaderDuration(t *testing.T) {
	recorder := NewRecorder()

	// Set leader duration (no pod label)
	recorder.RecordLeaderDuration("default", "my-service", 125.5)

	// Reset it - should not panic (no pod label)
	recorder.ResetLeaderDuration("default", "my-service")

	// Function executed without panic - test passes
}

func TestRecordLeaderServicesTotal(t *testing.T) {
	recorder := NewRecorder()

	// Record leader services total - should not panic
	recorder.RecordLeaderServicesTotal("default", 5)

	// Function executed without panic - test passes
}

func TestRecordEndpointSlicesTotal(t *testing.T) {
	recorder := NewRecorder()

	// Record endpoint slices total - should not panic
	recorder.RecordEndpointSlicesTotal("default", 5)

	// Function executed without panic - test passes
}

func TestRecordStickyLeaderHit(t *testing.T) {
	recorder := NewRecorder()

	// Record sticky leader hit - should not panic
	recorder.RecordStickyLeaderHit("default", "my-service")

	// Function executed without panic - test passes
}

func TestRecordStickyLeaderMiss(t *testing.T) {
	recorder := NewRecorder()

	// Record sticky leader miss - should not panic
	recorder.RecordStickyLeaderMiss("default", "my-service")

	// Function executed without panic - test passes
}

func TestRecordLeaderSelectionAttempt(t *testing.T) {
	recorder := NewRecorder()

	// Record leader selection attempt - should not panic
	recorder.RecordLeaderSelectionAttempt("default", "my-service")

	// Function executed without panic - test passes
}

func TestRecordLeaderPodAge(t *testing.T) {
	recorder := NewRecorder()

	// Record leader pod age - should not panic (no pod label for cardinality)
	recorder.RecordLeaderPodAge("default", "my-service", 3600.0)

	// Function executed without panic - test passes
}

func TestRecordLeaderServiceWithoutEndpoints(t *testing.T) {
	recorder := NewRecorder()

	// Record service without endpoints - should not panic
	recorder.RecordLeaderServiceWithoutEndpoints("default", "my-service", true)
	recorder.RecordLeaderServiceWithoutEndpoints("default", "my-service", false)

	// Function executed without panic - test passes
}

func TestRecordReconciliation(t *testing.T) {
	recorder := NewRecorder()

	// Record successful reconciliation - should not panic
	recorder.RecordReconciliation("default", "my-service", "success")
	recorder.RecordReconciliation("default", "my-service", "error")

	// Function executed without panic - test passes
}
