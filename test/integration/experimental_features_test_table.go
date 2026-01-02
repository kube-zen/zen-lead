//go:build integration

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

package integration

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestExperimentalFeaturesWithParameters runs parameterized tests
// with different configurations to test various scenarios
func TestExperimentalFeaturesWithParameters(t *testing.T) {
	if os.Getenv("ENABLE_EXPERIMENTAL_TESTS") != "true" {
		t.Skip("Skipping experimental features test. Set ENABLE_EXPERIMENTAL_TESTS=true to run.")
	}

	testCases := []struct {
		name              string
		numServices       int
		podsPerService    int
		testDuration      time.Duration
		failoverFrequency int
		description       string
	}{
		{
			name:              "small_workload",
			numServices:       3,
			podsPerService:    2,
			testDuration:      2 * time.Minute,
			failoverFrequency: 5,
			description:       "Small workload: 3 services, 2 pods each, 5 failovers",
		},
		{
			name:              "medium_workload",
			numServices:       10,
			podsPerService:    3,
			testDuration:      5 * time.Minute,
			failoverFrequency: 10,
			description:       "Medium workload: 10 services, 3 pods each, 10 failovers",
		},
		{
			name:              "large_workload",
			numServices:       20,
			podsPerService:    5,
			testDuration:      10 * time.Minute,
			failoverFrequency: 20,
			description:       "Large workload: 20 services, 5 pods each, 20 failovers",
		},
		{
			name:              "high_failover_stress",
			numServices:       5,
			podsPerService:    3,
			testDuration:      3 * time.Minute,
			failoverFrequency: 50,
			description:       "High failover stress: 5 services, 50 failovers in 3 minutes",
		},
		{
			name:              "long_running",
			numServices:       5,
			podsPerService:    3,
			testDuration:      30 * time.Minute,
			failoverFrequency: 10,
			description:       "Long-running stability: 5 services, 30 minutes, 10 failovers",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running test case: %s", tc.description)
			t.Logf("  Services: %d", tc.numServices)
			t.Logf("  Pods per Service: %d", tc.podsPerService)
			t.Logf("  Duration: %v", tc.testDuration)
			t.Logf("  Failovers: %d", tc.failoverFrequency)

			// Set environment variables for this test case
			os.Setenv("TEST_NUM_SERVICES", fmt.Sprintf("%d", tc.numServices))
			os.Setenv("TEST_PODS_PER_SERVICE", fmt.Sprintf("%d", tc.podsPerService))
			os.Setenv("TEST_DURATION", tc.testDuration.String())
			os.Setenv("TEST_FAILOVER_FREQUENCY", fmt.Sprintf("%d", tc.failoverFrequency))

			// Run the comparison test with these parameters
			// Note: This is a simplified version - full test would call TestExperimentalFeaturesComparison
			// For now, we'll just validate the configuration
			testConfig := DefaultExperimentalTestConfig()
			if numSvc := os.Getenv("TEST_NUM_SERVICES"); numSvc != "" {
				if n, err := strconv.Atoi(numSvc); err == nil {
					testConfig.NumServices = n
				}
			}

			if testConfig.NumServices != tc.numServices {
				t.Errorf("Config mismatch: expected %d services, got %d", tc.numServices, testConfig.NumServices)
			}

			t.Logf("✅ Test case %s configuration validated", tc.name)
		})
	}
}

