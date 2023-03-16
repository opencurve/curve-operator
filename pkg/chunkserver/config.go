package chunkserver

// etcdConfig for a single etcd
type chunkserverConfig struct {
	// the name that operator gives to etcd resources in k8s metadata
	ResourceName string

	// the ID of etcd daemon ("a", "b", ...)
	DaemonID string

	// location to store data in container and local host
	DataPathMap *DataPathMap
}

// A DataPathMap is a struct which contains information about where Curve daemon data is stored in
// containers and it used by chunkserver only
type DataPathMap struct {
	// HostDataDir should be set to the path on the host where the specific daemon's data is stored.
	HostDataDir string

	// ContainerDataDir should be set to the path in the container
	// where the specific daemon's data is stored.
	ContainerDataDir string
}
