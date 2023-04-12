package chunkserver

import (
	"encoding/json"
	"fmt"
	"sort"
)

const (
	KIND_CURVEBS     = "curvebs"
	KIND_CURVEFS     = "curvefs"
	ROLE_CHUNKSERVER = "chunkserver"
	ROLE_METASERVER  = "metaserver"

	DEFAULT_CHUNKSERVER_COPYSETS = 100
	DEFAULT_REPLICAS_PER_COPYSET = 3
	DEFAULT_ZONES_PER_POOL       = 3
	DEFAULT_TYPE                 = 0
	DEFAULT_SCATTER_WIDTH        = 0
)

// Generate topology.json file below from curveadm

/*
 * curvebs_cluster_topo:
 *   servers:
 *     - name: server1
 *       internalip: 127.0.0.1
 *       internalport: 16701
 *       externalip: 127.0.0.1
 *       externalport: 16701
 *       zone: zone1
 *       physicalpool: pool1
 *    ...
 *   logicalpools:
 *     - name: pool1
 *       physicalpool: pool1
 *       replicasnum: 3
 *       copysetnum: 100
 *       zonenum: 3
 *       type: 0
 *       scatterwidth: 0
 *     ...
 *
 *
 * curvefs_cluster_topo:
 *   servers:
 *     - name: server1
 *       internalip: 127.0.0.1
 *       internalport: 16701
 *       externalip: 127.0.0.1
 *       externalport: 16701
 *       zone: zone1
 *       pool: pool1
 *     ...
 *   pools:
 *     - name: pool1
 *       replicasnum: 3
 *       copysetnum: 100
 *       zonenum: 3
 */

type (
	LogicalPool struct {
		Name         string `json:"name"`
		Replicas     int    `json:"replicasnum"`
		Zones        int    `json:"zonenum"`
		Copysets     int    `json:"copysetnum"`
		Type         int    `json:"type"`         // curvebs
		ScatterWidth int    `json:"scatterwidth"` // curvebs
		PhysicalPool string `json:"physicalpool"` // curvebs
	}

	Server struct {
		Name         string `json:"name"`
		InternalIp   string `json:"internalip"`
		InternalPort int    `json:"internalport"`
		ExternalIp   string `json:"externalip"`
		ExternalPort int    `json:"externalport"`
		Zone         string `json:"zone"`
		PhysicalPool string `json:"physicalpool,omitempty"` // curvebs
		Pool         string `json:"pool,omitempty"`         // curvefs
	}

	CurveClusterTopo struct {
		Servers      []Server      `json:"servers"`
		LogicalPools []LogicalPool `json:"logicalpools,omitempty"` // curvebs
		Pools        []LogicalPool `json:"pools,omitempty"`        // curvefs
		NPools       int           `json:"npools"`
	}
)

func genNextZone(zones int) func() string {
	idx := 0
	return func() string {
		idx++
		return fmt.Sprintf("zone%d", (idx-1)%zones+1)
	}
}

func formatName(dc *chunkserverConfig) string {
	return fmt.Sprintf("%s_%d", dc.NodeName, dc.ReplicasSequence)
}

// we should sort the "dcs" for generate correct zone number
func SortDeployConfigs() {
	sort.Slice(chunkserverConfigs, func(i, j int) bool {
		csServer1, csServer2 := chunkserverConfigs[i], chunkserverConfigs[j]

		if csServer1.HostSequence == csServer2.HostSequence {
			return csServer1.ReplicasSequence < csServer2.ReplicasSequence
		}

		return csServer1.HostSequence < csServer2.HostSequence
	})
}

// createLogicalPool
func (c *Cluster) createLogicalPool(logicalPool string) (LogicalPool, []Server) {
	var zone string
	copysets := 0
	servers := []Server{}
	zones := DEFAULT_ZONES_PER_POOL
	nextZone := genNextZone(zones)
	physicalPool := logicalPool

	// ensure the number of copysets on one node
	copysetsPerChunkserver := DEFAULT_CHUNKSERVER_COPYSETS
	if c.spec.Storage.CopySets != 0 {
		copysetsPerChunkserver = c.spec.Storage.CopySets
	}
	// !important
	SortDeployConfigs()

	for _, csConfig := range chunkserverConfigs {
		if csConfig.ReplicasSequence == 0 {
			zone = nextZone()
		}

		// NOTE: if we deploy chunkservers with replica feature
		// and the value of replica greater than 1, we should
		// set internal port and external port to 0 for let MDS
		// attribute them as services on the same machine.
		// see issue: https://github.com/opencurve/curve/issues/1441
		internalPort := csConfig.Port
		externalPort := csConfig.Port
		if csConfig.Replicas > 1 {
			internalPort = 0
			externalPort = 0
		}

		// json Server field
		server := Server{
			Name:         formatName(&csConfig),
			InternalIp:   csConfig.NodeIP,
			InternalPort: internalPort,
			ExternalIp:   csConfig.NodeIP,
			ExternalPort: externalPort,
			Zone:         zone,
		}

		server.PhysicalPool = physicalPool

		// copysets number ddefault value is 100
		copysets += copysetsPerChunkserver
		servers = append(servers, server)

	}

	// copysets
	copysets = copysets / DEFAULT_REPLICAS_PER_COPYSET
	if copysets == 0 {
		copysets = 1
	}

	// logical pool field in topology.json file
	lpool := LogicalPool{
		Name:     logicalPool,
		Copysets: copysets,
		Zones:    zones,
		Replicas: DEFAULT_REPLICAS_PER_COPYSET,
	}

	lpool.ScatterWidth = DEFAULT_SCATTER_WIDTH
	lpool.Type = DEFAULT_TYPE
	lpool.PhysicalPool = physicalPool

	return lpool, servers
}

func (c *Cluster) genClusterPool() string {
	// create CurveClusterTopo object by call createLogicalPool
	lpool, servers := c.createLogicalPool("pool1")
	topo := CurveClusterTopo{Servers: servers, NPools: 1}

	// curvebs
	topo.LogicalPools = []LogicalPool{lpool}

	// generate the topology.json
	var bytes []byte
	bytes, err := json.Marshal(topo)
	if err != nil {
		return ""
	}
	clusterPoolJson := string(bytes)
	logger.Info(clusterPoolJson)
	return clusterPoolJson
}

func (c *Cluster) getRegisterJobLabel(poolType string) map[string]string {
	labels := make(map[string]string)
	labels["app"] = RegisterJobName
	labels["pool"] = poolType
	return labels
}
