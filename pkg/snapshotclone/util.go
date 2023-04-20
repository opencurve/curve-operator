package snapshotclone

// getLabels Add labels for mds deployment
func (c *Cluster) getPodLabels(snapConfig *snapConfig) map[string]string {
	labels := make(map[string]string)
	labels["app"] = AppName
	labels["snapshotclone"] = snapConfig.DaemonID
	labels["curve_daemon_id"] = snapConfig.DaemonID
	labels["curve_cluster"] = c.NamespacedName.Namespace
	return labels
}
