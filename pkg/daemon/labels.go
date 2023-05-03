package daemon

const (
	AppAttr         = "app"
	ClusterAttr     = "curve_cluster"
	daemonTypeLabel = "curve_daemon_type"
	DaemonIDLabel   = "ceph_daemon_id"
	ResourceKind    = "kind"
)

// AppLabels returns labels common for all Rook-Ceph applications which may be useful for admins.
// App name is the name of the application: e.g., 'rook-ceph-mon', 'rook-ceph-mgr', etc.
func AppLabels(appName, namespace string) map[string]string {
	return map[string]string{
		AppAttr:     appName,
		ClusterAttr: namespace,
	}
}

// CephDaemonAppLabels returns pod labels common to all Rook-Ceph pods which may be useful for admins.
// App name is the name of the application: e.g., 'rook-ceph-mon', 'rook-ceph-mgr', etc
// Daemon type is the Ceph daemon type: "mon", "mgr", "osd", "mds", "rgw"
// Daemon ID is the ID portion of the Ceph daemon name: "a" for "mon.a"; "c" for "mds.c"
// ResourceKind is the CR type: "CephCluster", "CephFilesystem", etc
func CephDaemonAppLabels(appName, namespace, daemonType, daemonID, resourceKind string) map[string]string {
	labels := AppLabels(appName, namespace)
	labels[daemonTypeLabel] = daemonType
	labels[DaemonIDLabel] = daemonID
	labels[daemonType] = daemonID
	labels[ResourceKind] = resourceKind
	return labels
}
