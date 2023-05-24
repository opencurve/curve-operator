package daemon

import (
	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"k8s.io/apimachinery/pkg/types"
)

type Cluster struct {
	Kind               string
	Context            clusterd.Context
	Namespace          string
	NamespacedName     types.NamespacedName
	ObservedGeneration int64
	OwnerInfo          *k8sutil.OwnerInfo
	IsUpgrade          bool

	Nodes         []string
	CurveVersion  curvev1.CurveVersionSpec
	Etcd          curvev1.EtcdSpec
	Mds           curvev1.MdsSpec
	SnapShotClone curvev1.SnapShotCloneSpec
	Chunkserver   curvev1.StorageScopeSpec
	Metaserver    curvev1.MetaServerSpec
	Monitor       curvev1.MonitorSpec

	HostDataDir     string
	DataDirHostPath string
	LogDirHostPath  string
	ConfDirHostPath string
}
