package config

const (
	// configmap to record the endpoints of etcd
	OverrideCM        = "etcd-endpoints-override"
	OvverideCMDataKey = "etcdEndpoints"

	// configmap to record the endpoints of mds
	MdsOverrideCM        = "mds-endpoints-override"
	MdsOvverideCMDataKey = "mdsEndpoints"

	// configuration
	// etcd.conf
	EtcdConfigMapDataKey      = "etcd.conf"
	EtcdConfigMapMountPathDir = "/curvebs/etcd/conf"

	// mds.conf
	MdsConfigMapDataKey      = "mds.conf"
	MdsConfigMapMountPathDir = "/curvebs/mds/conf"

	// chunkserver.conf
	ChunkserverConfigMapName         = "curve-chunkserver-conf"
	ChunkserverConfigMapDataKey      = "chunkserver.conf"
	ChunkserverConfigMapMountPathDir = "/curvebs/chunkserver/conf"

	// cs_client.conf
	CSClientConfigMapName         = "cs-client-conf"
	CSClientConfigMapDataKey      = "cs_client.conf"
	CSClientConfigMapMountPathDir = "/curvebs/chunkserver/conf"

	// s3.conf
	S3ConfigMapName         = "s3-conf"
	S3ConfigMapDataKey      = "s3.conf"
	S3ConfigMapMountPathDir = "/curvebs/chunkserver/conf"
)
