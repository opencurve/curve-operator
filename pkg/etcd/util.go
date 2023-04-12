package etcd

// getLabels adds labels for etcd deployment
func (c *Cluster) getPodLabels(etcdConfig *etcdConfig) map[string]string {
	return map[string]string{
		"app":             AppName,
		"etcd":            etcdConfig.DaemonID,
		"curve_daemon_id": etcdConfig.DaemonID,
		"curve_cluster":   c.namespacedName.Namespace,
	}
}
