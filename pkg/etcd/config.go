package etcd

import "github.com/opencurve/curve-operator/pkg/config"

// etcdConfig for a single etcd
type etcdConfig struct {
	// the name that operator gives to etcd resources in k8s metadata
	ResourceName string

	// the ID of etcd daemon ("a", "b", ...)
	DaemonID string

	// location to store data in container and local host
	DataPathMap *config.DataPathMap
}
