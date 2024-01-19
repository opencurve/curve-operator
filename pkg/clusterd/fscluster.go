package clusterd

import (
	"github.com/go-logr/logr"
	curvev1 "github.com/opencurve/curve-operator/api/v1"
)

var _ Clusterer = &FsClusterManager{}

type FsClusterManager struct {
	Context Context
	Cluster *curvev1.Curvefs
	Logger  logr.Logger

	UUID      string
	Kind      string
	OwnerInfo *OwnerInfo
}

func (c *FsClusterManager) GetContext() Context                           { return c.Context }
func (c *FsClusterManager) GetName() string                               { return c.Cluster.Name }
func (c *FsClusterManager) GetNameSpace() string                          { return c.Cluster.Namespace }
func (c *FsClusterManager) GetUUID() string                               { return c.UUID }
func (c *FsClusterManager) GetKind() string                               { return c.Kind }
func (c *FsClusterManager) GetOwnerInfo() *OwnerInfo                      { return c.OwnerInfo }
func (c *FsClusterManager) GetNodes() []string                            { return c.Cluster.Spec.Nodes }
func (c *FsClusterManager) GetDataDir() string                            { return c.Cluster.Spec.DataDir }
func (c *FsClusterManager) GetLogDir() string                             { return c.Cluster.Spec.LogDir }
func (c *FsClusterManager) GetContainerImage() string                     { return c.Cluster.Spec.CurveVersion.Image }
func (c *FsClusterManager) GetCopysets() int                              { return *c.Cluster.Spec.Copysets }
func (c *FsClusterManager) GetEtcdSpec() *curvev1.EtcdSpec                { return c.Cluster.Spec.Etcd }
func (c *FsClusterManager) GetMdsSpec() *curvev1.MdsSpec                  { return c.Cluster.Spec.Mds }
func (c *FsClusterManager) GetChunkserverSpec() *curvev1.StorageScopeSpec { return nil }
func (c *FsClusterManager) GetMetaserverSpec() *curvev1.MetaServerSpec {
	return c.Cluster.Spec.MetaServer
}
func (c *FsClusterManager) GetSnapShotSpec() *curvev1.SnapShotCloneSpec { return nil }
func (c *FsClusterManager) GetRoleInstances(role string) int {
	switch role {
	case ROLE_ETCD, ROLE_MDS:
		if len(c.GetNodes()) == 1 { // stand alone
			return 3
		}
	case ROLE_METASERVER:
		return c.Cluster.Spec.MetaServer.Instances
	}

	return 1
}

func (c *FsClusterManager) GetRolePort(role string) int {
	switch role {
	case ROLE_ETCD:
		return *c.Cluster.Spec.Etcd.PeerPort
	case ROLE_MDS:
		return *c.Cluster.Spec.Mds.Port
	case ROLE_METASERVER:
		return *c.Cluster.Spec.MetaServer.Port
	default:
		return 0
	}
}

func (c *FsClusterManager) GetRoleClientPort(role string) int {
	switch role {
	case ROLE_ETCD:
		return *c.Cluster.Spec.Etcd.ClientPort
	default:
		return 0
	}
}

func (c *FsClusterManager) GetRoleDummyPort(role string) int {
	switch role {
	case ROLE_MDS:
		return *c.Cluster.Spec.Mds.DummyPort
	default:
		return 0
	}
}

func (c *FsClusterManager) GetRoleProxyPort(role string) int {
	return 0
}

func (c *FsClusterManager) GetRoleExternalPort(role string) int {
	switch role {
	case ROLE_METASERVER:
		return *c.Cluster.Spec.MetaServer.ExternalPort
	default:
		return 0
	}
}

func (c *FsClusterManager) GetRoleConfigs(role string) map[string]string {
	switch role {
	case ROLE_ETCD:
		return c.Cluster.Spec.Etcd.Config
	case ROLE_MDS:
		return c.Cluster.Spec.Mds.Config
	case ROLE_METASERVER:
		return c.Cluster.Spec.MetaServer.Config
	default:
		return nil
	}
}
