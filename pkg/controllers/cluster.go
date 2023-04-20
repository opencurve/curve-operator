package controllers

import (
	"context"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/chunkserver"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/mds"
	"github.com/opencurve/curve-operator/pkg/snapshotclone"
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "controller")

func newCluster(kind string, isUpgrade bool) *daemon.Cluster {
	return &daemon.Cluster{
		Kind:      kind,
		IsUpgrade: isUpgrade,
	}
}

// reconcileCurveDaemons start all daemon progress of Curve
func reconcileCurveDaemons(c *daemon.Cluster) error {
	// get node name and internal ip mapping
	nodeNameIP, err := k8sutil.GetNodeInfoMap(c.Nodes, c.Context.Clientset)
	if err != nil {
		return errors.Wrap(err, "failed get all nodes specified in spec nodes")
	}
	logger.Infof("using %v to create curve cluster", nodeNameIP)

	// 1. Create a pod to get all config file from curve image
	job, err := makeReadConfJob(c)
	if err != nil {
		return errors.Wrap(err, "failed to start job to read all config file from curve image")
	}
	logger.Info("starting read config file template job")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	chn := make(chan bool, 1)
	ctx, canf := context.WithTimeout(context.Background(), 10*60*time.Second)
	defer canf()
	k8sutil.CheckJobStatus(ctx, c.Context.Clientset, ticker, chn, c.Namespace, job.Name)
	flag := <-chn
	if !flag {
		return errors.Errorf("failed to check job %q status", job.GetName())
	}

	// 2. Create ConfigMaps for all configs
	err = createEachConfigMap(c)
	if err != nil {
		return errors.Wrap(err, "failed to create all config file template configmap")
	}
	logger.Info("create config template configmap successfully")

	// 2. Start etcd cluster
	etcds := etcd.New(c)
	err = etcds.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve etcd")
	}

	// wait to etcd election finished
	time.Sleep(20 * time.Second)

	// 3. Start Mds cluster
	mds := mds.New(c)
	err = mds.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve mds")
	}
	k8sutil.UpdateCondition(context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeMdsReady, curvev1.ConditionTrue, curvev1.ConditionMdsClusterCreatedReason, "MDS cluster has been created")

	// 4. chunkserver
	chunkservers := chunkserver.New(c)
	err = chunkservers.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve chunkserver")
	}
	k8sutil.UpdateCondition(context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeChunkServerReady, curvev1.ConditionTrue, curvev1.ConditionChunkServerClusterCreatedReason, "Chunkserver cluster has been created")

	// 5. snapshotclone
	if c.SnapShotClone.Enable {
		snapshotclone := snapshotclone.New(c)
		err = snapshotclone.Start(nodeNameIP)
		if err != nil {
			return errors.Wrap(err, "failed to start curve snapshotclone")
		}
	}
	k8sutil.UpdateCondition(context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeSnapShotCloneReady, curvev1.ConditionTrue, curvev1.ConditionSnapShotCloneClusterCreatedReason, "Snapshotclone cluster has been created")

	return nil
}
