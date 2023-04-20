/*


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

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/controllers"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

type OperatorOptions struct {
	// MetricsAddr
	MetricsAddr string
	// EnableLeaderElection
	EnableLeaderElection bool
}

func NewOperatorOptions() (*OperatorOptions, error) {
	return &OperatorOptions{
		MetricsAddr:          ":8080",
		EnableLeaderElection: false,
	}, nil
}

func main() {
	// opts, err := NewOperatorOptions()
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	// 	os.Exit(1)
	// }
	// cmd := cobra.Command{
	// 	Use:  "curve-operator",
	// 	Long: `curve-operator is daemon that operator curve cluster on Kubernetes`,
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		setupLog.Error(opts.Run(), "failed to run curve-operator")
	// 		os.Exit(1)
	// 	},
	// }
	// opts.AddFlags(cmd.Flags())

	// if err := cmd.Execute(); err != nil {
	// 	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	// 	os.Exit(1)
	// }

	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// get config file and create clientSet that pass to context

	config := ctrl.GetConfigOrDie()

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "create clientset failed")
		os.Exit(1)
	}

	// Create context
	context := clusterd.Context{
		KubeConfig: config,
		Clientset:  clientSet,
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
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
		setupLog.Error(err, "unable to create controller", "controller", "Curvefs")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// AddFlags adds flags to fs and binds them to options.
func (opts *OperatorOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.MetricsAddr, "metricsAddr", opts.MetricsAddr, "The address on which to advertise.")
	fs.BoolVar(&opts.EnableLeaderElection, "enableLeaderElection", opts.EnableLeaderElection, "Enables leader election for kubediag master.")
}
