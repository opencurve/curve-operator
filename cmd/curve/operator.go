package curve

import (
	"fmt"
	operatorv1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/controllers"
	"github.com/opencurve/curve-operator/pkg/discover"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	OperatorCmd = &cobra.Command{
		Use: "operator",
		// TODO: Rewrite this long message.
		Long: `The Curve-Operator is a daemon to deploy Curve and auto it on kubernetes. 
		It support for Curve storage to natively integrate with cloud-native environments.`,
	}
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme

	options, err := NewCurveOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	options.AddFlags(OperatorCmd.Flags())
	OperatorCmd.Run = func(cmd *cobra.Command, args []string) {
		setupLog.Error(options.Run(), "failed to run curve-operator")
		os.Exit(1)
	}
}

type CurveOptions struct {
	MetricsAddr          string
	EnableLeaderElection bool
}

// NewCurveOptions creates a new CurveOptions with a default config
func NewCurveOptions() (*CurveOptions, error) {
	return &CurveOptions{
		MetricsAddr:          ":8080",
		EnableLeaderElection: false,
	}, nil
}

func (opts *CurveOptions) Run() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	config := ctrl.GetConfigOrDie()
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "create clientset failed")
		os.Exit(1)
	}

	// Create clusterd context
	context := clusterd.Context{
		KubeConfig: config,
		Clientset:  clientSet,
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: opts.MetricsAddr,
		Port:               9443,
		LeaderElection:     opts.EnableLeaderElection,
		LeaderElectionID:   "aa88fc6c.curve.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (controllers.NewCurveClusterReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("CurveCluster"),
		mgr.GetScheme(),
		context,
	)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CurveCluster")
		os.Exit(1)
	}
	if err = (controllers.NewCurvefsReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Curvefs"),
		mgr.GetScheme(),
		context,
	)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CurvefsCluster")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// reconcile discover daemonSet
	err = discover.ReconcileDiscoveryDaemon()
	if err != nil {
		setupLog.Error(err, "problem discover")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	return nil
}

// AddFlags adds flags to fs and binds them to options.
func (opts *CurveOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.MetricsAddr, "metrics-port", opts.MetricsAddr, "The address on which to advertise.")
	fs.BoolVar(&opts.EnableLeaderElection, "enable-leader-election", opts.EnableLeaderElection, "Enables leader election for curve-operator master.")
}
