package monitor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/topology"
	"github.com/pkg/errors"
)

// filterNodeForExporter distinct nodes and return nodes ip
func filterNodeForExporter(nodesInfo []daemon.NodeInfo) []string {
	var preNodeName string
	var ret []string
	for _, node := range nodesInfo {
		if node.NodeName != preNodeName {
			ret = append(ret, node.NodeIP)
		}
		preNodeName = node.NodeName
	}
	return ret
}

// filterNodeNameForExporter distinct nodes and return nodes name
func filterNodeNameForExporter(nodesInfo []daemon.NodeInfo) []string {
	var preNodeName string
	var ret []string
	for _, node := range nodesInfo {
		if node.NodeName != preNodeName {
			ret = append(ret, node.NodeName)
		}
		preNodeName = node.NodeName
	}
	return ret
}

// getExporterEndpoints get nodes that to deploy node-exporter on it
func (c *Cluster) getExporterEndpoints(nodeIPs []string) string {
	endpoint := []string{}
	for _, item := range nodeIPs {
		endpoint = append(endpoint, fmt.Sprintf("'%s:%d'", item, c.Monitor.NodeExporter.ListenPort))
	}
	return fmt.Sprintf("[%s]", strings.Join(endpoint, ","))
}

// parsePrometheusTarget parse topology and create target.json string.
func parsePrometheusTarget(dcs []*topology.DeployConfig) (string, error) {
	targets := []serviceTarget{}
	tMap := make(map[string]serviceTarget)
	for _, dc := range dcs {
		role := dc.Role
		ip := dc.NodeIP
		item := fmt.Sprintf("%s:%d", ip, dc.Port)
		if _, ok := tMap[role]; ok {
			t := tMap[role]
			t.Targets = append(t.Targets, item)
			tMap[role] = t
		} else {
			tMap[role] = serviceTarget{
				Labels:  map[string]string{"job": role},
				Targets: []string{item},
			}
		}
	}
	for _, v := range tMap {
		targets = append(targets, v)
	}
	target, err := json.Marshal(targets)
	if err != nil {
		return "", errors.New("failed to parse prometheus target")
	}
	return string(target), nil
}
