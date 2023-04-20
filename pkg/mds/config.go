package mds

import "github.com/opencurve/curve-operator/pkg/config"

// mdsConfig for a single mds
type mdsConfig struct {
	Prefix                        string
	ServiceAddr                   string
	ServicePort                   string
	ServiceDummyPort              string
	ClusterEtcdAddr               string
	ClusterSnapshotcloneProxyAddr string

	ResourceName         string
	CurrentConfigMapName string
	DaemonID             string
	DataPathMap          *config.DataPathMap
	ConfigMapMountPath   string
}

// mdsConfig implement ConfigInterface
func (c *mdsConfig) GetPrefix() string {
	return c.Prefix
}

func (c *mdsConfig) GetServiceId() string {
	return ""
}

func (c *mdsConfig) GetServiceRole() string {
	return "mds"
}

func (c *mdsConfig) GetServiceHost() string {
	return ""
}

func (c *mdsConfig) GetServiceHostSequence() string {
	return ""
}

func (c *mdsConfig) GetServiceReplicaSequence() string {
	return ""
}

func (c *mdsConfig) GetServiceReplicasSequence() string {
	return ""
}

func (c *mdsConfig) GetServiceAddr() string {
	return c.ServiceAddr
}

func (c *mdsConfig) GetServicePort() string {
	return c.ServicePort
}

func (c *mdsConfig) GetServiceClientPort() string {
	return ""
}

func (c *mdsConfig) GetServiceDummyPort() string {
	return c.ServiceDummyPort
}

func (c *mdsConfig) GetServiceProxyPort() string {
	return ""
}

func (c *mdsConfig) GetServiceExternalAddr() string {
	return ""
}

func (c *mdsConfig) GetServiceExternalPort() string {
	return ""
}

func (c *mdsConfig) GetLogDir() string {
	return ""
}

func (c *mdsConfig) GetDataDir() string {
	return ""
}

// cluster
func (c *mdsConfig) GetClusterEtcdHttpAddr() string {
	return ""
}

func (c *mdsConfig) GetClusterEtcdAddr() string {
	return c.ClusterEtcdAddr
}

func (c *mdsConfig) GetClusterMdsAddr() string {
	return ""
}

func (c *mdsConfig) GetClusterMdsDummyAddr() string {
	return ""
}

func (c *mdsConfig) GetClusterMdsDummyPort() string {
	return ""
}

func (c *mdsConfig) GetClusterChunkserverAddr() string {
	return ""
}

func (c *mdsConfig) GetClusterMetaserverAddr() string {
	return ""
}

func (c *mdsConfig) GetClusterSnapshotcloneAddr() string {
	return ""
}

func (c *mdsConfig) GetClusterSnapshotcloneProxyAddr() string {
	return c.ClusterSnapshotcloneProxyAddr
}

func (c *mdsConfig) GetClusterSnapshotcloneDummyPort() string {
	return ""
}

func (c *mdsConfig) GetClusterSnapshotcloneNginxUpstream() string {
	return ""
}
