package k8sutil

import (
	"github.com/coreos/pkg/capnslog"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "k8sutil")

// GetNodeHostNames returns the name of the node resource mapped to their hostname label.
// Typically these will be the same name, but sometimes they are not such as when nodes have a longer
// dns name, but the hostname is short.
func GetNodeHostNames(clientset kubernetes.Interface) (map[string]string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeMap := map[string]string{}
	for _, node := range nodes.Items {
		nodeMap[node.Name] = node.Labels[v1.LabelHostname]
	}
	return nodeMap, nil
}

// GetValidNodes returns all nodes that are ready and is schedulable
func GetValidNodes(c clusterd.Context, storageNodes []string) ([]v1.Node, error) {
	nodes := []v1.Node{}
	for _, curveNode := range storageNodes {
		n, err := c.Clientset.CoreV1().Nodes().Get(curveNode, metav1.GetOptions{})
		if err != nil {
			logger.Errorf("failed to get node %v info", curveNode)
			return nil, errors.Wrap(err, "failed to get node info by node name")
		}

		// not scheduled
		if n.Spec.Unschedulable {
			continue
		}

		// ready status
		for _, c := range n.Status.Conditions {
			if c.Type == v1.NodeReady && c.Status == v1.ConditionTrue {
				nodes = append(nodes, *n)
			}
		}
	}

	return nodes, nil
}
