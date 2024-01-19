package service

import (
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

const (
	CURVE_TOPOLOGY_CONFIGMAP = "curve-cluster-pool"
	TOPO_JSON_FILE_NAME      = "topology.json"
)

const (
	KIND_CURVEBS     = topology.KIND_CURVEBS
	KIND_CURVEFS     = topology.KIND_CURVEFS
	ROLE_CHUNKSERVER = topology.ROLE_CHUNKSERVER
	ROLE_METASERVER  = topology.ROLE_METASERVER

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

// prepare get cluster pool or create new cluster pool
func createOrUpdatePoolConfigMap(cluster clusterd.Clusterer, dcs []*topology.DeployConfig) error {
	clusterPool, err := getClusterPool(cluster, dcs)
	if err != nil {
		return err
	}

	var bytes []byte
	bytes, err = json.Marshal(clusterPool)
	if err != nil {
		return err
	}
	clusterPoolJson := string(bytes)
	data := map[string]string{
		TOPO_JSON_FILE_NAME: clusterPoolJson,
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CURVE_TOPOLOGY_CONFIGMAP,
			Namespace: cluster.GetNameSpace(),
		},
		Data: data,
	}

	_, err = k8sutil.CreateOrUpdateConfigMap(cluster.GetContext().Clientset, cm)
	if err != nil {
		return nil
	}

	return nil
}

func getClusterPool(cluster clusterd.Clusterer, dcs []*topology.DeployConfig) (CurveClusterTopo, error) {
	cm, err := k8sutil.GetConfigMapByName(cluster.GetContext().Clientset, cluster.GetNameSpace(), CURVE_TOPOLOGY_CONFIGMAP)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return generateDefaultClusterPool(dcs)
		}
		return CurveClusterTopo{}, err
	}

	oldPool := CurveClusterTopo{}
	oldPoolStrData := cm.Data[TOPO_JSON_FILE_NAME]
	err = json.Unmarshal([]byte(oldPoolStrData), &oldPool)
	pool, err := generateDefaultClusterPool(dcs)
	if err != nil {
		return pool, err
	}

	// gurantee oldPool and pool has same servers
	for i, server := range pool.Servers {
		oldPool.Servers[i].InternalIp = server.InternalIp
		oldPool.Servers[i].InternalPort = server.InternalPort
		oldPool.Servers[i].ExternalIp = server.ExternalIp
		oldPool.Servers[i].ExternalPort = server.ExternalPort
	}
	if dcs[0].GetKind() == topology.KIND_CURVEBS {
		for i, pool := range pool.LogicalPools {
			oldPool.LogicalPools[i].Copysets = pool.Copysets
		}
	}

	return oldPool, nil
}

func generateDefaultClusterPool(dcs []*topology.DeployConfig) (topo CurveClusterTopo, err error) {
	topo = generateClusterPool(dcs, "pool1")
	return
}

func generateClusterPool(dcs []*topology.DeployConfig, poolName string) CurveClusterTopo {
	lpool, servers := createLogicalPool(dcs, poolName)
	topo := CurveClusterTopo{Servers: servers, NPools: 1}
	if dcs[0].GetKind() == KIND_CURVEBS {
		topo.LogicalPools = []LogicalPool{lpool}
	} else {
		topo.Pools = []LogicalPool{lpool}
	}
	return topo
}

func createLogicalPool(dcs []*topology.DeployConfig, logicalPool string) (LogicalPool, []Server) {
	var zone string
	copysets := 0
	servers := []Server{}
	zones := DEFAULT_ZONES_PER_POOL
	nextZone := genNextZone(zones)
	physicalPool := logicalPool
	kind := dcs[0].GetKind()
	SortDeployConfigs(dcs)
	for _, dc := range dcs {
		role := dc.GetRole()
		if (role == ROLE_CHUNKSERVER && kind == KIND_CURVEBS) ||
			(role == ROLE_METASERVER && kind == KIND_CURVEFS) {
			if dc.GetParentId() == dc.GetId() {
				zone = nextZone()
			}

			// NOTE: if we deploy chunkservers with instance feature
			// and the value of instance greater than 1, we should
			// set internal port and external port to 0 for let MDS
			// attribute them as services on the same machine.
			// see issue: https://github.com/opencurve/curve/issues/1441
			internalPort := dc.GetListenPort()
			externalPort := dc.GetListenExternalPort()
			if dc.GetInstances() > 1 {
				internalPort = 0
				externalPort = 0
			}

			server := Server{
				Name:         formatName(dc),
				InternalIp:   dc.GetHostIp(),
				InternalPort: internalPort,
				ExternalIp:   dc.GetListenExternalIp(),
				ExternalPort: externalPort,
				Zone:         zone,
			}
			if kind == KIND_CURVEBS {
				server.PhysicalPool = physicalPool
			} else {
				server.Pool = logicalPool
			}
			copysets += dc.GetCopysets()
			servers = append(servers, server)
		}
	}

	// copysets
	copysets = (int)(copysets / DEFAULT_REPLICAS_PER_COPYSET)
	if copysets == 0 {
		copysets = 1
	}

	// logical pool
	lpool := LogicalPool{
		Name:     logicalPool,
		Copysets: copysets,
		Zones:    zones,
		Replicas: DEFAULT_REPLICAS_PER_COPYSET,
	}
	if kind == KIND_CURVEBS {
		lpool.ScatterWidth = DEFAULT_SCATTER_WIDTH
		lpool.Type = DEFAULT_TYPE
		lpool.PhysicalPool = physicalPool
	}

	return lpool, servers
}

func genNextZone(zones int) func() string {
	idx := 0
	return func() string {
		idx++
		return fmt.Sprintf("zone%d", (idx-1)%zones+1)
	}
}

// we should sort the "dcs" for generate correct zone number
func SortDeployConfigs(dcs []*topology.DeployConfig) {
	sort.Slice(dcs, func(i, j int) bool {
		dc1, dc2 := dcs[i], dcs[j]
		if dc1.GetRole() == dc2.GetRole() {
			if dc1.GetHostSequence() == dc2.GetHostSequence() {
				return dc1.GetInstancesSequence() < dc2.GetInstancesSequence()
			}
			return dc1.GetHostSequence() < dc2.GetHostSequence()
		}
		return dc1.GetRole() < dc2.GetRole()
	})
}

func formatName(dc *topology.DeployConfig) string {
	return fmt.Sprintf("%s_%s_%d", dc.GetHost(), dc.GetName(), dc.GetInstancesSequence())
}
