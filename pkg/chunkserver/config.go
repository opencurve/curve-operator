package chunkserver

// chunkserverConfig for a single chunkserver
type chunkserverConfig struct {
	// the name that operator gives to chunkserver resources in k8s metadata
	ResourceName string

	// location to store data in container and local host
	DataPathMap *chunkserverDataPathMap

	// node name
	NodeName string

	// device name
	DeviceName string

	// port
	Port int
}

// chunkserverDataPathMap represents the device on host and referred Mount Path in container
type chunkserverDataPathMap struct {
	// HostDevice is the device name such as '/dev/sdb'
	HostDevice string

	// ContainerDataDir is the data dir of chunkserver such as '/curvebs/chunkserver/data/'
	ContainerDataDir string

	// ContainerLogDir is the log dir of chunkserver such as '/curvebs/chunkserver/logs'
	ContainerLogDir string
}

type configData struct {
	data map[string]string
}
