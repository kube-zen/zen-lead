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

	"github.com/kube-zen/zen-sdk/pkg/retry"
	"github.com/kube-zen/zen-lead/pkg/metrics"
)

// retryDoWithMetrics wraps retry.Do to record retry attempt metrics.
// It tracks the number of attempts and records metrics for observability.
func retryDoWithMetrics(ctx context.Context, cfg retry.Config, fn func() error, recorder *metrics.Recorder, namespace, service, operation string) error {
	if recorder == nil {
		// Fallback to regular retry if metrics recorder is not available
		return retry.Do(ctx, cfg, fn)
	}

	var attemptCount int
	succeededAfterRetry := false

	// Wrap the function to track attempts
	// Note: succeededAfterRetry is captured by reference in the closure, so modifications
	// inside wrappedFn are visible outside after retry.Do completes.
	wrappedFn := func() error {
		attemptCount++
		err := fn()

		// Record attempt metric
		if attemptCount == 1 {
			if err != nil {
				recorder.RecordRetryAttempt(namespace, service, operation, "1")
			}
			return err
		}

		// Subsequent attempts
		attemptLabel := fmt.Sprintf("%d", attemptCount)
		if attemptCount >= cfg.MaxAttempts {
			attemptLabel = "max"
		}
		recorder.RecordRetryAttempt(namespace, service, operation, attemptLabel)

		// If this attempt succeeds and it's not the first attempt, record success after retry
		// This flag is captured by reference, so the change is visible after retry.Do returns
		if err == nil && attemptCount > 1 {
			succeededAfterRetry = true
		}

		return err
	}

	// Use zen-sdk retry with wrapped function
	err := retry.Do(ctx, cfg, wrappedFn)

	// Record success after retry if applicable
	// Note: succeededAfterRetry was set inside the closure but is accessible here
	// because closures capture variables by reference in Go
	if succeededAfterRetry {
		recorder.RecordRetrySuccessAfterRetry(namespace, service, operation)
	}

	// If we exhausted all attempts and still failed, record max attempt
	if err != nil && attemptCount >= cfg.MaxAttempts {
		recorder.RecordRetryAttempt(namespace, service, operation, "max")
	}

	return err
}

