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

package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kube-zen/zen-lead/pkg/director"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var leaderElectionID string
	var probeAddr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "zen-lead-controller-leader-election",
		"The ID for leader election. Must be unique per controller instance in the same namespace.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Leader election namespace from Downward API (POD_NAMESPACE)
	leaderElectionNS := os.Getenv("POD_NAMESPACE")
	if leaderElectionNS == "" {
		setupLog.Error(nil, "POD_NAMESPACE environment variable must be set for leader election")
		os.Exit(1)
	}

	// Set REST config QPS/Burst to avoid self-throttling under churn
	restConfig := ctrl.GetConfigOrDie()
	restConfig.QPS = 50    // Default is 20, increase for faster reconciliation
	restConfig.Burst = 100 // Default is 30, increase for burst handling

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          true, // Always enabled for HA safety
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: leaderElectionNS,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup Service Director controller (traffic routing to leader pods)
	// Non-invasive Service-based approach: watches Services with zen-lead.io/enabled annotation
	eventRecorder := mgr.GetEventRecorderFor("zen-lead-controller")
	reconciler := director.NewServiceDirectorReconciler(mgr.GetClient(), mgr.GetScheme(), eventRecorder)
	if err = reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceDirector")
		os.Exit(1)
	}
	setupLog.Info("Service Director controller enabled")

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
