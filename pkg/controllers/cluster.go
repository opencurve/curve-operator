package controllers

import (
	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
)

// cluster represent a instance of Curve Cluster
type cluster struct {
	context            clusterd.Context
	NameSpace          string
	NamespacedName     types.NamespacedName
	Spec               *curvev1.CurveClusterSpec
	isUpgrade          bool
	observedGeneration int64
}

func newCluster(ctx clusterd.Context, c *curvev1.CurveCluster) *cluster {
	return &cluster{
		// at this phase of the cluster creation process, the identity components of the cluster are
		// not yet established. we reserve this struct which is filled in as soon as the cluster's
		// identity can be established.
		context:        ctx,
		NamespacedName: types.NamespacedName{Namespace: c.Namespace, Name: c.Name},
		Spec:           &c.Spec,
		isUpgrade:      false,
		// update observedGeneration with current generation value,
		// because generation can be changed before reconcile got completed
		// CR status will be updated at end of reconcile, so to reflect the reconcile has finished
		observedGeneration: c.ObjectMeta.Generation,
	}
}

// reconcileCurveDaemons start all daemon progress of curve
func (c *cluster) reconcileCurveDaemons() error {
	// 1. Start Etcd cluster
	etcds := etcd.New(c.context, c.NamespacedName, *c.Spec)
	err := etcds.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start curve etcd")
	}
	// 2. Start Mds cluster
	// 3. Start ChunkServer cluster
	return nil
}
