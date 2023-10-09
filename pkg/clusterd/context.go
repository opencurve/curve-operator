package clusterd

import (
	"fmt"
	"github.com/opencurve/curve-operator/pkg/util/exec"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Context struct {
	// The kubernetes config used for this context
	KubeConfig *rest.Config

	// Clientset is a connection to the core kubernetes API
	Clientset kubernetes.Interface

	// Represents the Client provided by the controller-runtime package to interact with Kubernetes objects
	Client client.Client

	Executor *exec.CommandExecutor
}

func NewContext() *Context {
	ctx := &Context{
		Executor: &exec.CommandExecutor{},
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	ctx.KubeConfig = config
	ctx.Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return ctx
}
