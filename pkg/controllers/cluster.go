package controllers

import (
	"time"

	"github.com/coreos/pkg/capnslog"

	"github.com/opencurve/curve-operator/pkg/chunkserver"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/opencurve/curve-operator/pkg/mds"
	"github.com/opencurve/curve-operator/pkg/metaserver"
	"github.com/opencurve/curve-operator/pkg/monitor"
	"github.com/opencurve/curve-operator/pkg/snapshotclone"
	"github.com/opencurve/curve-operator/pkg/topology"
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "controller")

func newCluster(uuid, kind string, isUpgrade bool) *daemon.Cluster {
	return &daemon.Cluster{
		UUID:      uuid,
		Kind:      kind,
		IsUpgrade: isUpgrade,
	}
}

func reconcileSharedServer(c *daemon.Cluster) ([]daemon.NodeInfo, []*topology.DeployConfig, error) {
	// get node name and internal ip mapping
	nodesInfo, err := daemon.ConfigureNodeInfo(c)
	if err != nil {
		return nil, nil, err
	}

	err = createSyncDeployment(c)
	if err != nil {
		return nil, nil, err
	}
	time.Sleep(20 * time.Second)

	err = createDefaultConfigMap(c)
	if err != nil {
		return nil, nil, err
	}

	if c.Monitor.Enable {
		err = createGrafanaConfigMapTemplate(c)
		if err != nil {
			return nil, nil, err
		}
	}

	logger.Info("create config template configmap successfully")

	if c.EnableReport {
		err = createReportConfigMap(c)
		if err != nil {
			return nil, nil, err
		}
	}

	// Start etcd cluster
	etcds := etcd.New(c)
	dcs, err := etcds.Start(nodesInfo)
	if err != nil {
		return nil, nil, err
	}
	// wait until etcd election finished
	time.Sleep(20 * time.Second)

	// Start Mds cluster
	mds := mds.New(c)
	dcs, err = mds.Start(nodesInfo, dcs)
	if err != nil {
		return nil, nil, err
	}
	// wait until mds election finished
	time.Sleep(20 * time.Second)

	return nodesInfo, dcs, nil
}

// reconcileCurveDaemons start all daemon progress of Curve
func reconcileCurveDaemons(c *daemon.Cluster) error {
	// shared server
	nodesInfo, dcs, err := reconcileSharedServer(c)
	if err != nil {
		return err
	}
	// chunkserver
	chunkservers := chunkserver.New(c)
	dcs, err = chunkservers.Start(nodesInfo, dcs)
	if err != nil {
		return err
	}

	// snapshotclone
	if c.SnapShotClone.Enable {
		snapshotclone := snapshotclone.New(c)
		dcs, err = snapshotclone.Start(nodesInfo, dcs)
		if err != nil {
			return err
		}
	}

	if c.Monitor.Enable {
		monitor := monitor.New(c)
		err = monitor.Start(nodesInfo, dcs)
		if err != nil {
			return err
		}
	}

	if c.EnableReport {
		// report cluster
		err = runReportCronJob(c, c.SnapShotClone.Enable)
		if err != nil {
			return err
		}
	}

	return nil
}

// reconcileCurveDaemons start all daemon progress of Curve
func reconcileCurveFSDaemons(c *daemon.Cluster) error {
	// shared server
	nodesInfo, dcs, err := reconcileSharedServer(c)
	if err != nil {
		return err
	}

	// metaserver
	metaservers := metaserver.New(c)
	dcs, err = metaservers.Start(nodesInfo, dcs)
	if err != nil {
		return err
	}

	if c.Monitor.Enable {
		monitor := monitor.New(c)
		err = monitor.Start(nodesInfo, dcs)
		if err != nil {
			return err
		}
	}

	if c.EnableReport {
		// report cluster
		err = runReportCronJob(c, c.SnapShotClone.Enable)
		if err != nil {
			return err
		}
	}

	return nil
}
