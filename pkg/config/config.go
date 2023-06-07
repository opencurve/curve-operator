package config

import "github.com/coreos/pkg/capnslog"

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "config")

const (
	KIND_CURVEBS = "curvebs"
	KIND_CURVEFS = "curvefs"
)

const (
	ROLE_ETCD          = "etcd"
	ROLE_MDS           = "mds"
	ROLE_SNAPSHOTCLONE = "snapshotclone"
	ROLE_METASERVER    = "metaserver"
	ROLE_CHUNKSERVER   = "chunkserver"
)

const (
	// template configmap
	DefaultConfigMapName = "curve-conf-default"

	// all chunkserver config map
	ChunkserverAllConfigMapName   = "chunkserver-all-config"
	SnapShotCloneAllConfigMapName = "snapshotclone-all-config"

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
	// etcd.conf
	EtcdConfigMapDataKey        = "etcd.conf"
	EtcdConfigMapMountPathDir   = "/curvebs/etcd/conf"
	FSEtcdConfigMapMountPathDir = "/curvefs/etcd/conf"
	FSMdsConfigMapMountPathDir  = "/curvefs/mds/conf"

	// mds.conf
	MdsConfigMapDataKey      = "mds.conf"
	MdsConfigMapMountPathDir = "/curvebs/mds/conf"

	// chunkserver.conf
	ChunkserverConfigMapDataKey      = "chunkserver.conf"
	ChunkserverConfigMapMountPathDir = "/curvebs/chunkserver/conf"

	// cs_client.conf
	CSClientConfigMapDataKey      = "cs_client.conf"
	CSClientConfigMapMountPathDir = "/curvebs/chunkserver/conf"

	// s3.conf
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
	SnapClientConfigMapDataKey   = "snap_client.conf"
	SnapClientConfigMapMountPath = "/curvebs/snapshotclone/conf"

	// snapshotclone.conf
	SnapShotCloneConfigMapDataKey   = "snapshotclone.conf"
	SnapShotCloneConfigMapMountPath = "/curvebs/snapshotclone/conf"

	// nginx.conf
	NginxConfigMapDataKey   = "nginx.conf"
	NginxConfigMapMountPath = "/curvebs/snapshotclone/conf"

	// start nginx.conf
	StartSnapConfigMapDataKey   = "start_snap.sh"
	StartSnapConfigMapMountPath = "/curvebs/tools/sbin/start_snap.sh"

	// metaserver.conf
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

	// report.sh
	ReportConfigMapName            = "report-conf"
	ReportConfigMapDataKey         = "report.sh"
	ReportConfigMapMountPathCommon = "tools/sbin/report" // a new path
)

const GrafanaDashboardsTemp = "grafana-dashboard-temp"
