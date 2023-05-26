package config

import "github.com/coreos/pkg/capnslog"

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "config")

const (
	KIND_CURVEBS = "curvebs"
	KIND_CURVEFS = "curvefs"
)

const (
	ETCD_ROLE          = "etcd"
	MDS_ROLE           = "mds"
	SNAPSHOTCLONE_ROLE = "snapshotclone"
	METASERVER_ROLE    = "metaserver"
	CHUNKSERVER_ROLE   = "chunkserver"
)

const (
	// configmap to record the endpoints of etcd
	EtcdOverrideConfigMapName    = "etcd-endpoints-override"
	EtcdOvverideConfigMapDataKey = "etcdEndpoints"
	ClusterEtcdAddr              = "clusterEtcdAddr"

	// configmap to record the endpoints of mds
	MdsOverrideConfigMapName    = "mds-endpoints-override"
	MdsOvverideConfigMapDataKey = "mdsEndpoints"
	ClusterMdsDummyAddr         = "clusterMdsDummyAddr"
	ClusterMdsDummyPort         = "clusterMdsDummyPort"

	// configuration
	// etcd.conf - it not be used
	EtcdConfigMapName           = "curve-etcd-conf"
	EtcdConfigMapDataKey        = "etcd.conf"
	EtcdConfigMapMountPathDir   = "/curvebs/etcd/conf"
	FSEtcdConfigMapMountPathDir = "/curvefs/etcd/conf"
	FSMdsConfigMapMountPathDir  = "/curvefs/mds/conf"

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
	S3ConfigMapName             = "s3-conf"
	S3ConfigMapDataKey          = "s3.conf"
	S3ConfigMapMountPathDir     = "/curvebs/chunkserver/conf"
	S3ConfigMapMountSnapPathDir = "/curvebs/snapshotclone/conf"

	// topology.json
	TopoJsonConfigMapName           = "topology-json-conf"
	TopoJsonConfigmapDataKey        = "topology.json"
	TopoJsonConfigmapMountPathDir   = "/curvebs/tools/conf"
	FSTopoJsonConfigmapMountPathDir = "/curvefs/tools/conf"

	// tools.conf
	ToolsConfigMapName           = "tools-conf"
	ToolsConfigMapDataKey        = "tools.conf"
	ToolsConfigMapMountPathDir   = "/etc/curve"
	FSToolsConfigMapMountPathDir = "/etc/curvefs"

	// snap_client.conf
	SnapClientConfigMapName      = "snap-client-conf"
	SnapClientConfigMapDataKey   = "snap_client.conf"
	SnapClientConfigMapMountPath = "/curvebs/snapshotclone/conf"

	// snapshotclone.conf
	SnapShotCloneConfigMapName      = "snapshotclone-conf"
	SnapShotCloneConfigMapDataKey   = "snapshotclone.conf"
	SnapShotCloneConfigMapMountPath = "/curvebs/snapshotclone/conf"

	// nginx.conf
	NginxConfigMapName      = "nginx-conf"
	NginxConfigMapDataKey   = "nginx.conf"
	NginxConfigMapMountPath = "/curvebs/snapshotclone/conf"

	// start nginx.conf
	StartSnapConfigMap          = "start-snap-conf"
	StartSnapConfigMapDataKey   = "start_snap.sh"
	StartSnapConfigMapMountPath = "/curvebs/tools/sbin/start_snap.sh"

	// metaserver.conf
	MetaServerConfigMapName      = "metaserver-conf"
	MetaServerConfigMapDataKey   = "metaserver.conf"
	MetaServerConfigMapMountPath = "/curvefs/metaserver/conf"

	// prometheus.yaml
	PrometheusConfigMapName    = "prometheus-conf"
	PrometheusConfigMapDataKey = "prometheus.yml"

	// grafana datasource yaml
	GrafanaDataSourcesConfigMapName      = "grafana-conf"
	GrafanaDataSourcesConfigMapDataKey   = "all.yml"
	GrafanaDataSourcesConfigMapMountPath = "/etc/grafana/provisioning/datasources"

	// grafana dashboards
	GrafanaDashboardsMountPath = "/etc/grafana/provisioning/dashboards"

	// grafana INI config
	GrafanaINIConfigMapDataKey = "grafana.ini"
	GrafanaINIConfigMountPath  = "/etc/grafana"
)

const (
	EtcdConfigTemp             = "etcd-conf-template"
	MdsConfigMapTemp           = "mds-conf-template"
	ChunkServerConfigMapTemp   = "chunkserver-conf-template"
	S3ConfigMapTemp            = "s3-conf-template"
	SnapShotCloneConfigMapTemp = "snapshotclone-conf-template"
	CsClientConfigMapTemp      = "cs-conf-template"
	SnapClientConfigMapTemp    = "snap-conf-template"
	ToolsConfigMapTemp         = "tools-conf-template"
	NginxConfigMapTemp         = "nginx-conf-template"
	MetaserverConfigMapTemp    = "metaserver-conf-template"
	GrafanaDashboardsTemp      = "grafana-dashboard-template"
)
