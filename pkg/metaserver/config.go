package metaserver

import "github.com/opencurve/curve-operator/pkg/config"

type metaserverConfig struct {
	Prefix                string
	ServiceAddr           string
	ServicePort           string
	ServiceExternalAddr   string
	ServiceExternalPort   string
	ClusterEtcdAddr       string
	ClusterMdsAddr        string
	ClusterMdsDummyAddr   string
	ClusterMetaserverAddr string

	ResourceName         string
	CurrentConfigMapName string
	DaemonID             string
	NodeName             string
	NodeIP               string
	DataPathMap          *config.DataPathMap
}

func (c *metaserverConfig) GetPrefix() string                  { return c.Prefix }
func (c *metaserverConfig) GetServiceId() string               { return "" }
func (c *metaserverConfig) GetServiceRole() string             { return "metaserver" }
func (c *metaserverConfig) GetServiceHost() string             { return "" }
func (c *metaserverConfig) GetServiceHostSequence() string     { return "" }
func (c *metaserverConfig) GetServiceReplicaSequence() string  { return "" }
func (c *metaserverConfig) GetServiceReplicasSequence() string { return "" }
func (c *metaserverConfig) GetServiceAddr() string             { return c.ServiceAddr }
func (c *metaserverConfig) GetServicePort() string             { return c.ServicePort }
func (c *metaserverConfig) GetServiceClientPort() string       { return "" }
func (c *metaserverConfig) GetServiceDummyPort() string        { return "" }
func (c *metaserverConfig) GetServiceProxyPort() string        { return "" }
func (c *metaserverConfig) GetServiceExternalAddr() string     { return c.ServiceExternalAddr }
func (c *metaserverConfig) GetServiceExternalPort() string     { return c.ServiceExternalPort }
func (c *metaserverConfig) GetLogDir() string                  { return "" }
func (c *metaserverConfig) GetDataDir() string                 { return "" }

// cluster
func (c *metaserverConfig) GetClusterEtcdHttpAddr() string               { return "" }
func (c *metaserverConfig) GetClusterEtcdAddr() string                   { return c.ClusterEtcdAddr }
func (c *metaserverConfig) GetClusterMdsAddr() string                    { return c.ClusterMdsAddr }
func (c *metaserverConfig) GetClusterMdsDummyAddr() string               { return c.ClusterMdsDummyAddr }
func (c *metaserverConfig) GetClusterMdsDummyPort() string               { return "" }
func (c *metaserverConfig) GetClusterChunkserverAddr() string            { return "" }
func (c *metaserverConfig) GetClusterMetaserverAddr() string             { return c.ClusterMetaserverAddr }
func (c *metaserverConfig) GetClusterSnapshotcloneAddr() string          { return "" }
func (c *metaserverConfig) GetClusterSnapshotcloneProxyAddr() string     { return "" }
func (c *metaserverConfig) GetClusterSnapshotcloneDummyPort() string     { return "" }
func (c *metaserverConfig) GetClusterSnapshotcloneNginxUpstream() string { return "" }
