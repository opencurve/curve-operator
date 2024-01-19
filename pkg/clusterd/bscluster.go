package clusterd

import (
	"github.com/go-logr/logr"
	curvev1 "github.com/opencurve/curve-operator/api/v1"
)

var _ Clusterer = &BsClusterManager{}

type BsClusterManager struct {
	Context Context
	Cluster *curvev1.CurveCluster
	Logger  logr.Logger

	UUID      string
	Kind      string
	OwnerInfo *OwnerInfo
}

func (c *BsClusterManager) GetContext() Context            { return c.Context }
func (c *BsClusterManager) GetName() string                { return c.Cluster.Name }
func (c *BsClusterManager) GetNameSpace() string           { return c.Cluster.Namespace }
func (c *BsClusterManager) GetUUID() string                { return c.UUID }
func (c *BsClusterManager) GetKind() string                { return c.Kind }
func (c *BsClusterManager) GetOwnerInfo() *OwnerInfo       { return c.OwnerInfo }
func (c *BsClusterManager) GetNodes() []string             { return c.Cluster.Spec.Nodes }
func (c *BsClusterManager) GetDataDir() string             { return c.Cluster.Spec.DataDir }
func (c *BsClusterManager) GetLogDir() string              { return c.Cluster.Spec.LogDir }
func (c *BsClusterManager) GetContainerImage() string      { return c.Cluster.Spec.CurveVersion.Image }
func (c *BsClusterManager) GetCopysets() int               { return *c.Cluster.Spec.Copysets }
func (c *BsClusterManager) GetEtcdSpec() *curvev1.EtcdSpec { return c.Cluster.Spec.Etcd }
func (c *BsClusterManager) GetMdsSpec() *curvev1.MdsSpec   { return c.Cluster.Spec.Mds }
func (c *BsClusterManager) GetChunkserverSpec() *curvev1.StorageScopeSpec {
	return c.Cluster.Spec.Chunkserver
}
func (c *BsClusterManager) GetMetaserverSpec() *curvev1.MetaServerSpec { return nil }
func (c *BsClusterManager) GetSnapShotSpec() *curvev1.SnapShotCloneSpec {
	return c.Cluster.Spec.SnapShotClone
}
func (c *BsClusterManager) GetRoleInstances(role string) int {
	switch role {
	case ROLE_ETCD, ROLE_MDS:
		if len(c.GetNodes()) == 1 { // stand alone
			return 3
		}
		return 1
	case ROLE_CHUNKSERVER:
		return c.Cluster.Spec.Chunkserver.Instances
	}
	return 0
}

func (c *BsClusterManager) GetRolePort(role string) int {
	switch role {
	case ROLE_ETCD:
		return *c.Cluster.Spec.Etcd.PeerPort
	case ROLE_MDS:
		return *c.Cluster.Spec.Mds.Port
	case ROLE_CHUNKSERVER:
		return *c.Cluster.Spec.Chunkserver.Port
	default:
		return 0
	}
}

func (c *BsClusterManager) GetRoleClientPort(role string) int {
	switch role {
	case ROLE_ETCD:
		return *c.Cluster.Spec.Etcd.ClientPort
	default:
		return 0
	}
}

func (c *BsClusterManager) GetRoleDummyPort(role string) int {
	switch role {
	case ROLE_MDS:
		return *c.Cluster.Spec.Mds.DummyPort
	case ROLE_SNAPSHOTCLONE:
		return *c.Cluster.Spec.SnapShotClone.DummyPort
	default:
		return 0
	}
}

func (c *BsClusterManager) GetRoleProxyPort(role string) int {
	switch role {
	case ROLE_SNAPSHOTCLONE:
		return *c.Cluster.Spec.SnapShotClone.ProxyPort
	}
	return 0
}

func (c *BsClusterManager) GetRoleExternalPort(role string) int {
	return 0
}

func (c *BsClusterManager) GetRoleConfigs(role string) map[string]string {
	switch role {
	case ROLE_ETCD:
		return c.Cluster.Spec.Etcd.Config
	case ROLE_MDS:
		return c.Cluster.Spec.Mds.Config
	case ROLE_METASERVER:
		return c.Cluster.Spec.Chunkserver.Config
	case ROLE_SNAPSHOTCLONE:
		return c.Cluster.Spec.SnapShotClone.Config
	default:
		return nil
	}
}
