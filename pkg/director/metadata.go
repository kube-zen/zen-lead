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

// GitOpsTrackingLabels contains common GitOps tool labels that should NOT be copied to generated resources
// These labels would cause ownership/prune conflicts when resources are managed by controllers
var gitOpsTrackingLabels = map[string]struct{}{
	"app.kubernetes.io/instance":            {},
	"app.kubernetes.io/managed-by":          {}, // Controllers set their own value
	"app.kubernetes.io/part-of":             {},
	"app.kubernetes.io/version":             {},
	"argocd.argoproj.io/instance":           {},
	"fluxcd.io/part-of":                     {},
	"kustomize.toolkit.fluxcd.io/name":      {},
	"kustomize.toolkit.fluxcd.io/namespace": {},
	"kustomize.toolkit.fluxcd.io/revision": {},
}

// GitOpsTrackingAnnotations contains common GitOps tool annotations that should NOT be copied to generated resources
// These annotations would cause ownership/prune conflicts when resources are managed by controllers
var gitOpsTrackingAnnotations = map[string]struct{}{
	"argocd.argoproj.io/sync-wave":         {},
	"argocd.argoproj.io/sync-options":      {},
	"fluxcd.io/sync-checksum":              {},
	"kustomize.toolkit.fluxcd.io/checksum": {},
}

// filterGitOpsLabels removes GitOps tracking labels from a label map
// Optimized: O(n) with map lookup instead of O(n*m) with nested loops
// Returns a new map with GitOps tracking labels removed
func filterGitOpsLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return make(map[string]string)
	}
	// Pre-allocate with estimated capacity (most labels will pass through)
	filtered := make(map[string]string, len(labels))
	for k, v := range labels {
		if _, skip := gitOpsTrackingLabels[k]; !skip {
			filtered[k] = v
		}
	}
	return filtered
}

// filterGitOpsAnnotations removes GitOps tracking annotations from an annotation map
// Optimized: O(n) with map lookup instead of O(n*m) with nested loops
// Returns a new map with GitOps tracking annotations removed
func filterGitOpsAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		return make(map[string]string)
	}
	// Pre-allocate with estimated capacity (most annotations will pass through)
	filtered := make(map[string]string, len(annotations))
	for k, v := range annotations {
		if _, skip := gitOpsTrackingAnnotations[k]; !skip {
			filtered[k] = v
		}
	}
	return filtered
}

