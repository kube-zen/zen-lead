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
	"os"
	"testing"
)

func TestDetermineIdentity(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		env      map[string]string
		wantErr  bool
	}{
		{
			name:     "pod strategy with POD_NAME",
			strategy: "pod",
			env: map[string]string{
				"POD_NAME": "test-pod",
				"POD_UID":  "test-uid",
			},
			wantErr: false,
		},
		{
			name:     "pod strategy without POD_NAME",
			strategy: "pod",
			env:      map[string]string{},
			wantErr:  false, // Falls back to hostname
		},
		{
			name:     "custom strategy with identity",
			strategy: "custom",
			env: map[string]string{
				"ZEN_LEAD_IDENTITY": "custom-identity",
			},
			wantErr: false,
		},
		{
			name:     "custom strategy without identity",
			strategy: "custom",
			env:      map[string]string{},
			wantErr:  true,
		},
		{
			name:     "unknown strategy",
			strategy: "unknown",
			env:      map[string]string{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			defer func() {
				// Clean up
				for k := range tt.env {
					os.Unsetenv(k)
				}
			}()

			identity, err := determineIdentity(tt.strategy)
			if (err != nil) != tt.wantErr {
				t.Errorf("determineIdentity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && identity == "" {
				t.Error("determineIdentity() returned empty identity")
			}
		})
	}
}

func TestElection_IsLeader(t *testing.T) {
	// Note: This is a simple test since Election requires a real Kubernetes client
	// Full integration tests would require a test environment
	e := &Election{
		isLeader: false,
	}

	if e.IsLeader() {
		t.Error("Expected IsLeader() to return false")
	}

	e.mu.Lock()
	e.isLeader = true
	e.mu.Unlock()

	if !e.IsLeader() {
		t.Error("Expected IsLeader() to return true")
	}
}

func TestElection_Identity(t *testing.T) {
	e := &Election{
		identity: "test-identity",
	}

	if e.Identity() != "test-identity" {
		t.Errorf("Expected identity 'test-identity', got '%s'", e.Identity())
	}
}

