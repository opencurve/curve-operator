package k8sutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
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

func GetValidDaemonHosts(c clusterd.Context, curveCluster *curvev1.CurveCluster) ([]v1.Node, error) {
	daemonHosts := curveCluster.Spec.Nodes
	validDaemonHosts, err := GetValidNodes(c, daemonHosts)
	return validDaemonHosts, err
}

func GetValidChunkserverHosts(c clusterd.Context, curveCluster *curvev1.CurveCluster) ([]v1.Node, error) {
	if !curveCluster.Spec.Storage.UseSelectedNodes {
		chunkserverHosts := curveCluster.Spec.Storage.Nodes
		validNodes, err := GetValidNodes(c, chunkserverHosts)
		return validNodes, err
	}
	// useSelectedNodes == true
	var chunkserverHosts []string

	for _, s := range curveCluster.Spec.Storage.SelectedNodes {
		chunkserverHosts = append(chunkserverHosts, s.Node)
	}
	valiedChunkHosts, err := GetValidNodes(c, chunkserverHosts)

	return valiedChunkHosts, err
}

func MergeNodesOfDaemonAndChunk(daemonHosts []v1.Node, chunkserverHosts []v1.Node) []v1.Node {
	var nodes []v1.Node
	nodes = append(nodes, daemonHosts...)
	nodes = append(nodes, chunkserverHosts...)

	var retNodes []v1.Node
	tmpMap := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		if _, ok := tmpMap[n.Name]; !ok {
			tmpMap[n.Name] = struct{}{}
			retNodes = append(retNodes, n)
		}
	}
	return retNodes
}

// TruncateNodeNameForJob hashes the nodeName in case it would case the name to be longer than 63 characters
// and avoids for a K8s 1.22 bug in the job pod name generation. If the job name contains a . or - in a certain
// position, the pod will fail to create.
func TruncateNodeNameForJob(format, nodeName string) string {
	// In k8s 1.22, the job name is truncated an additional 10 characters which can cause an issue
	// in the generated pod name if it then ends in a non-alphanumeric character. In that case,
	// we more aggressively generate a hashed job name.
	jobNameShortenLength := 10
	return truncateNodeName(format, nodeName, validation.DNS1035LabelMaxLength-jobNameShortenLength)
}

// Hash stableName computes a stable pseudorandom string suitable for inclusion in a Kubernetes object name from the given seed string.
// Do **NOT** edit this function in a way that would change its output as it needs to
// provide consistent mappings from string to hash across versions of rook.
func Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}

// truncateNodeName takes the max length desired for a string and hashes the value if needed to shorten it.
func truncateNodeName(format, nodeName string, maxLength int) string {
	if len(nodeName)+len(fmt.Sprintf(format, "")) > maxLength {
		hashed := Hash(nodeName)
		logger.Infof("format and nodeName longer than %d chars, nodeName %s will be %s", maxLength, nodeName, hashed)
		nodeName = hashed
	}
	return fmt.Sprintf(format, nodeName)
}

// GetNodeHostName returns the hostname label given the node name.
func GetNodeHostName(ctx context.Context, clientset kubernetes.Interface, nodeName string) (string, error) {
	node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return GetNodeHostNameLabel(node)
}

func GetNodeHostNameLabel(node *v1.Node) (string, error) {
	hostname, ok := node.Labels[v1.LabelHostname]
	if !ok {
		return "", fmt.Errorf("hostname not found on the node")
	}
	return hostname, nil
}
