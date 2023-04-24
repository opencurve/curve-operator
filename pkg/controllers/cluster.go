package controllers

import (
	"context"
	"time"

	"github.com/coreos/pkg/capnslog"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/chunkserver"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
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

func reconcileSharedServer(c *daemon.Cluster) (map[string]string, error) {
	// get node name and internal ip mapping
	nodeNameIP, err := k8sutil.GetNodeInfoMap(c.Nodes, c.Context.Clientset)
	if err != nil {
		return nil, err
	}
	logger.Infof("using %v to create curve cluster", nodeNameIP)

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
	err = etcds.Start(nodeNameIP)
	if err != nil {
		return nil, err
	}
	// wait until etcd election finished
	time.Sleep(20 * time.Second)

	// Start Mds cluster
	mds := mds.New(c)
	err = mds.Start(nodeNameIP)
	if err != nil {
		return nil, err
	}
	// wait until mds election finished
	time.Sleep(20 * time.Second)

	return nodeNameIP, nil
}

// reconcileCurveDaemons start all daemon progress of Curve
func reconcileCurveDaemons(c *daemon.Cluster) error {
	// shared server
	nodeNameIP, err := reconcileSharedServer(c)
	if err != nil {
		return err
	}
	// chunkserver
	chunkservers := chunkserver.New(c)
	err = chunkservers.Start(nodeNameIP)
	if err != nil {
		return err
	}
	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeChunkServerReady, curvev1.ConditionTrue, curvev1.ConditionChunkServerClusterCreatedReason, "Chunkserver cluster has been created")

	// snapshotclone
	if c.SnapShotClone.Enable {
		snapshotclone := snapshotclone.New(c)
		err = snapshotclone.Start(nodeNameIP)
		if err != nil {
			return err
		}
	}
	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeSnapShotCloneReady, curvev1.ConditionTrue, curvev1.ConditionSnapShotCloneClusterCreatedReason, "Snapshotclone cluster has been created")

	return nil
}

// reconcileCurveDaemons start all daemon progress of Curve
func reconcileCurveFSDaemons(c *daemon.Cluster) error {
	// shared server
	nodeNameIP, err := reconcileSharedServer(c)
	if err != nil {
		return err
	}

	// metaserver
	metaservers := metaserver.New(c)
	err = metaservers.Start(nodeNameIP)
	if err != nil {
		return err
	}

	return nil
}
