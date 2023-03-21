package config

const (
	// configmap to record the endpoints of etcd
	EtcdOverrideConfigMapName    = "etcd-endpoints-override"
	EtcdOvverideConfigMapDataKey = "etcdEndpoints"

	// configmap to record the endpoints of mds
	MdsOverrideConfigMapName    = "mds-endpoints-override"
	MdsOvverideConfigMapDataKey = "mdsEndpoints"

	// configuration
	// etcd.conf
	EtcdConfigMapDataKey      = "etcd.conf"
	EtcdConfigMapMountPathDir = "/curvebs/etcd/conf"

	// mds.conf
	MdsConfigMapName         = "curve-mds-config"
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

	// topology.json
	TopoJsonConfigMapName         = "topology-json-conf"
	TopoJsonConfigmapDataKey      = "topology.json"
	TopoJsonConfigmapMountPathDir = "/curvebs/tools/conf"

	// tools.conf
	ToolsConfigMapName         = "tools-conf"
	ToolsConfigMapDataKey      = "tools.conf"
	ToolsConfigMapMountPathDir = "/etc/curve"
)
