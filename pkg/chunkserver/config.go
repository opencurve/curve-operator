package chunkserver

import "strconv"

// chunkserverConfig for a single chunkserver
type chunkserverConfig struct {
	Prefix                        string
	Port                          int    // chunkserver.conf(service_port)
	ClusterMdsAddr                string // chunkserver.conf, snap_client.conf, tools.conf
	ClusterMdsDummyPort           string // tools.conf
	ClusterEtcdAddr               string // tools.conf
	ClusterSnapshotcloneAddr      string // tools.conf
	ClusterSnapshotcloneDummyPort string // tools.conf

	// ResourceName represents the name that operator gives to chunkserver resources in k8s metadata
	ResourceName string

	//
	CurrentConfigMapName string

	// location to store data in container and local host
	DataPathMap *chunkserverDataPathMap

	// device name represents the device name of the chunkserver, each device has one chunkserver.
	DeviceName string

	// node name represents the name of the node that the chunkserver is running on.
	NodeName string

	// node ip represents the ip of the node that the chunkserver is running on.
	NodeIP string

	// host sequence is the host(node) number.
	HostSequence int

	// replicas sequence represent the chunkserver replicas sequence number on the node.
	ReplicasSequence int

	// replicas represents the chunkserver replicas on the node.
	Replicas int
}

// chunkserverDataPathMap represents the device on host and referred Mount Path in container
type chunkserverDataPathMap struct {
	// HostDevice is the device name such as '/dev/sdb'
	HostDevice string

	// HostLogDir
	HostLogDir string

	// ContainerDataDir is the data dir of chunkserver such as '/curvebs/chunkserver/data/'
	ContainerDataDir string

	// ContainerLogDir is the log dir of chunkserver such as '/curvebs/chunkserver/logs'
	ContainerLogDir string
}

type configData struct {
	data map[string]string
}

// chunkserverConfig implement ConfigInterface
func (c *chunkserverConfig) GetPrefix() string {
	return c.Prefix
}

func (c *chunkserverConfig) GetServiceId() string {
	return ""
}

func (c *chunkserverConfig) GetServiceRole() string {
	return ""
}

func (c *chunkserverConfig) GetServiceHost() string {
	return ""
}

func (c *chunkserverConfig) GetServiceHostSequence() string {
	return ""
}

func (c *chunkserverConfig) GetServiceReplicaSequence() string {
	return ""
}

func (c *chunkserverConfig) GetServiceReplicasSequence() string {
	return ""
}

func (c *chunkserverConfig) GetServiceAddr() string {
	return ""
}

func (c *chunkserverConfig) GetServicePort() string {
	return strconv.Itoa(c.Port)
}

func (c *chunkserverConfig) GetServiceClientPort() string {
	return ""
}

func (c *chunkserverConfig) GetServiceDummyPort() string {
	return ""
}

func (c *chunkserverConfig) GetServiceProxyPort() string {
	return ""
}

func (c *chunkserverConfig) GetServiceExternalAddr() string {
	return ""
}

func (c *chunkserverConfig) GetServiceExternalPort() string {
	return ""
}

func (c *chunkserverConfig) GetLogDir() string {
	return ""
}

func (c *chunkserverConfig) GetDataDir() string {
	return ""
}

// cluster
func (c *chunkserverConfig) GetClusterEtcdHttpAddr() string {
	return ""
}

func (c *chunkserverConfig) GetClusterEtcdAddr() string {
	return c.ClusterEtcdAddr
}

func (c *chunkserverConfig) GetClusterMdsAddr() string {
	return c.ClusterMdsAddr
}

func (c *chunkserverConfig) GetClusterMdsDummyAddr() string {
	return ""
}

func (c *chunkserverConfig) GetClusterMdsDummyPort() string {
	return c.ClusterMdsDummyPort
}

func (c *chunkserverConfig) GetClusterChunkserverAddr() string {
	return ""
}

func (c *chunkserverConfig) GetClusterSnapshotcloneAddr() string {
	return c.ClusterSnapshotcloneAddr
}

func (c *chunkserverConfig) GetClusterSnapshotcloneProxyAddr() string {
	return ""
}

func (c *chunkserverConfig) GetClusterSnapshotcloneDummyPort() string {
	return c.ClusterSnapshotcloneDummyPort
}

func (c *chunkserverConfig) GetClusterSnapshotcloneNginxUpstream() string {
	return ""
}
