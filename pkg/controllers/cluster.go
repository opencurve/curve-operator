package controllers

import (
	"context"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/chunkserver"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/mds"
	"github.com/opencurve/curve-operator/pkg/snapshotclone"
)

// cluster represent a instance of Curve Cluster
type cluster struct {
	context        clusterd.Context
	NameSpace      string
	NamespacedName types.NamespacedName
	Spec           *curvev1.CurveClusterSpec
	// hostpath
	dataDirHostPath    string
	logDirHostPath     string
	confDirHostPath    string
	ownerInfo          *k8sutil.OwnerInfo
	isUpgrade          bool
	observedGeneration int64
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "controller")

func newCluster(ctx clusterd.Context, c *curvev1.CurveCluster, ownerInfo *k8sutil.OwnerInfo) *cluster {
	return &cluster{
		// at this phase of the cluster creation process, the identity components of the cluster are
		// not yet established. we reserve this struct which is filled in as soon as the cluster's
		// identity can be established.
		context:        ctx,
		NamespacedName: types.NamespacedName{Namespace: c.Namespace, Name: c.Name},
		Spec:           c.Spec,
		ownerInfo:      ownerInfo,
		isUpgrade:      false,
		// update observedGeneration with current generation value,
		// because generation can be changed before reconcile got completed
		// CR status will be updated at end of reconcile, so to reflect the reconcile has finished
		observedGeneration: c.ObjectMeta.Generation,
	}
}

// reconcileCurveDaemons start all daemon progress of Curve
func (c *cluster) reconcileCurveDaemons() error {
	// get node name and internal ip mapping
	nodeNameIP, err := k8sutil.GetNodeInfoMap(c.Spec, c.context.Clientset)
	if err != nil {
		return errors.Wrap(err, "failed get all nodes specified in spec nodes")
	}
	logger.Infof("using %v to create curve cluster", nodeNameIP)

	// 1. Create a pod to get all config file from curve image
	job, err := c.makeReadConfJob()
	if err != nil {
		return errors.Wrap(err, "failed to start job to read all config file from curve image")
	}
	logger.Info("starting read config file template job")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	chn := make(chan bool, 1)
	ctx, canf := context.WithTimeout(context.Background(), time.Duration(10*60*time.Second))
	defer canf()
	k8sutil.CheckJobStatus(ctx, c.context.Clientset, ticker, chn, c.NameSpace, job.Name)
	flag := <-chn
	if !flag {
		return errors.Errorf("failed to check job %q status", job.GetName())
	}

	// 2. Create ConfigMaps for all configs
	err = c.createEachConfigMap()
	if err != nil {
		return errors.Wrap(err, "failed to create all config file template configmap")
	}
	logger.Info("create config template configmap successed")

	// 2. Start etcd cluster
	etcds := etcd.New(c.context, c.NamespacedName, *c.Spec, c.ownerInfo, c.dataDirHostPath, c.logDirHostPath, c.confDirHostPath)
	err = etcds.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve etcd")
	}

	// wait to etcd election finished
	time.Sleep(20 * time.Second)

	// 3. Start Mds cluster
	mds := mds.New(c.context, c.NamespacedName, *c.Spec, c.ownerInfo, c.dataDirHostPath, c.logDirHostPath, c.confDirHostPath)
	err = mds.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve mds")
	}
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.NamespacedName, curvev1.ConditionTypeMdsReady, curvev1.ConditionTrue, curvev1.ConditionMdsClusterCreatedReason, "MDS cluster has been created")

	// 4. chunkserver
	chunkservers := chunkserver.New(c.context, c.NamespacedName, *c.Spec, c.ownerInfo, c.dataDirHostPath, c.logDirHostPath, c.confDirHostPath)
	err = chunkservers.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve chunkserver")
	}
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.NamespacedName, curvev1.ConditionTypeChunkServerReady, curvev1.ConditionTrue, curvev1.ConditionChunkServerClusterCreatedReason, "Chunkserver cluster has been created")

	// 5. snapshotclone
	if c.Spec.SnapShotClone.Enable {
		snapshotclone := snapshotclone.New(c.context, c.NamespacedName, *c.Spec, c.ownerInfo, c.dataDirHostPath, c.logDirHostPath, c.confDirHostPath)
		err = snapshotclone.Start(nodeNameIP)
		if err != nil {
			return errors.Wrap(err, "failed to start curve snapshotclone")
		}
	}
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.NamespacedName, curvev1.ConditionTypeSnapShotCloneReady, curvev1.ConditionTrue, curvev1.ConditionSnapShotCloneClusterCreatedReason, "Snapshotclone cluster has been created")

	return nil
}
