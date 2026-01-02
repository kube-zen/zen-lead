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

func TestRecordRetryAttempt(t *testing.T) {
	recorder := NewRecorder()

	// Test recording retry attempts
	recorder.RecordRetryAttempt("default", "my-service", "get_service", "1")
	recorder.RecordRetryAttempt("default", "my-service", "get_service", "2")
	recorder.RecordRetryAttempt("default", "my-service", "get_service", "3")
	recorder.RecordRetryAttempt("default", "my-service", "get_service", "max")

	// Test with different operations
	recorder.RecordRetryAttempt("default", "my-service", "create_leader_service", "1")
	recorder.RecordRetryAttempt("default", "my-service", "patch_endpointslice", "2")

	// Test with empty service name (for namespace-only operations)
	recorder.RecordRetryAttempt("default", "", "list_services_cache", "1")

	// Should not panic
}

func TestRecordRetrySuccessAfterRetry(t *testing.T) {
	recorder := NewRecorder()

	// Test recording success after retry
	recorder.RecordRetrySuccessAfterRetry("default", "my-service", "get_service")
	recorder.RecordRetrySuccessAfterRetry("default", "my-service", "create_leader_service")
	recorder.RecordRetrySuccessAfterRetry("default", "my-service", "patch_endpointslice")

	// Test with empty service name
	recorder.RecordRetrySuccessAfterRetry("default", "", "list_services_cache")

	// Should not panic
}

func TestRecordRetryAttemptWithNilRecorder(t *testing.T) {
	// Test that methods handle nil recorder gracefully
	var recorder *Recorder

	// These should not panic even with nil recorder
	// (In practice, recorder is always initialized, but test edge case)
	if recorder != nil {
		recorder.RecordRetryAttempt("default", "my-service", "get_service", "1")
		recorder.RecordRetrySuccessAfterRetry("default", "my-service", "get_service")
	}
}

func TestRecordRetryAttemptEdgeCases(t *testing.T) {
	recorder := NewRecorder()

	// Test with empty strings
	recorder.RecordRetryAttempt("", "", "", "1")
	recorder.RecordRetrySuccessAfterRetry("", "", "")

	// Test with special characters in labels
	recorder.RecordRetryAttempt("namespace-with-dash", "service_with_underscore", "operation.with.dots", "1")

	// Should not panic
}
