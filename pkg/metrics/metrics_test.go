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

	"github.com/prometheus/client_golang/prometheus"
)

// resetGlobalRecorder resets the global recorder for testing
func resetGlobalRecorder() { //nolint:unused // used in tests
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

func TestRecordLeaderStable(t *testing.T) {
	recorder := NewRecorder()

	// Record leader stability - should not panic
	recorder.RecordLeaderStable("default", "my-service", true)
	recorder.RecordLeaderStable("default", "my-service", false)

	// Function executed without panic - test passes
}

func TestRecordEndpointWriteError(t *testing.T) {
	recorder := NewRecorder()

	// Record endpoint write error - should not panic
	recorder.RecordEndpointWriteError("default", "my-service")
	recorder.RecordEndpointWriteError("default", "my-service")

	// Function executed without panic - test passes
}

func TestRecordCacheMetrics(t *testing.T) {
	recorder := NewRecorder()

	// Test cache metrics
	recorder.RecordCacheSize("default", 5)
	recorder.RecordCacheSize("default", 10)
	recorder.RecordCacheUpdateDuration("default", 0.5)
	recorder.RecordCacheHit("default")
	recorder.RecordCacheMiss("default")
	recorder.RecordTimeout("default", "cache_update")
	recorder.RecordTimeout("default", "metrics_collection")

	// Function executed without panic - test passes
}

func TestRecordFailoverLatency(t *testing.T) {
	recorder := NewRecorder()
	recorder.RecordFailoverLatency("default", "my-service", "notReady", 0.5)

	// Verify metric was observed
	failoverLatencyMetric := recorder.FailoverLatencySeconds()
	if failoverLatencyMetric == nil {
		t.Fatal("FailoverLatencySeconds() returned nil")
	}

	// Get metric value
	metric, err := failoverLatencyMetric.GetMetricWithLabelValues("default", "my-service", "notReady")
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}

	// Verify histogram value
	histogram := metric.(prometheus.Histogram)
	if histogram == nil {
		t.Fatal("Metric is not a Histogram")
	}
}

func TestRecordAPICallDuration(t *testing.T) {
	recorder := NewRecorder()
	recorder.RecordAPICallDuration("default", "my-service", "get", "success", 0.05)

	// Verify metric was observed
	apiCallDurationMetric := recorder.APICallDurationSeconds()
	if apiCallDurationMetric == nil {
		t.Fatal("APICallDurationSeconds() returned nil")
	}

	// Get metric value
	metric, err := apiCallDurationMetric.GetMetricWithLabelValues("default", "my-service", "get", "success")
	if err != nil {
		t.Fatalf("Failed to get metric: %v", err)
	}

	// Verify histogram value
	histogram := metric.(prometheus.Histogram)
	if histogram == nil {
		t.Fatal("Metric is not a Histogram")
	}
}

func TestMetricsEdgeCases(t *testing.T) {
	recorder := NewRecorder()

	// Test with empty strings
	recorder.RecordFailover("", "", "")
	recorder.RecordReconciliationError("", "", "")
	recorder.RecordPortResolutionFailure("", "", "")
	recorder.RecordLeaderSelectionAttempt("", "")
	recorder.RecordReconciliation("", "", "")

	// Test with negative values (should still work)
	recorder.RecordPodsAvailable("default", "my-service", -1)
	recorder.RecordLeaderServicesTotal("default", -1)
	recorder.RecordEndpointSlicesTotal("default", -1)
	recorder.RecordCacheSize("default", -1)

	// Test with zero values
	recorder.RecordPodsAvailable("default", "my-service", 0)
	recorder.RecordLeaderServicesTotal("default", 0)
	recorder.RecordEndpointSlicesTotal("default", 0)
	recorder.RecordCacheSize("default", 0)

	// Test with very large values
	recorder.RecordPodsAvailable("default", "my-service", 10000)
	recorder.RecordLeaderServicesTotal("default", 10000)
	recorder.RecordCacheSize("default", 10000)

	// Should not panic
}
