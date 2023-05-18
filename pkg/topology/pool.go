package topology

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/opencurve/curve-operator/pkg/config"
)

const (
	ROLE_CHUNKSERVER = "chunkserver"
	ROLE_METASERVER  = "metaserver"

	DEFAULT_CHUNKSERVER_COPYSETS = 100
	DEFAULT_REPLICAS_PER_COPYSET = 3
	DEFAULT_ZONES_PER_POOL       = 3
	DEFAULT_TYPE                 = 0
	DEFAULT_SCATTER_WIDTH        = 0
)

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

func formatName(dc *DeployConfig) string {
	return fmt.Sprintf("%s_%d", dc.NodeName, dc.ReplicasSequence)
}

// SortDeployConfigs we should sort the "dcs" for generate correct zone number
func SortDeployConfigs(dcs []*DeployConfig) {
	sort.Slice(dcs, func(i, j int) bool {
		dc1, dc2 := dcs[i], dcs[j]
		if dc1.Role == dc2.Role {
			if dc1.HostSequence == dc2.HostSequence {
				return dc1.ReplicasSequence < dc2.ReplicasSequence
			}
			return dc1.HostSequence < dc2.HostSequence
		}
		return dc1.Role < dc2.Role
	})
}

// createLogicalPool
func createLogicalPool(dcs []*DeployConfig, logicalPool string) (LogicalPool, []Server) {
	var zone string
	copysets := 0
	servers := []Server{}
	zones := DEFAULT_ZONES_PER_POOL
	nextZone := genNextZone(zones)
	physicalPool := logicalPool
	kind := dcs[0].Kind
	// !important
	SortDeployConfigs(dcs)

	for _, dc := range dcs {
		if dc.ReplicasSequence == 0 || dc.StandAlone {
			zone = nextZone()
			logger.Info("stand-alonedeployment? ", dc.StandAlone)
		}

		// NOTE: if we deploy chunkservers with replica feature
		// and the value of replica greater than 1, we should
		// set internal port and external port to 0 for let MDS
		// attribute them as services on the same machine.
		// see issue: https://github.com/opencurve/curve/issues/1441
		internalPort := dc.Port
		externalPort := dc.Port
		if dc.Replicas > 1 && !dc.StandAlone {
			internalPort = 0
			externalPort = 0
		}

		server := Server{
			Name:         formatName(dc),
			InternalIp:   dc.NodeIP,
			InternalPort: internalPort,
			ExternalIp:   dc.NodeIP,
			ExternalPort: externalPort,
			Zone:         zone,
		}
		if kind == config.KIND_CURVEBS {
			server.PhysicalPool = physicalPool
		} else {
			server.Pool = logicalPool
		}

		// copysets number ddefault value is 100
		copysets += dc.Copysets
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
	if kind == config.KIND_CURVEBS {
		lpool.ScatterWidth = DEFAULT_SCATTER_WIDTH
		lpool.Type = DEFAULT_TYPE
		lpool.PhysicalPool = physicalPool
	}
	return lpool, servers
}

func genClusterPool(dcs []*DeployConfig) string {
	// create CurveClusterTopo object by call createLogicalPool
	lpool, servers := createLogicalPool(dcs, "pool1")
	topo := CurveClusterTopo{Servers: servers, NPools: 1}

	if dcs[0].Kind == config.KIND_CURVEBS {
		topo.LogicalPools = []LogicalPool{lpool}
	} else {
		topo.Pools = []LogicalPool{lpool}
	}

	// generate the topology.json
	var bytes []byte
	bytes, err := json.Marshal(topo)
	if err != nil {
		return ""
	}
	clusterPoolJson := string(bytes)
	// for debug
	logger.Info(clusterPoolJson)

	return clusterPoolJson
}
