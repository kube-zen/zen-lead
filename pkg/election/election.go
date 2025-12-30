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

package election

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

// Election manages leader election for a pool of candidates
type Election struct {
	client    kubernetes.Interface
	namespace string
	name      string
	identity  string
	config    *Config
	onStarted func(context.Context)
	onStopped func()
	isLeader  bool
	mu        sync.RWMutex
}

// Config holds leader election configuration
type Config struct {
	LeaseDurationSeconds int32
	RenewDeadlineSeconds int32
	RetryPeriodSeconds   int32
	IdentityStrategy      string
}

// NewElection creates a new leader election manager
func NewElection(
	client kubernetes.Interface,
	namespace string,
	policyName string,
	config *Config,
) (*Election, error) {
	// Determine identity based on strategy
	identity, err := determineIdentity(config.IdentityStrategy)
	if err != nil {
		return nil, fmt.Errorf("failed to determine identity: %w", err)
	}

	return &Election{
		client:    client,
		namespace: namespace,
		name:      policyName,
		identity:  identity,
		config:    config,
		isLeader:  false,
	}, nil
}

// determineIdentity determines the identity based on the strategy
func determineIdentity(strategy string) (string, error) {
	switch strategy {
	case "pod":
		// Use Pod Name from environment (set by Kubernetes)
		podName := os.Getenv("POD_NAME")
		if podName == "" {
			// Fallback to hostname
			hostname, err := os.Hostname()
			if err != nil {
				return "", fmt.Errorf("failed to get hostname: %w", err)
			}
			podName = hostname
		}

		// Add unique suffix to avoid conflicts
		podUID := os.Getenv("POD_UID")
		if podUID != "" {
			return fmt.Sprintf("%s-%s", podName, podUID), nil
		}

		// Fallback: use timestamp
		return fmt.Sprintf("%s-%d", podName, time.Now().Unix()), nil

	case "custom":
		// Use custom identity from annotation
		identity := os.Getenv("ZEN_LEAD_IDENTITY")
		if identity == "" {
			return "", fmt.Errorf("ZEN_LEAD_IDENTITY must be set for custom identity strategy")
		}
		return identity, nil

	default:
		return "", fmt.Errorf("unknown identity strategy: %s", strategy)
	}
}

// SetCallbacks sets the callbacks for leader election
func (e *Election) SetCallbacks(onStarted func(context.Context), onStopped func()) {
	e.onStarted = onStarted
	e.onStopped = onStopped
}

// Run starts the leader election process (blocks until context is canceled)
func (e *Election) Run(ctx context.Context) error {
	// Create lease lock
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      e.name,
			Namespace: e.namespace,
		},
		Client: e.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: e.identity,
		},
	}

	// Set defaults if not provided
	leaseDuration := time.Duration(e.config.LeaseDurationSeconds) * time.Second
	if leaseDuration == 0 {
		leaseDuration = 15 * time.Second
	}

	renewDeadline := time.Duration(e.config.RenewDeadlineSeconds) * time.Second
	if renewDeadline == 0 {
		renewDeadline = 10 * time.Second
	}

	retryPeriod := time.Duration(e.config.RetryPeriodSeconds) * time.Second
	if retryPeriod == 0 {
		retryPeriod = 2 * time.Second
	}

	// Leader election configuration
	lec := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   leaseDuration,
		RenewDeadline:   renewDeadline,
		RetryPeriod:     retryPeriod,
		ReleaseOnCancel: true,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				e.mu.Lock()
				e.isLeader = true
				e.mu.Unlock()

				klog.InfoS("Became leader",
					"identity", e.identity,
					"namespace", e.namespace,
					"policy", e.name,
				)

				if e.onStarted != nil {
					e.onStarted(ctx)
				}
			},
			OnStoppedLeading: func() {
				e.mu.Lock()
				e.isLeader = false
				e.mu.Unlock()

				klog.InfoS("Lost leadership",
					"identity", e.identity,
				)

				if e.onStopped != nil {
					e.onStopped()
				}
			},
			OnNewLeader: func(identity string) {
				klog.InfoS("New leader elected",
					"leader", identity,
					"self", e.identity,
				)
			},
		},
	}

	// Run leader election (blocks until context is canceled)
	leaderelection.RunOrDie(ctx, lec)

	return nil
}

// IsLeader returns whether this instance is the leader
func (e *Election) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.isLeader
}

// Identity returns the leader election identity
func (e *Election) Identity() string {
	return e.identity
}

