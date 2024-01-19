package controllers

import (
	"github.com/coreos/pkg/capnslog"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/service"
	"github.com/opencurve/curve-operator/pkg/topology"
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "controller")

const (
	REGEX_KV_SPLIT = "^(([^%s]+)%s\\s*)([^\\s#]*)"
)

func newFsClusterManager(uuid, kind string) *clusterd.FsClusterManager {
	return &clusterd.FsClusterManager{
		UUID: uuid,
		Kind: kind,
	}
}

func newBsClusterManager(uuid, kind string) *clusterd.BsClusterManager {
	return &clusterd.BsClusterManager{
		UUID: uuid,
		Kind: kind,
	}
}

// initCluster initialize a new cluster
func initCluster(cluster clusterd.Clusterer, dcs []*topology.DeployConfig) error {
	err := preClusterStartValidation(cluster)
	if err != nil {
		return err
	}

	err = reconcileCluster(cluster, dcs)
	if err != nil {
		return err
	}

	return nil
}

// preClusterStartValidation cluster Spec validation
func preClusterStartValidation(cluster clusterd.Clusterer) error {
	return nil
}

func reconcileCluster(cluster clusterd.Clusterer, dcs []*topology.DeployConfig) error {
	if err := constructConfigMap(cluster, dcs); err != nil {
		return err
	}
	if err := reconcileCurveDaemons(cluster, dcs); err != nil {
		return err
	}
	return nil
}

// reconcileCurveDaemons start all daemon progress of Curve of specified type
func reconcileCurveDaemons(cluster clusterd.Clusterer, dcs []*topology.DeployConfig) error {
	for _, dc := range dcs {
		serviceConfigs := dc.GetProjectLayout().ServiceConfFiles
		for _, conf := range serviceConfigs {
			err := mutateConfig(cluster, dc, conf.Name)
			if err != nil {
				return err
			}
		}
		// mutate tools.conf in configmp
		if err := mutateConfig(cluster, dc, topology.LAYOUT_TOOLS_NAME); err != nil {
			return err
		}

		// start specified service
		if err := service.StartService(cluster, dc); err != nil {
			return err
		}

		if dc.GetKind() == topology.KIND_CURVEBS && dc.GetRole() == topology.ROLE_MDS {
			// 创建物理池
			if err := service.StartJobCreatePool(cluster, dc, dcs, service.POOL_TYPE_PHYSICAL); err != nil {
				return err
			}
		} else if dc.GetKind() == topology.KIND_CURVEBS && dc.GetRole() == topology.ROLE_CHUNKSERVER {
			// 创建逻辑池
			if err := service.StartJobCreatePool(cluster, dc, dcs, service.POOL_TYPE_LOGICAL); err != nil {
				return err
			}
		} else if dc.GetKind() == topology.KIND_CURVEFS && dc.GetRole() == topology.ROLE_MDS {
			// 创建逻辑池
			if err := service.StartJobCreatePool(cluster, dc, dcs, service.POOL_TYPE_LOGICAL); err != nil {
				return err
			}
		}
	}

	return nil
}

// // reconcileCurveDaemons start all daemon progress of Curve
// func reconcileCurveFSDaemons(c *daemon.Cluster) error {
// 	// metaserver
// 	metaservers := metaserver.New(c)
// 	dcs, err = metaservers.Start(nodesInfo, dcs)
// 	if err != nil {
// 		return err
// 	}

// 	if c.Monitor.Enable {
// 		monitor := monitor.New(c)
// 		err = monitor.Start(nodesInfo, dcs)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	// report cluster
// 	err = runReportCronJob(c, c.SnapShotClone.Enable)
// 	if err != nil {
// 		return err
// 	}

// 	// clean up the cluster install environment
// 	err = cleanClusterInstallEnv(c)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
