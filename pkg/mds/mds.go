package mds

import (
	"github.com/coreos/pkg/capnslog"
	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"k8s.io/apimachinery/pkg/types"
)

const (
	AppName       = "curve-mds"
	configMapName = "curve-mds-config"

	// ContainerPath is the mount path of data and log
	ContainerDataDir = "/curvebs/mds/data"
	ContainerLogDir  = "/curvebs/mds/logs"

	DefaultMdsCount = 3
)

type Cluster struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	spec           curvev1.CurveClusterSpec
}

var log = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "mds")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec) *Cluster {
	return &Cluster{
		context:        context,
		namespacedName: namespacedName,
		spec:           spec,
	}
}

// Start begins the process of running a cluster of curve etcds.
func (c *Cluster) Start() error {

	return nil
}
