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
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	leadershipv1alpha1 "github.com/kube-zen/zen-lead/pkg/apis/leadership.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lead/pkg/controller"
	"github.com/kube-zen/zen-lead/pkg/director"
	"github.com/kube-zen/zen-sdk/pkg/leader"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"github.com/kube-zen/zen-sdk/pkg/observability"
)

var (
	scheme   = runtime.NewScheme()
	logger   *sdklog.Logger
	setupLog *sdklog.Logger
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(leadershipv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var leaderElectionID string
	var probeAddr string
	var enableLeaderGroups bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "zen-lead-controller-leader-election",
		"The ID for leader election. Must be unique per controller instance in the same namespace.")
	flag.BoolVar(&enableLeaderGroups, "enable-leader-groups", false,
		"Enable LeaderGroup CRD support (Profile C). Default: false (Profile A only, CRD-free).")

	flag.Parse()

	// Initialize zen-sdk logger (configures controller-runtime logger automatically)
	logger = sdklog.NewLogger("zen-lead")
	setupLog = logger.WithComponent("setup")

	// Initialize OpenTelemetry tracing (optional, uses environment variables)
	ctx := ctrl.SetupSignalHandler()
	shutdownTracing, err := observability.InitWithDefaults(ctx, "zen-lead")
	if err != nil {
		setupLog.Warn("Failed to initialize OpenTelemetry tracing", sdklog.ErrorCode("TRACING_INIT_ERROR"), sdklog.String("error", err.Error()))
		setupLog.Info("Continuing without tracing")
	} else {
		setupLog.Info("OpenTelemetry tracing initialized")
		defer func() {
			if err := shutdownTracing(ctx); err != nil {
				setupLog.Error(err, "Failed to shutdown tracing", sdklog.ErrorCode("TRACING_SHUTDOWN_ERROR"))
			}
		}()
	}

	// Get pod namespace (required for leader election)
	leaderElectionNS, err := leader.RequirePodNamespace()
	if err != nil {
		setupLog.Error(err, "failed to determine pod namespace for leader election", sdklog.ErrorCode("LEADER_ELECTION_ERROR"))
		os.Exit(1)
	}

	// Set REST config QPS/Burst defaults (via zen-sdk helper)
	restConfig := ctrl.GetConfigOrDie()
	leader.ApplyRestConfigDefaults(restConfig)

	// Configure manager options
	mgrOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
	}

	// Apply leader election (mandatory for zen-lead - always enabled, no option to disable)
	leader.ApplyLeaderElection(&mgrOpts, "zen-lead-controller", leaderElectionNS, leaderElectionID, true)

	mgr, err := ctrl.NewManager(restConfig, mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager", sdklog.ErrorCode("MANAGER_INIT_ERROR"))
		os.Exit(1)
	}

	// Setup Service Director controller (traffic routing to leader pods)
	// Non-invasive Service-based approach: watches Services with zen-lead.io/enabled annotation
	// This is Profile A (network-only, CRD-free) - always enabled
	eventRecorder := mgr.GetEventRecorderFor("zen-lead-controller")
	reconciler := director.NewServiceDirectorReconciler(mgr.GetClient(), mgr.GetScheme(), eventRecorder)
	if err = reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", sdklog.Component("ServiceDirector"), sdklog.ErrorCode("CONTROLLER_SETUP_ERROR"))
		os.Exit(1)
	}
	setupLog.Info("Service Director controller enabled (Profile A: network-only)", sdklog.Component("ServiceDirector"))

	// Setup LeaderGroup controller (Profile C: CRD-driven controller HA)
	// This is optional and disabled by default to maintain Day-0 CRD-free contract
	if enableLeaderGroups {
		leadergroupReconciler := &controller.LeaderGroupReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}
		if err = leadergroupReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", sdklog.Component("LeaderGroup"), sdklog.ErrorCode("CONTROLLER_SETUP_ERROR"))
			os.Exit(1)
		}
		setupLog.Info("LeaderGroup controller enabled (Profile C: CRD-driven)", sdklog.Component("LeaderGroup"))
	} else {
		setupLog.Info("LeaderGroup controller disabled (Profile A only, CRD-free)", sdklog.Component("LeaderGroup"))
	}

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check", sdklog.ErrorCode("HEALTH_CHECK_ERROR"))
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check", sdklog.ErrorCode("READY_CHECK_ERROR"))
		os.Exit(1)
	}

	setupLog.Info("starting manager", sdklog.Operation("start"))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager", sdklog.ErrorCode("MANAGER_RUN_ERROR"))
		os.Exit(1)
	}
}
