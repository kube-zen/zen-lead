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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// MetricsResult holds collected metrics for comparison
type MetricsResult struct {
	ReconciliationLatencyP50 float64
	ReconciliationLatencyP95 float64
	ReconciliationLatencyP99 float64
	FailoverLatencyP50       float64
	FailoverLatencyP95       float64
	FailoverLatencyP99       float64
	APICallLatencyP50        float64
	APICallLatencyP95        float64
	CacheHitRate             float64
	ErrorRate                float64
	GCStats                  *GCStats
}

// GCStats holds garbage collector statistics
type GCStats struct {
	NumGC        int64
	PauseTotal   float64
	PauseAvg     float64
	PauseMax     float64
	AllocTotal   int64
	AllocCurrent int64
}

// scrapeMetrics scrapes Prometheus metrics from a pod's metrics endpoint
func scrapeMetrics(ctx context.Context, c client.Client, namespace, podLabelSelector, metricsPort string) (string, error) {
	// Find pod by label selector
	podList := &corev1.PodList{}
	if err := c.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabels{
		"app.kubernetes.io/name": podLabelSelector,
	}); err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pods found with label %s", podLabelSelector)
	}

	pod := podList.Items[0]
	metricsURL := fmt.Sprintf("http://%s:%s/metrics", pod.Status.PodIP, metricsPort)

	// In real scenario, we'd use port-forward or service
	// For now, this is a placeholder that shows the structure
	req, err := http.NewRequestWithContext(ctx, "GET", metricsURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read metrics: %w", err)
	}

	return string(body), nil
}

// parsePrometheusMetrics parses Prometheus metrics text format
func parsePrometheusMetrics(metricsText string) map[string]float64 {
	result := make(map[string]float64)
	lines := strings.Split(metricsText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Parse metric line: metric_name{labels} value
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		metricName := strings.Split(parts[0], "{")[0]
		value, err := strconv.ParseFloat(parts[len(parts)-1], 64)
		if err != nil {
			continue
		}

		result[metricName] = value
	}

	return result
}

// collectMetrics collects metrics from a controller deployment
func collectMetrics(ctx context.Context, c client.Client, namespace, deploymentName, metricsPort string) (*MetricsResult, error) {
	// Scrape metrics
	metricsText, err := scrapeMetrics(ctx, c, namespace, deploymentName, metricsPort)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape metrics: %w", err)
	}

	metrics := parsePrometheusMetrics(metricsText)

	result := &MetricsResult{}

	// Extract reconciliation latency (histogram quantiles)
	// In real implementation, would query Prometheus for quantiles
	if val, ok := metrics["zen_lead_reconciliation_duration_seconds"]; ok {
		result.ReconciliationLatencyP50 = val
	}

	// Extract failover latency
	if val, ok := metrics["zen_lead_failover_latency_seconds"]; ok {
		result.FailoverLatencyP50 = val
	}

	// Calculate cache hit rate
	cacheHits := metrics["zen_lead_cache_hits_total"]
	cacheMisses := metrics["zen_lead_cache_misses_total"]
	if cacheHits+cacheMisses > 0 {
		result.CacheHitRate = cacheHits / (cacheHits + cacheMisses)
	}

	// Calculate error rate
	errors := metrics["zen_lead_reconciliation_errors_total"]
	reconciliations := metrics["zen_lead_reconciliations_total"]
	if reconciliations > 0 {
		result.ErrorRate = errors / reconciliations
	}

	return result, nil
}

// compareMetrics compares metrics between standard and experimental deployments
func compareMetrics(standard, experimental *MetricsResult) string {
	var report strings.Builder

	report.WriteString("=== Performance Comparison ===\n\n")

	// Reconciliation latency
	if standard.ReconciliationLatencyP50 > 0 && experimental.ReconciliationLatencyP50 > 0 {
		improvement := ((standard.ReconciliationLatencyP50 - experimental.ReconciliationLatencyP50) / standard.ReconciliationLatencyP50) * 100
		report.WriteString(fmt.Sprintf("Reconciliation Latency (P50):\n"))
		report.WriteString(fmt.Sprintf("  Standard:     %.3f ms\n", standard.ReconciliationLatencyP50*1000))
		report.WriteString(fmt.Sprintf("  Experimental: %.3f ms\n", experimental.ReconciliationLatencyP50*1000))
		report.WriteString(fmt.Sprintf("  Improvement:   %.1f%%\n\n", improvement))
	}

	// Failover latency
	if standard.FailoverLatencyP50 > 0 && experimental.FailoverLatencyP50 > 0 {
		improvement := ((standard.FailoverLatencyP50 - experimental.FailoverLatencyP50) / standard.FailoverLatencyP50) * 100
		report.WriteString(fmt.Sprintf("Failover Latency (P50):\n"))
		report.WriteString(fmt.Sprintf("  Standard:     %.3f ms\n", standard.FailoverLatencyP50*1000))
		report.WriteString(fmt.Sprintf("  Experimental: %.3f ms\n", experimental.FailoverLatencyP50*1000))
		report.WriteString(fmt.Sprintf("  Improvement:   %.1f%%\n\n", improvement))
	}

	// Cache hit rate
	report.WriteString(fmt.Sprintf("Cache Hit Rate:\n"))
	report.WriteString(fmt.Sprintf("  Standard:     %.2f%%\n", standard.CacheHitRate*100))
	report.WriteString(fmt.Sprintf("  Experimental: %.2f%%\n\n", experimental.CacheHitRate*100))

	// Error rate
	report.WriteString(fmt.Sprintf("Error Rate:\n"))
	report.WriteString(fmt.Sprintf("  Standard:     %.4f%%\n", standard.ErrorRate*100))
	report.WriteString(fmt.Sprintf("  Experimental: %.4f%%\n\n", experimental.ErrorRate*100))

	return report.String()
}

// ExperimentalTestConfig holds test configuration parameters
type ExperimentalTestConfig struct {
	NumServices       int
	PodsPerService    int
	TestDuration      time.Duration
	FailoverFrequency int // Number of failovers to trigger
	EnableJSONv2      bool
	EnableGreenTeaGC  bool
}

// DefaultExperimentalTestConfig returns default test configuration
func DefaultExperimentalTestConfig() ExperimentalTestConfig {
	return ExperimentalTestConfig{
		NumServices:       5,
		PodsPerService:    3,
		TestDuration:      5 * time.Minute,
		FailoverFrequency: 10,
		EnableJSONv2:      false,
		EnableGreenTeaGC:  false,
	}
}

// TestExperimentalFeaturesComparison runs performance and stability tests
// comparing standard build vs experimental features (jsonv2, greenteagc)
//
// This test requires:
// 1. Two controller deployments: one standard, one with experimental features
// 2. Same workload pattern applied to both
// 3. Metrics collection and comparison
//
// Usage:
//   - Build standard image: docker build -t kubezen/zen-lead:standard .
//   - Build experimental: docker build --build-arg GOEXPERIMENT=jsonv2,greenteagc -t kubezen/zen-lead:experimental .
//   - Deploy both and run this test
func TestExperimentalFeaturesComparison(t *testing.T) {
	// Parse test configuration from environment
	testConfig := DefaultExperimentalTestConfig()
	if numSvc := os.Getenv("TEST_NUM_SERVICES"); numSvc != "" {
		if n, err := strconv.Atoi(numSvc); err == nil {
			testConfig.NumServices = n
		}
	}
	if numPods := os.Getenv("TEST_PODS_PER_SERVICE"); numPods != "" {
		if n, err := strconv.Atoi(numPods); err == nil {
			testConfig.PodsPerService = n
		}
	}
	if duration := os.Getenv("TEST_DURATION"); duration != "" {
		if d, err := time.ParseDuration(duration); err == nil {
			testConfig.TestDuration = d
		}
	}
	if failovers := os.Getenv("TEST_FAILOVER_FREQUENCY"); failovers != "" {
		if n, err := strconv.Atoi(failovers); err == nil {
			testConfig.FailoverFrequency = n
		}
	}

	t.Logf("Test Configuration:")
	t.Logf("  Services: %d", testConfig.NumServices)
	t.Logf("  Pods per Service: %d", testConfig.PodsPerService)
	t.Logf("  Test Duration: %v", testConfig.TestDuration)
	t.Logf("  Failover Frequency: %d", testConfig.FailoverFrequency)
	if os.Getenv("ENABLE_EXPERIMENTAL_TESTS") != "true" {
		t.Skip("Skipping experimental features test. Set ENABLE_EXPERIMENTAL_TESTS=true to run.")
	}

	// Get kubeconfig
	cfg, err := config.GetConfig()
	if err != nil {
		t.Fatalf("Failed to get kubeconfig: %v", err)
	}

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	namespace := "zen-lead-experimental-test"

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = c.Delete(ctx, ns)
	}()

	// Test workload: Create multiple services with zen-lead enabled
	testServices := make([]string, testConfig.NumServices)
	for i := 0; i < testConfig.NumServices; i++ {
		testServices[i] = fmt.Sprintf("test-app-%d", i+1)
	}

	t.Logf("Creating %d test services...", len(testServices))
	for _, svcName := range testServices {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: namespace,
				Annotations: map[string]string{
					"zen-lead.io/enabled": "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": svcName,
				},
				Ports: []corev1.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt32(8080),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}
		if err := c.Create(ctx, svc); err != nil {
			t.Fatalf("Failed to create service %s: %v", svcName, err)
		}
	}

	// Wait for reconciliation to complete
	if err := wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		// Check that leader services were created
		leaderSvcList := &corev1.ServiceList{}
		if err := c.List(ctx, leaderSvcList, client.InNamespace(namespace), client.MatchingLabels{
			"app.kubernetes.io/managed-by": "zen-lead",
		}); err != nil {
			return false, err
		}
		return len(leaderSvcList.Items) >= len(testServices), nil
	}); err != nil {
		t.Fatalf("Failed to wait for reconciliation: %v", err)
	}

	// Create pods for each service to enable leader selection
	t.Logf("Creating %d pods per service...", testConfig.PodsPerService)
	for _, svcName := range testServices {
		for i := 0; i < testConfig.PodsPerService; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-pod-%d", svcName, i+1),
					Namespace: namespace,
					Labels: map[string]string{
						"app": svcName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx:latest",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "http",
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:               corev1.PodReady,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
						},
					},
					PodIP: fmt.Sprintf("10.0.0.%d", i+1),
				},
			}
			if err := c.Create(ctx, pod); err != nil {
				t.Logf("Warning: Failed to create pod %s: %v", pod.Name, err)
			}
		}
	}

	// Wait for reconciliation to complete
	t.Log("Waiting for initial reconciliation...")
	if err := wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		// Check that leader services were created
		leaderSvcList := &corev1.ServiceList{}
		if err := c.List(ctx, leaderSvcList, client.InNamespace(namespace), client.MatchingLabels{
			"app.kubernetes.io/managed-by": "zen-lead",
		}); err != nil {
			return false, err
		}
		return len(leaderSvcList.Items) >= len(testServices), nil
	}); err != nil {
		t.Fatalf("Failed to wait for reconciliation: %v", err)
	}

	// Trigger failovers if configured
	if testConfig.FailoverFrequency > 0 {
		t.Logf("Triggering %d failovers for stress testing...", testConfig.FailoverFrequency)
		for i := 0; i < testConfig.FailoverFrequency && i < len(testServices); i++ {
			svcName := testServices[i]
			// Get current leader pod and delete it to trigger failover
			leaderSvc := &corev1.Service{}
			leaderSvcName := fmt.Sprintf("%s-leader", svcName)
			if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: leaderSvcName}, leaderSvc); err == nil {
				if leaderPodName, ok := leaderSvc.Annotations["zen-lead.io/leader-pod-name"]; ok {
					pod := &corev1.Pod{}
					if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: leaderPodName}, pod); err == nil {
						if err := c.Delete(ctx, pod); err == nil {
							t.Logf("Deleted leader pod %s to trigger failover", leaderPodName)
							time.Sleep(2 * time.Second) // Wait for failover
						}
					}
				}
			}
		}
	}

	// Allow metrics to accumulate
	t.Logf("Collecting metrics for %v...", testConfig.TestDuration)
	time.Sleep(testConfig.TestDuration)

	// Collect metrics from standard deployment
	standardNamespace := os.Getenv("STANDARD_DEPLOYMENT_NAMESPACE")
	if standardNamespace == "" {
		standardNamespace = "zen-lead-standard"
	}
	standardMetrics, err := collectMetrics(ctx, c, standardNamespace, "zen-lead", "8080")
	if err != nil {
		t.Logf("Warning: Failed to collect standard metrics: %v", err)
		t.Logf("Note: Ensure standard deployment is running in namespace %s", standardNamespace)
		standardMetrics = &MetricsResult{} // Use empty metrics for comparison
	}

	// Collect metrics from experimental deployment
	experimentalNamespace := os.Getenv("EXPERIMENTAL_DEPLOYMENT_NAMESPACE")
	if experimentalNamespace == "" {
		experimentalNamespace = "zen-lead-experimental"
	}
	experimentalMetrics, err := collectMetrics(ctx, c, experimentalNamespace, "zen-lead", "8080")
	if err != nil {
		t.Logf("Warning: Failed to collect experimental metrics: %v", err)
		t.Logf("Note: Ensure experimental deployment is running in namespace %s", experimentalNamespace)
		experimentalMetrics = &MetricsResult{} // Use empty metrics for comparison
	}

	// Compare metrics
	comparison := compareMetrics(standardMetrics, experimentalMetrics)
	t.Log(comparison)

	// Save comparison to file for documentation
	if os.Getenv("SAVE_COMPARISON_REPORT") == "true" {
		reportFile := os.Getenv("COMPARISON_REPORT_FILE")
		if reportFile == "" {
			reportFile = "/tmp/experimental_features_comparison.txt"
		}
		if err := os.WriteFile(reportFile, []byte(comparison), 0644); err != nil {
			t.Logf("Warning: Failed to save comparison report: %v", err)
		} else {
			t.Logf("Comparison report saved to: %s", reportFile)
		}
	}
}

// BenchmarkReconciliationLatency benchmarks reconciliation performance
// with and without experimental features
func BenchmarkReconciliationLatency(b *testing.B) {
	if os.Getenv("ENABLE_EXPERIMENTAL_TESTS") != "true" {
		b.Skip("Skipping experimental features benchmark. Set ENABLE_EXPERIMENTAL_TESTS=true to run.")
	}

	// Placeholder for reconciliation latency benchmarks
	// Would measure:
	// - Time to reconcile service
	// - Time to create/update EndpointSlice
	// - API call latency
	// - JSON serialization time (if jsonv2 enabled)
}

// BenchmarkFailoverTime benchmarks failover performance
// with and without experimental features
func BenchmarkFailoverTime(b *testing.B) {
	if os.Getenv("ENABLE_EXPERIMENTAL_TESTS") != "true" {
		b.Skip("Skipping experimental features benchmark. Set ENABLE_EXPERIMENTAL_TESTS=true to run.")
	}

	// Placeholder for failover time benchmarks
	// Would measure:
	// - Time from leader unhealthy to new leader selected
	// - EndpointSlice update latency
	// - GC pause impact on failover (if greenteagc enabled)
}

// TestStability runs stability tests with experimental features
// to ensure no regressions
func TestStability(t *testing.T) {
	if os.Getenv("ENABLE_EXPERIMENTAL_TESTS") != "true" {
		t.Skip("Skipping experimental features stability test. Set ENABLE_EXPERIMENTAL_TESTS=true to run.")
	}

	// Placeholder for stability tests
	// Would test:
	// - Long-running reconciliation (memory leaks)
	// - High-frequency failovers (stress test)
	// - Concurrent service updates
	// - Error handling and recovery
}
