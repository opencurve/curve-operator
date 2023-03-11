package config

const (
	// configmap to record the endpoints of etcd
	OverrideCM        = "etcd-endpoints-override"
	OvverideCMDataKey = "etcdEndpoints"

	// etcd
	EtcdConfigMapDataKey      = "etcd.conf"
	EtcdConfigMapMountPathDir = "/curvebs/etcd/conf"
	// mds
)
