package snapshotclone

import "github.com/opencurve/curve-operator/pkg/config"

// mdsConfig for a single mds
type snapConfig struct {
	Prefix           string
	ServiceAddr      string
	ServicePort      string
	ServiceDummyPort string
	ServiceProxyPort string
	ClusterEtcdAddr  string
	ClusterMdsAddr   string

	ResourceName         string
	CurrentConfigMapName string
	DaemonID             string
	DataPathMap          *config.DataPathMap
}

func (c *snapConfig) GetPrefix() string {
	return c.Prefix
}

func (c *snapConfig) GetServiceId() string {
	return ""
}

func (c *snapConfig) GetServiceRole() string {
	return ""
}

func (c *snapConfig) GetServiceHost() string {
	return ""
}

func (c *snapConfig) GetServiceHostSequence() string {
	return ""
}

func (c *snapConfig) GetServiceReplicaSequence() string {
	return ""
}

func (c *snapConfig) GetServiceReplicasSequence() string {
	return ""
}

func (c *snapConfig) GetServiceAddr() string {
	return c.ServiceAddr
}

func (c *snapConfig) GetServicePort() string {
	return c.ServicePort
}

func (c *snapConfig) GetServiceClientPort() string {
	return ""
}

func (c *snapConfig) GetServiceDummyPort() string {
	return c.ServiceDummyPort
}

func (c *snapConfig) GetServiceProxyPort() string {
	return c.ServiceProxyPort
}

func (c *snapConfig) GetServiceExternalAddr() string {
	return ""
}

func (c *snapConfig) GetServiceExternalPort() string {
	return ""
}

func (c *snapConfig) GetLogDir() string {
	return ""
}

func (c *snapConfig) GetDataDir() string {
	return ""
}

// cluster
func (c *snapConfig) GetClusterEtcdHttpAddr() string {
	return ""
}

func (c *snapConfig) GetClusterEtcdAddr() string {
	return c.ClusterEtcdAddr
}

func (c *snapConfig) GetClusterMdsAddr() string {
	return c.ClusterMdsAddr
}

func (c *snapConfig) GetClusterMdsDummyAddr() string {
	return ""
}

func (c *snapConfig) GetClusterMdsDummyPort() string {
	return ""
}

func (c *snapConfig) GetClusterChunkserverAddr() string {
	return ""
}

func (c *snapConfig) GetClusterMetaserverAddr() string {
	return ""
}

func (c *snapConfig) GetClusterSnapshotcloneAddr() string {
	return ""
}

func (c *snapConfig) GetClusterSnapshotcloneProxyAddr() string {
	return ""
}

func (c *snapConfig) GetClusterSnapshotcloneDummyPort() string {
	return ""
}

func (c *snapConfig) GetClusterSnapshotcloneNginxUpstream() string {
	return ""
}
