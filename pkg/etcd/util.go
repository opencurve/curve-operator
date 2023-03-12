package etcd

import (
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Cluster) getDaemonIDs() ([]string, error) {
	var daemonIDs []string
	replicas := len(c.spec.Nodes)
	if replicas != 3 {
		return nil, errors.Errorf("nodes replicas must be set to 3")
	}
	for i := 0; i < replicas; i++ {
		daemonIDs = append(daemonIDs, k8sutil.IndexToName(i))
	}
	return daemonIDs, nil
}

// getNodeInfoMap get node info for label "app=etcd"
func (c *Cluster) getNodeInfoMap() (map[string]string, error) {
	nodes, err := c.context.Clientset.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: "app=etcd",
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list all nodes")
	}
	if len(nodes.Items) != 3 {
		log.Errorf("node count must be set 3 %v", err)
		return nil, errors.Wrapf(err, "failed to list all nodes, must have 3 node, obly has %d nodes in cluster!!!", len(nodes.Items))
	}

	// Map node name and node InternalIP
	nodeNameIP := make(map[string]string)

	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == "InternalIP" {
				nodeNameIP[node.Name] = address.Address
			}
		}
	}
	return nodeNameIP, nil
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
