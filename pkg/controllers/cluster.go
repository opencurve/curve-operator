package controllers

import (
	"time"

	"github.com/coreos/pkg/capnslog"
	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/chunkserver"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/opencurve/curve-operator/pkg/mds"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "etcd")

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
	// get node name and internal ip map
	nodeNameIP, err := c.getNodeInfoMap()
	if err != nil {
		return errors.Wrap(err, "failed get node with app=etcd label")
	}

	// 1. Start Etcd cluster
	etcds := etcd.New(c.context, c.NamespacedName, *c.Spec)
	err = etcds.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve etcd")
	}

	time.Sleep(10 * time.Second)

	// 2. Start Mds cluster
	mds := mds.New(c.context, c.NamespacedName, *c.Spec)
	err = mds.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve mds")
	}

	// 3. chunkserver
	chunkservers := chunkserver.New(c.context, c.NamespacedName, *c.Spec)
	err = chunkservers.Start(nodeNameIP)
	if err != nil {
		return errors.Wrap(err, "failed to start curve chunkserver")
	}

	return nil
}

// getNodeInfoMap get node info for label "app=etcd"
func (c *cluster) getNodeInfoMap() (map[string]string, error) {
	nodes, err := c.context.Clientset.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: "app=etcd",
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list all nodes")
	}
	if len(nodes.Items) != 3 {
		logger.Errorf("node count must be set 3 %v", err)
		return nil, errors.Wrapf(err, "failed to list all nodes, must have 3 node, obly has %d nodes in cluster!!!", len(nodes.Items))
	}

	// Map node name and node InternalIP
	nodeNameIP := make(map[string]string)

	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == "InternalIP" {
				nodeNameIP[node.Name] = address.Address
			}
		}
	}
	return nodeNameIP, nil
}
