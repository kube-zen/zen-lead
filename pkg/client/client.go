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

// Package client provides an optional client SDK for checking leader status.
// NOTE: This client uses Leases and pool names for leader election.
// The primary zen-lead functionality uses Service-annotation opt-in and
// network-level routing, which does not require this client SDK.
package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultCacheTTL is the default cache TTL for leader status
	DefaultCacheTTL = 2 * time.Second
)

// Client provides a simple API for checking leader status
// This is the "Simple Query" API that tools use to ask "Am I the leader?"
type Client struct {
	k8sClient client.Client
	cache     map[string]cacheEntry
	cacheMu   sync.RWMutex
	podName   string
	podUID    string
}

type cacheEntry struct {
	isLeader bool
	expires  time.Time
}

// NewClient creates a new zen-lead client
// It reads POD_NAME and POD_UID from environment variables or pod metadata
func NewClient(k8sClient client.Client) (*Client, error) {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName = os.Getenv("HOSTNAME")
	}

	podUID := os.Getenv("POD_UID")

	return &Client{
		k8sClient: k8sClient,
		cache:     make(map[string]cacheEntry),
		podName:   podName,
		podUID:    podUID,
	}, nil
}

// IsLeader checks if the current pod is the leader for the given pool
// This is the single, simple API that tools use to check leadership status
//
// Returns:
//   - true if this pod is the leader
//   - false if this pod is not the leader
//   - error if there's an issue checking (e.g., zen-lead not installed)
//
// Error handling:
//   - If zen-lead is not installed (Lease doesn't exist), returns error
//   - If pod name cannot be determined, returns error
//   - If namespace cannot be determined, returns error
//   - All errors should be handled explicitly by the caller
func (c *Client) IsLeader(ctx context.Context, poolName string) (bool, error) {
	// Check cache first
	c.cacheMu.RLock()
	if entry, ok := c.cache[poolName]; ok && time.Now().Before(entry.expires) {
		isLeader := entry.isLeader
		c.cacheMu.RUnlock()
		return isLeader, nil
	}
	c.cacheMu.RUnlock()

	// If pod name is not set, return error (don't assume leader in production)
	if c.podName == "" {
		return false, fmt.Errorf("pod name not set (POD_NAME or HOSTNAME environment variable required)")
	}

	// Get namespace
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		// Try to read from service account namespace file
		const maxNamespaceFileSize = 256 // Namespace names are typically < 100 bytes
		data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err == nil {
			// Validate file size to prevent resource exhaustion
			if len(data) > maxNamespaceFileSize {
				return false, fmt.Errorf("namespace file too large: %d bytes (max: %d)", len(data), maxNamespaceFileSize)
			}
			namespace = strings.TrimSpace(string(data))
			// Validate namespace format (RFC 1123 label)
			if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
				return false, fmt.Errorf("invalid namespace format: %v", errs)
			}
		} else {
			// Can't determine namespace - return error instead of assuming leader
			// This prevents split-brain scenarios in production
			return false, fmt.Errorf("cannot determine namespace: %w", err)
		}
	} else {
		// Validate namespace from environment variable
		namespace = strings.TrimSpace(namespace)
		if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
			return false, fmt.Errorf("invalid namespace format from POD_NAMESPACE: %v", errs)
		}
	}

	// Get the Lease resource for this pool
	lease := &coordinationv1.Lease{}
	leaseKey := types.NamespacedName{
		Name:      poolName,
		Namespace: namespace,
	}

	if err := c.k8sClient.Get(ctx, leaseKey, lease); err != nil {
		// Lease doesn't exist - zen-lead might not be installed
		// Return error instead of assuming leader to prevent split-brain scenarios
		return false, fmt.Errorf("lease not found for pool %s in namespace %s (zen-lead may not be installed): %w", poolName, namespace, err)
	}

	// Check if this pod is the leader
	isLeader := false
	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity != "" {
		leaderIdentity := *lease.Spec.HolderIdentity

		// Match identity - check if identity matches pod name or pod-name-uid format
		if c.podName == leaderIdentity ||
			fmt.Sprintf("%s-%s", c.podName, c.podUID) == leaderIdentity {
			isLeader = true
		}
	}

	// Update cache
	c.cacheMu.Lock()
	c.cache[poolName] = cacheEntry{
		isLeader: isLeader,
		expires:  time.Now().Add(DefaultCacheTTL),
	}
	c.cacheMu.Unlock()

	return isLeader, nil
}

// IsLeaderWithNamespace checks if the current pod is the leader for the given pool in a specific namespace
// This variant allows specifying the namespace explicitly
func (c *Client) IsLeaderWithNamespace(ctx context.Context, poolName, namespace string) (bool, error) {
	// Validate namespace format
	namespace = strings.TrimSpace(namespace)
	if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
		return false, fmt.Errorf("invalid namespace format: %v", errs)
	}
	
	// Validate poolName format (should be a valid DNS subdomain)
	if errs := validation.IsDNS1123Subdomain(poolName); len(errs) > 0 {
		return false, fmt.Errorf("invalid pool name format: %v", errs)
	}
	
	// If pod name is not set, return error (don't assume leader in production)
	if c.podName == "" {
		return false, fmt.Errorf("pod name not set (POD_NAME or HOSTNAME environment variable required)")
	}
	
	// Check cache first (using namespace-qualified key)
	cacheKey := fmt.Sprintf("%s/%s", namespace, poolName)
	c.cacheMu.RLock()
	if entry, ok := c.cache[cacheKey]; ok && time.Now().Before(entry.expires) {
		isLeader := entry.isLeader
		c.cacheMu.RUnlock()
		return isLeader, nil
	}
	c.cacheMu.RUnlock()

	// Get the Lease resource for this pool
	lease := &coordinationv1.Lease{}
	leaseKey := types.NamespacedName{
		Name:      poolName,
		Namespace: namespace,
	}

	if err := c.k8sClient.Get(ctx, leaseKey, lease); err != nil {
		// Lease doesn't exist - zen-lead might not be installed
		// Return error instead of assuming leader to prevent split-brain scenarios
		return false, fmt.Errorf("lease not found for pool %s in namespace %s (zen-lead may not be installed): %w", poolName, namespace, err)
	}

	// Check if this pod is the leader
	isLeader := false
	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity != "" {
		leaderIdentity := *lease.Spec.HolderIdentity

		// Match identity - check if identity matches pod name or pod-name-uid format
		if c.podName == leaderIdentity ||
			fmt.Sprintf("%s-%s", c.podName, c.podUID) == leaderIdentity {
			isLeader = true
		}
	}

	// Update cache
	c.cacheMu.Lock()
	c.cache[cacheKey] = cacheEntry{
		isLeader: isLeader,
		expires:  time.Now().Add(DefaultCacheTTL),
	}
	c.cacheMu.Unlock()

	return isLeader, nil
}

// ClearCache clears the leader status cache
// Useful for testing or when you want to force a fresh check
func (c *Client) ClearCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache = make(map[string]cacheEntry)
}
