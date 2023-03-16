package chunkserver

import (
	"github.com/coreos/pkg/capnslog"
	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	AppName       = "curve-chunkserver"
	configMapName = "curve-chunkserver-config"

	// ContainerPath is the mount path of data and log
	ContainerDataDir = "/curvebs/chunkserver/data"
	ContainerLogDir  = "/curvebs/chunkserver/logs"

	DefaultMdsCount = 3
)

type Cluster struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	spec           curvev1.CurveClusterSpec
}

var log = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "chunkserver")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec) *Cluster {
	return &Cluster{
		context:        context,
		namespacedName: namespacedName,
		spec:           spec,
	}
}

// Start begins the process of running a cluster of curve mds.
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	log.Infof("start running chunkserver in namespace %q", c.namespacedName.Namespace)

	if !c.spec.Storage.UseSelectedNodes && (len(c.spec.Storage.Nodes) == 0 || len(c.spec.Storage.Devices) == 0) {
		log.Error("useSelectedNodes is set to false but no node or device specified")
		return errors.New("useSelectedNodes is set to false but no node specified")
	}

	if c.spec.Storage.UseSelectedNodes && len(c.spec.Storage.SelectedNodes) == 0 {
		log.Error("useSelectedNodes is set to false but selectedNodes not be specified")
		return errors.New("useSelectedNodes is set to false but selectedNodes not be specified")
	}

	log.Info("starting provisioning the chunkfilepool")

	err := c.startProvisioningOverNodes()
	if err != nil {
		log.Error("failed to provision chunkfilepool")
		return errors.Wrap(err, "failed to provision chunkfilepool")
	}
	return nil
}
