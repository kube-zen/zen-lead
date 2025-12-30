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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// Strategy represents the HA enforcement strategy for a resource type
type Strategy string

const (
	// StrategyTrafficDirector routes network traffic to leader pod via Service selector
	// Used for: Deployments, StatefulSets, Services
	StrategyTrafficDirector Strategy = "TrafficDirector"

	// StrategyStateGuard ensures only leader pod executes logic/reconciles
	// Used for: Jobs, CronJobs
	StrategyStateGuard Strategy = "StateGuard"

	// StrategyNone indicates no HA strategy needed
	// Used for: Single replica deployments, or resources that don't need HA
	StrategyNone Strategy = "None"
)

// DetectStrategy detects the appropriate HA strategy based on resource type
// This enables "Zero-Opinionated HA" - zen-lead automatically chooses the right strategy
func DetectStrategy(resourceType string) Strategy {
	switch resourceType {
	case "Deployment", "deployment", "apps/v1.Deployment":
		return StrategyTrafficDirector
	case "StatefulSet", "statefulset", "apps/v1.StatefulSet":
		return StrategyTrafficDirector
	case "Service", "service", "v1.Service":
		return StrategyTrafficDirector
	case "Job", "job", "batch/v1.Job":
		return StrategyStateGuard
	case "CronJob", "cronjob", "batch/v1.CronJob":
		return StrategyStateGuard
	default:
		return StrategyNone
	}
}

// DetectStrategyFromObject detects strategy from a Kubernetes object
func DetectStrategyFromObject(obj interface{}) Strategy {
	switch obj.(type) {
	case *appsv1.Deployment:
		return StrategyTrafficDirector
	case *appsv1.StatefulSet:
		return StrategyTrafficDirector
	case *corev1.Service:
		return StrategyTrafficDirector
	case *batchv1.Job:
		return StrategyStateGuard
	case *batchv1.CronJob:
		return StrategyStateGuard
	default:
		return StrategyNone
	}
}

// ShouldEnableHA determines if HA should be enabled based on replica count
// Smart Default: If replicas > 1, assume HA is desired
func ShouldEnableHA(replicas int32) bool {
	return replicas > 1
}

