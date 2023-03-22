package controllers

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getNodeInfoMap get node ip by node name that user specified and return a mapping of nodeName:nodeIP
func (c *cluster) getNodeInfoMap() (map[string]string, error) {
	nodeNameIP := make(map[string]string)

	for _, nodeName := range c.Spec.Nodes {
		n, err := c.context.Clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			logger.Errorf("failed to get node %s info in cluster", nodeName)
			return nil, errors.Wrapf(err, "failed to list all nodes")
		}

		for _, address := range n.Status.Addresses {
			if address.Type == "InternalIP" {
				nodeNameIP[n.Name] = address.Address
			}
		}
	}

	return nodeNameIP, nil
}
