package chunkserver

// chunkserverConfig for a single chunkserver
type chunkserverConfig struct {
	// ResourceName represents the name that operator gives to chunkserver resources in k8s metadata
	ResourceName string

	// location to store data in container and local host
	DataPathMap *chunkserverDataPathMap

	// device name represents the device name of the chunkserver, each device has one chunkserver.
	DeviceName string

	// node name represents the name of the node that the chunkserver is running on.
	NodeName string

	// node ip represents the ip of the node that the chunkserver is running on.
	NodeIP string

	// port represents the chunkserver is listening on.
	Port int

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
