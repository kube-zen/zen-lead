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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Recorder provides zen-lead-specific Prometheus metrics
type Recorder struct {
	// Zen-lead specific metrics
	leaderDurationSeconds         *prometheus.GaugeVec
	failoverCountTotal            *prometheus.CounterVec
	reconciliationDurationSeconds *prometheus.HistogramVec
	podsAvailable                 *prometheus.GaugeVec
	portResolutionFailuresTotal   *prometheus.CounterVec
	reconciliationErrorsTotal     *prometheus.CounterVec
	leaderServicesTotal           *prometheus.GaugeVec
	endpointSlicesTotal           *prometheus.GaugeVec
	stickyLeaderHitsTotal         *prometheus.CounterVec
	stickyLeaderMissesTotal       *prometheus.CounterVec
	leaderSelectionAttemptsTotal  *prometheus.CounterVec
	leaderPodAgeSeconds           *prometheus.GaugeVec
	leaderServiceWithoutEndpoints *prometheus.GaugeVec
	reconciliationsTotal          *prometheus.CounterVec
}

var (
	// Global recorder instance
	globalRecorder *Recorder
)

// ResetGlobalRecorder resets the global recorder (for testing only)
func ResetGlobalRecorder() {
	globalRecorder = nil
}

// NewRecorder creates a new metrics recorder for zen-lead
func NewRecorder() *Recorder {
	if globalRecorder != nil {
		return globalRecorder
	}

	// Create zen-lead-specific metrics
	recorder := &Recorder{

		// Leader duration: how long a pod has been the leader (H011.8: no pod label for cardinality)
		leaderDurationSeconds: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "zen_lead_leader_duration_seconds",
				Help: "Duration in seconds that the current leader pod has been the leader",
			},
			[]string{"namespace", "service"},
		),

		// Failover count: total number of leader changes
		failoverCountTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zen_lead_failover_count_total",
				Help: "Total number of leader failovers (leader changes)",
			},
			[]string{"namespace", "service"},
		),

		// Reconciliation duration: duration of reconciliation loops
		reconciliationDurationSeconds: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "zen_lead_reconciliation_duration_seconds",
				Help:    "Duration of reconciliation loops in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
			},
			[]string{"namespace", "service", "result"}, // result: success, error
		),

		// Pods available: number of Ready pods available for leader selection
		podsAvailable: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "zen_lead_pods_available",
				Help: "Number of Ready pods available for leader selection",
			},
			[]string{"namespace", "service"},
		),

		// Port resolution failures: failures in resolving named targetPorts
		portResolutionFailuresTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zen_lead_port_resolution_failures_total",
				Help: "Total number of port resolution failures (named targetPort)",
			},
			[]string{"namespace", "service", "port_name"},
		),

		// Reconciliation errors: total number of reconciliation errors
		reconciliationErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zen_lead_reconciliation_errors_total",
				Help: "Total number of reconciliation errors",
			},
			[]string{"namespace", "service", "error_type"},
		),

		// Leader services: total number of leader Services managed
		leaderServicesTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "zen_lead_leader_services_total",
				Help: "Total number of leader Services currently managed",
			},
			[]string{"namespace"},
		),

		// EndpointSlices: total number of EndpointSlices managed
		endpointSlicesTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "zen_lead_endpointslices_total",
				Help: "Total number of EndpointSlices currently managed",
			},
			[]string{"namespace"},
		),

		// Sticky leader hits: when sticky leader was kept (no change)
		stickyLeaderHitsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zen_lead_sticky_leader_hits_total",
				Help: "Total number of times sticky leader was kept (no leader change)",
			},
			[]string{"namespace", "service"},
		),

		// Sticky leader misses: when sticky leader was not available and new leader selected
		stickyLeaderMissesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zen_lead_sticky_leader_misses_total",
				Help: "Total number of times sticky leader was not available (new leader selected)",
			},
			[]string{"namespace", "service"},
		),

		// Leader selection attempts: total number of leader selection operations
		leaderSelectionAttemptsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zen_lead_leader_selection_attempts_total",
				Help: "Total number of leader selection attempts",
			},
			[]string{"namespace", "service"},
		),

		// Leader pod age: age of the current leader pod in seconds (H011.8: no pod label for cardinality)
		leaderPodAgeSeconds: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "zen_lead_leader_pod_age_seconds",
				Help: "Age of the current leader pod in seconds (since pod creation)",
			},
			[]string{"namespace", "service"},
		),

		// Leader service without endpoints: leader Services that have no endpoints
		leaderServiceWithoutEndpoints: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "zen_lead_leader_service_without_endpoints",
				Help: "Leader Services that have no endpoints (1 = no endpoints, 0 = has endpoints)",
			},
			[]string{"namespace", "service"},
		),

		// Reconciliations total: total number of reconciliations
		reconciliationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zen_lead_reconciliations_total",
				Help: "Total number of reconciliations",
			},
			[]string{"namespace", "service", "result"},
		),
	}

	globalRecorder = recorder
	return recorder
}

// RecordLeaderDuration records how long the current leader pod has been the leader (H011.8: no pod label).
// Call this periodically (e.g., on each reconciliation) to update the metric.
// Leader identity is exposed via leader Service annotations (zen-lead.io/leader-pod-name, etc.)
func (r *Recorder) RecordLeaderDuration(namespace, service string, durationSeconds float64) {
	r.leaderDurationSeconds.WithLabelValues(namespace, service).Set(durationSeconds)
}

// RecordFailover increments the failover counter when a leader changes.
func (r *Recorder) RecordFailover(namespace, service string) {
	r.failoverCountTotal.WithLabelValues(namespace, service).Inc()
}

// RecordReconciliationDuration records the duration of a reconciliation loop.
func (r *Recorder) RecordReconciliationDuration(namespace, service, result string, durationSeconds float64) {
	r.reconciliationDurationSeconds.WithLabelValues(namespace, service, result).Observe(durationSeconds)
}

// RecordPodsAvailable records the number of Ready pods available.
func (r *Recorder) RecordPodsAvailable(namespace, service string, count int) {
	r.podsAvailable.WithLabelValues(namespace, service).Set(float64(count))
}

// RecordPortResolutionFailure increments the port resolution failure counter.
func (r *Recorder) RecordPortResolutionFailure(namespace, service, portName string) {
	r.portResolutionFailuresTotal.WithLabelValues(namespace, service, portName).Inc()
}

// RecordReconciliationError increments the reconciliation error counter.
func (r *Recorder) RecordReconciliationError(namespace, service, errorType string) {
	r.reconciliationErrorsTotal.WithLabelValues(namespace, service, errorType).Inc()
}

// ResetLeaderDuration resets the leader duration metric (H011.8: no pod label).
// Call this when a pod is no longer the leader.
func (r *Recorder) ResetLeaderDuration(namespace, service string) {
	r.leaderDurationSeconds.WithLabelValues(namespace, service).Set(0)
}

// RecordLeaderServicesTotal records the total number of leader Services managed.
func (r *Recorder) RecordLeaderServicesTotal(namespace string, count int) {
	r.leaderServicesTotal.WithLabelValues(namespace).Set(float64(count))
}

// RecordEndpointSlicesTotal records the total number of EndpointSlices managed.
func (r *Recorder) RecordEndpointSlicesTotal(namespace string, count int) {
	r.endpointSlicesTotal.WithLabelValues(namespace).Set(float64(count))
}

// RecordStickyLeaderHit records when sticky leader was kept (no change).
func (r *Recorder) RecordStickyLeaderHit(namespace, service string) {
	r.stickyLeaderHitsTotal.WithLabelValues(namespace, service).Inc()
}

// RecordStickyLeaderMiss records when sticky leader was not available (new leader selected).
func (r *Recorder) RecordStickyLeaderMiss(namespace, service string) {
	r.stickyLeaderMissesTotal.WithLabelValues(namespace, service).Inc()
}

// RecordLeaderSelectionAttempt records a leader selection attempt.
func (r *Recorder) RecordLeaderSelectionAttempt(namespace, service string) {
	r.leaderSelectionAttemptsTotal.WithLabelValues(namespace, service).Inc()
}

// RecordLeaderPodAge records the age of the current leader pod (H011.8: no pod label).
// Leader identity is exposed via leader Service annotations (zen-lead.io/leader-pod-name, etc.)
func (r *Recorder) RecordLeaderPodAge(namespace, service string, ageSeconds float64) {
	r.leaderPodAgeSeconds.WithLabelValues(namespace, service).Set(ageSeconds)
}

// RecordLeaderServiceWithoutEndpoints records if a leader Service has no endpoints.
func (r *Recorder) RecordLeaderServiceWithoutEndpoints(namespace, service string, hasNoEndpoints bool) {
	value := 0.0
	if hasNoEndpoints {
		value = 1.0
	}
	r.leaderServiceWithoutEndpoints.WithLabelValues(namespace, service).Set(value)
}

// RecordReconciliation records a reconciliation attempt (counter).
func (r *Recorder) RecordReconciliation(namespace, service, result string) {
	r.reconciliationsTotal.WithLabelValues(namespace, service, result).Inc()
}

// Exported getters for testing (access to metric vectors)

// PodsAvailable returns the pods available gauge vector (for testing)
func (r *Recorder) PodsAvailable() *prometheus.GaugeVec {
	return r.podsAvailable
}

// ReconciliationsTotal returns the reconciliations counter vector (for testing)
func (r *Recorder) ReconciliationsTotal() *prometheus.CounterVec {
	return r.reconciliationsTotal
}

// LeaderSelectionAttemptsTotal returns the leader selection attempts counter vector (for testing)
func (r *Recorder) LeaderSelectionAttemptsTotal() *prometheus.CounterVec {
	return r.leaderSelectionAttemptsTotal
}

// FailoverCountTotal returns the failover count counter vector (for testing)
func (r *Recorder) FailoverCountTotal() *prometheus.CounterVec {
	return r.failoverCountTotal
}

// PortResolutionFailuresTotal returns the port resolution failures counter vector (for testing)
func (r *Recorder) PortResolutionFailuresTotal() *prometheus.CounterVec {
	return r.portResolutionFailuresTotal
}
