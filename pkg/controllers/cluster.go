package controllers

import (
	"time"

	"github.com/coreos/pkg/capnslog"

	"github.com/opencurve/curve-operator/pkg/chunkserver"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/opencurve/curve-operator/pkg/mds"
	"github.com/opencurve/curve-operator/pkg/metaserver"
	"github.com/opencurve/curve-operator/pkg/snapshotclone"
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "controller")

func newCluster(kind string, isUpgrade bool) *daemon.Cluster {
	return &daemon.Cluster{
		Kind:      kind,
		IsUpgrade: isUpgrade,
	}
}

func reconcileSharedServer(c *daemon.Cluster) ([]daemon.NodeInfo, error) {
	// get node name and internal ip mapping
	nodesInfo, err := daemon.ConfigureNodeInfo(c)
	if err != nil {
		return nil, err
	}

	err = createSyncDeployment(c)
	if err != nil {
		return nil, err
	}
	time.Sleep(20 * time.Second)

	err = createConfigMapTemplate(c)
	if err != nil {
		return nil, err
	}
	logger.Info("create config template configmap successfully")

	// Start etcd cluster
	etcds := etcd.New(c)
	err = etcds.Start(nodesInfo)
	if err != nil {
		return nil, err
	}
	// wait until etcd election finished
	time.Sleep(20 * time.Second)

	// Start Mds cluster
	mds := mds.New(c)
	err = mds.Start(nodesInfo)
	if err != nil {
		return nil, err
	}
	// wait until mds election finished
	time.Sleep(20 * time.Second)

	return nodesInfo, nil
}

// reconcileCurveDaemons start all daemon progress of Curve
func reconcileCurveDaemons(c *daemon.Cluster) error {
	// shared server
	nodesInfo, err := reconcileSharedServer(c)
	if err != nil {
		return err
	}
	// chunkserver
	chunkservers := chunkserver.New(c)
	err = chunkservers.Start(nodesInfo)
	if err != nil {
		return err
	}

	// snapshotclone
	if c.SnapShotClone.Enable {
		snapshotclone := snapshotclone.New(c)
		err = snapshotclone.Start(nodesInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

// reconcileCurveDaemons start all daemon progress of Curve
func reconcileCurveFSDaemons(c *daemon.Cluster) error {
	// shared server
	nodesInfo, err := reconcileSharedServer(c)
	if err != nil {
		return err
	}

	// metaserver
	metaservers := metaserver.New(c)
	err = metaservers.Start(nodesInfo)
	if err != nil {
		return err
	}

	return nil
}
