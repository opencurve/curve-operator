package etcd

import (
	"github.com/pkg/errors"

	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

func (c *Cluster) getDaemonIDs() ([]string, error) {
	var daemonIDs []string
	replicas := len(c.spec.Nodes)
	if replicas != 3 {
		return nil, errors.New("nodes replicas must be set to 3")
	}
	for i := 0; i < replicas; i++ {
		daemonIDs = append(daemonIDs, k8sutil.IndexToName(i))
	}
	return daemonIDs, nil
}

// getLabels Add labels for etcd deployment
func (c *Cluster) getPodLabels(etcdConfig *etcdConfig) map[string]string {
	labels := make(map[string]string)
	labels["app"] = AppName
	labels["etcd"] = etcdConfig.DaemonID
	labels["curve_daemon_id"] = etcdConfig.DaemonID
	labels["curve_cluster"] = c.namespacedName.Namespace
	return labels
}
