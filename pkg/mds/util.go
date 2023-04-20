package mds

// getLabels Add labels for mds deployment
func (c *Cluster) getPodLabels(mdsConfig *mdsConfig) map[string]string {
	labels := make(map[string]string)
	labels["app"] = AppName
	labels["mds"] = mdsConfig.DaemonID
	labels["curve_daemon_id"] = mdsConfig.DaemonID
	labels["curve_cluster"] = c.NamespacedName.Namespace
	return labels
}
