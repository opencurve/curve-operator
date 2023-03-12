package config

// A DataPathMap is a struct which contains information about where Curve daemon data is stored in
// containers and whether the data should be persisted to the host. If it is persisted to the host,
// directory on the host where the specific daemon's data is stored is given.
type DataPathMap struct {
	// HostDataDir should be set to the path on the host where the specific daemon's data is stored.
	HostDataDir string

	// HostLogDir should be set to the path on the host where the specific daemon's log is stored.
	HostLogDir string

	// ContainerDataDir should be set to the path in the container
	// where the specific daemon's data is stored.
	ContainerDataDir string

	// ContainerDataDir should be set to the path in the container
	// where the specific daemon's log is stored.
	ContainerLogDir string
}

// NewDaemonDataPathMap returns a new DataPathMap for a daemon which does not utilize a data
// dir in the container as the mon, mgr, osd, mds, and rgw daemons do.
func NewDaemonDataPathMap(hostDataDir string, hostLogDir string, containerDataDir string, containerLogDir string) *DataPathMap {
	return &DataPathMap{
		HostDataDir:      hostDataDir,
		HostLogDir:       hostLogDir,
		ContainerDataDir: containerDataDir,
		ContainerLogDir:  containerLogDir,
	}
}
