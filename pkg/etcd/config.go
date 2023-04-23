package etcd

import "github.com/opencurve/curve-operator/pkg/config"

// etcdConfig implements config.ConfigInterface
var _ config.ConfigInterface = &etcdConfig{}

// etcdConfig for a single etcd
type etcdConfig struct {
	Prefix                 string
	ServiceHostSequence    string
	ServiceReplicaSequence string
	ServiceAddr            string
	ServicePort            string
	ServiceClientPort      string
	ClusterEtcdHttpAddr    string

	// the name that operator gives to etcd resources in k8s metadata
	ResourceName string

	//
	CurrentConfigMapName string

	// the ID of etcd daemon ("a", "b", ...)
	DaemonID string

	// location to store data in container and local host
	DataPathMap *config.DataPathMap
}

func (c *etcdConfig) GetPrefix() string                  { return Prefix }
func (c *etcdConfig) GetServiceId() string               { return "" }
func (c *etcdConfig) GetServiceRole() string             { return "" }
func (c *etcdConfig) GetServiceHost() string             { return "" }
func (c *etcdConfig) GetServiceHostSequence() string     { return c.ServiceHostSequence }
func (c *etcdConfig) GetServiceReplicaSequence() string  { return c.ServiceReplicaSequence }
func (c *etcdConfig) GetServiceReplicasSequence() string { return "" }
func (c *etcdConfig) GetServiceAddr() string             { return c.ServiceAddr }
func (c *etcdConfig) GetServicePort() string             { return c.ServicePort }
func (c *etcdConfig) GetServiceClientPort() string       { return c.ServiceClientPort }
func (c *etcdConfig) GetServiceDummyPort() string        { return "" }
func (c *etcdConfig) GetServiceProxyPort() string        { return "" }
func (c *etcdConfig) GetServiceExternalAddr() string     { return "" }
func (c *etcdConfig) GetServiceExternalPort() string     { return "" }
func (c *etcdConfig) GetLogDir() string                  { return "" }
func (c *etcdConfig) GetDataDir() string                 { return "" }

func (c *etcdConfig) GetClusterEtcdHttpAddr() string               { return c.ClusterEtcdHttpAddr }
func (c *etcdConfig) GetClusterEtcdAddr() string                   { return "" }
func (c *etcdConfig) GetClusterMdsAddr() string                    { return "" }
func (c *etcdConfig) GetClusterMdsDummyAddr() string               { return "" }
func (c *etcdConfig) GetClusterMdsDummyPort() string               { return "" }
func (c *etcdConfig) GetClusterChunkserverAddr() string            { return "" }
func (c *etcdConfig) GetClusterSnapshotcloneAddr() string          { return "" }
func (c *etcdConfig) GetClusterSnapshotcloneProxyAddr() string     { return "" }
func (c *etcdConfig) GetClusterSnapshotcloneDummyPort() string     { return "" }
func (c *etcdConfig) GetClusterSnapshotcloneNginxUpstream() string { return "" }
