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
	context            clusterd.Context
	NameSpace          string
	NamespacedName     types.NamespacedName
	Spec               *curvev1.CurveClusterSpec
	ownerInfo          *k8sutil.OwnerInfo
	isUpgrade          bool
	observedGeneration int64
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "controllers")

func newCluster(ctx clusterd.Context, c *curvev1.CurveCluster, ownerInfo *k8sutil.OwnerInfo) *cluster {
	return &cluster{
		// at this phase of the cluster creation process, the identity components of the cluster are
		// not yet established. we reserve this struct which is filled in as soon as the cluster's
		// identity can be established.
		context:        ctx,
		NamespacedName: types.NamespacedName{Namespace: c.Namespace, Name: c.Name},
		Spec:           &c.Spec,
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
	nodeNameIP, err := c.getNodeInfoMap()
	if err != nil {
		return errors.Wrap(err, "failed get node with app=etcd label")
	}

	// for debug
	logger.Infof("nodeNameIP: %+v", nodeNameIP)

	// 1. Start etcd cluster
	etcds := etcd.New(c.context, c.NamespacedName, *c.Spec, c.ownerInfo)
	err = etcds.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve etcd")
	}

	// wait to etcd election finished
	time.Sleep(10 * time.Second)

	// 2. Start Mds cluster
	mds := mds.New(c.context, c.NamespacedName, *c.Spec, c.ownerInfo)
	err = mds.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve mds")
	}
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.NamespacedName, curvev1.ConditionTypeMdsReady, curvev1.ConditionTrue, curvev1.ConditionMdsClusterCreatedReason, "MDS cluster has been created")

	// 3. chunkserver
	chunkservers := chunkserver.New(c.context, c.NamespacedName, *c.Spec, c.ownerInfo)
	err = chunkservers.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve chunkserver")
	}
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.NamespacedName, curvev1.ConditionTypeChunkServerReady, curvev1.ConditionTrue, curvev1.ConditionChunkServerClusterCreatedReason, "Chunkserver cluster has been created")

	// // 4. snapshotclone
	if c.Spec.SnapShotClone.Enable {
		snapshotclone := snapshotclone.New(c.context, c.NamespacedName, *c.Spec, c.ownerInfo)
		err = snapshotclone.Start(nodeNameIP)
		if err != nil {
			return errors.Wrap(err, "failed to start curve snapshotclone")
		}
	}
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.NamespacedName, curvev1.ConditionTypeSnapShotCloneReady, curvev1.ConditionTrue, curvev1.ConditionSnapShotCloneClusterCreatedReason, "Snapshotclone cluster has been created")

	return nil
}
