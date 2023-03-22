package snapshotclone

import "github.com/opencurve/curve-operator/pkg/config"

// mdsConfig for a single mds
type snapConfig struct {
	// the name that operator gives to mds resources in k8s metadata
	ResourceName string

	// the ID of etcd daemon ("a", "b", ...)
	DaemonID string

	// location to store data in container and local host
	DataPathMap *config.DataPathMap
}
