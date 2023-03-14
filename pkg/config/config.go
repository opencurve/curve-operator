package config

const (
	// configmap to record the endpoints of etcd
	OverrideCM        = "etcd-endpoints-override"
	OverrideCMDataKey = "etcdEndpoints"

	// etcd
	EtcdConfigMapDataKey      = "etcd.conf"
	EtcdConfigMapMountPathDir = "/curvebs/etcd/conf"

	// mds
	MdsConfigMapDataKey      = "mds.conf"
	MdsConfigMapMountPathDir = "/curvebs/mds/conf"
)
