// formspec-operator is the FormSpec CRD controller — it translates FormSpec CRDs
// (Workspace, Datastore, ResourceClaim, ClusterClass) into native K8s
// resources (Deployment, Service, Secret refs, ConfigMap, HPA) and reports
// cluster health to Cluster Control.
//
// See docs/runtimes/03-formspec-operator.md and
// docs/architecture/06-k8s-operator.md. CRD and RBAC manifests live in
// deploy/operator/.
//
// Usage:
//
//	formspec-operator \
//	  --control-cluster-url https://control-cluster.jkt-premium-01.svc \
//	  --leader-elect \
//	  --metrics-bind-address :8443 \
//	  --health-probe-bind-address :8081 \
//	  --workspace-concurrency 10 \
//	  --namespace ""            # empty = watch all namespaces
package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	formspecv1alpha1 "github.com/primadi/formspec/internal/operator/api/v1alpha1"
	formspeccontroller "github.com/primadi/formspec/internal/operator/controller"
	"github.com/primadi/formspec/internal/operator/report"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(formspecv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		controlClusterURL    string
		leaderElect          bool
		metricsAddr          string
		probeAddr            string
		workspaceConcurrency int
		namespace            string
		resourceImage        string
		insecureSkipVerify   bool
	)
	flag.StringVar(&controlClusterURL, "control-cluster-url", "", "Cluster Control URL for health/status reporting (empty: reporting disabled)")
	flag.BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for HA operator replicas")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8443", "Prometheus metrics listen address (\"0\" to disable)")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Liveness/readiness probe listen address")
	flag.IntVar(&workspaceConcurrency, "workspace-concurrency", 10, "Max parallel reconciles per controller")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch (empty: all namespaces)")
	flag.StringVar(&resourceImage, "resource-image", "formahub/formspec-resource:latest", "Generic formspec-resource image for workspace Deployments (pin a version in production)")
	flag.BoolVar(&insecureSkipVerify, "insecure-skip-signature-verify", false, "DEV ONLY: accept ResourceClaims without ed25519 verification")

	opts := zap.Options{Development: false}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgrOptions := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElect,
		LeaderElectionID:       "formspec-operator.formspec.dev",
	}
	if namespace != "" {
		mgrOptions.Cache = cache.Options{
			DefaultNamespaces: map[string]cache.Config{namespace: {}},
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOptions)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	reporter := report.New(controlClusterURL, mgr.GetClient())

	concurrency := controller.Options{MaxConcurrentReconciles: workspaceConcurrency}

	wsReconciler := &formspeccontroller.WorkspaceReconciler{
		Client:            mgr.GetClient(),
		ResourceImage:     resourceImage,
		ControlClusterURL: controlClusterURL,
		Reporter:          reporter,
	}
	if err := wsReconciler.SetupWithManagerWithOptions(mgr, concurrency); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "Workspace")
		os.Exit(1)
	}

	if err := (&formspeccontroller.DatastoreReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "Datastore")
		os.Exit(1)
	}

	if err := (&formspeccontroller.ResourceClaimReconciler{
		Client:                      mgr.GetClient(),
		InsecureSkipSignatureVerify: insecureSkipVerify,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "ResourceClaim")
		os.Exit(1)
	}

	// Node-health loop runs only on the elected leader, alongside the
	// reconcilers (docs/runtimes/03-formspec-operator.md §3.2).
	if err := mgr.Add(manager.RunnableFunc(reporter.RunNodeHealthLoop)); err != nil {
		setupLog.Error(err, "unable to add node-health reporter")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if insecureSkipVerify {
		setupLog.Info("WARNING: ResourceClaim signature verification is DISABLED (--insecure-skip-signature-verify)")
	}
	setupLog.Info("starting formspec-operator",
		"resourceImage", resourceImage, "controlClusterURL", controlClusterURL,
		"leaderElect", leaderElect, "namespace", namespace)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
