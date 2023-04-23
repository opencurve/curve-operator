package mds

import "github.com/opencurve/curve-operator/pkg/config"

// mdsConfig implements config.ConfigInterface
var _ config.ConfigInterface = &mdsConfig{}

// mdsConfig for a single mds
type mdsConfig struct {
	ServiceAddr                   string
	ServicePort                   string
	ServiceDummyPort              string
	ClusterEtcdAddr               string
	ClusterSnapshotcloneProxyAddr string

	// the name that operator gives to mds resources in k8s metadata
	ResourceName string

	// The referred configmap name for current mds daemon
	CurrentConfigMapName string

	// the ID of etcd daemon ("a", "b", ...)
	DaemonID string

	// location to store data in container and local host
	DataPathMap *config.DataPathMap
}

func (c *mdsConfig) GetPrefix() string                  { return Prefix }
func (c *mdsConfig) GetServiceId() string               { return "" }
func (c *mdsConfig) GetServiceRole() string             { return "mds" }
func (c *mdsConfig) GetServiceHost() string             { return "" }
func (c *mdsConfig) GetServiceHostSequence() string     { return "" }
func (c *mdsConfig) GetServiceReplicaSequence() string  { return "" }
func (c *mdsConfig) GetServiceReplicasSequence() string { return "" }
func (c *mdsConfig) GetServiceAddr() string             { return c.ServiceAddr }
func (c *mdsConfig) GetServicePort() string             { return c.ServicePort }
func (c *mdsConfig) GetServiceClientPort() string       { return "" }
func (c *mdsConfig) GetServiceDummyPort() string        { return c.ServiceDummyPort }
func (c *mdsConfig) GetServiceProxyPort() string        { return "" }
func (c *mdsConfig) GetServiceExternalAddr() string     { return "" }
func (c *mdsConfig) GetServiceExternalPort() string     { return "" }
func (c *mdsConfig) GetLogDir() string                  { return "" }
func (c *mdsConfig) GetDataDir() string                 { return "" }

func (c *mdsConfig) GetClusterEtcdHttpAddr() string               { return "" }
func (c *mdsConfig) GetClusterEtcdAddr() string                   { return c.ClusterEtcdAddr }
func (c *mdsConfig) GetClusterMdsAddr() string                    { return "" }
func (c *mdsConfig) GetClusterMdsDummyAddr() string               { return "" }
func (c *mdsConfig) GetClusterMdsDummyPort() string               { return "" }
func (c *mdsConfig) GetClusterChunkserverAddr() string            { return "" }
func (c *mdsConfig) GetClusterSnapshotcloneAddr() string          { return "" }
func (c *mdsConfig) GetClusterSnapshotcloneProxyAddr() string     { return c.ClusterSnapshotcloneProxyAddr }
func (c *mdsConfig) GetClusterSnapshotcloneDummyPort() string     { return "" }
func (c *mdsConfig) GetClusterSnapshotcloneNginxUpstream() string { return "" }
