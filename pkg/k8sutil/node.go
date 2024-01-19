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
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "k8sutil")

func GetNodeIpByName(nodeName string, clientset kubernetes.Interface) (string, error) {
	n, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "failed to find %s's ip by node name", nodeName)
	}

	var addr string
	for _, address := range n.Status.Addresses {
		if address.Type == "InternalIP" {
			addr = address.Address
			return addr, nil
		}
	}

	if len(addr) == 0 {
		return "", errors.Errorf("failed to get host ip of %s by node name", nodeName)
	}

	return "", nil
}

// GetValidNodes returns all nodes that are ready and is schedulable
func GetValidNodes(clientset kubernetes.Interface, nodes []string) ([]v1.Node, error) {
	validNodes := []v1.Node{}
	for _, node := range nodes {
		n, err := clientset.CoreV1().Nodes().Get(node, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		// not scheduled
		if n.Spec.Unschedulable {
			continue
		}

		// ready status
		for _, c := range n.Status.Conditions {
			if c.Type == v1.NodeReady && c.Status == v1.ConditionTrue {
				validNodes = append(validNodes, *n)
			}
		}
	}

	return validNodes, nil
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
