package snapshotclone

import (
	"context"
	"fmt"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	AppName = "curve-snapshotclone"

	// ContainerPath is the mount path of data and log
	ContainerDataDir = "/curvebs/snapshotclone/data"
	ContainerLogDir  = "/curvebs/snapshotclone/logs"
)

type Cluster struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	spec           curvev1.CurveClusterSpec
	ownerInfo      *k8sutil.OwnerInfo
}

var log = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "snapshotclone")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec, ownerInfo *k8sutil.OwnerInfo) *Cluster {
	return &Cluster{
		context:        context,
		namespacedName: namespacedName,
		spec:           spec,
		ownerInfo:      ownerInfo,
	}
}

// Start Curve snapshotclone daemon
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	log.Info("starting snapshotclone server")

	c.prepareConfigMap()

	var snapEndpoints string
	for _, ipAddr := range nodeNameIP {
		snapEndpoints = fmt.Sprint(snapEndpoints, "server ", ipAddr, ":", c.spec.SnapShotClone.Port, "; ")
	}
	err := c.createNginxConfigMap(snapEndpoints)
	if err != nil {
		log.Error("failed to create nginx.conf configMap")
	}

	err = c.createStartSnapConfigMap()
	if err != nil {
		log.Error("failed to create start snapshotclone configMap")
	}

	// reorder the nodeNameIP according to the order of nodes spec defined by the user
	// nodes:
	// - node1 - curve-snap-a
	// - node2  - curve-snap-b
	// - node3 - curve-snap-c
	nodeNamesOrdered := make([]string, 0)
	for _, n := range c.spec.Nodes {
		for nodeName := range nodeNameIP {
			if n == nodeName {
				nodeNamesOrdered = append(nodeNamesOrdered, nodeName)
			}
		}
	}

	if len(nodeNamesOrdered) != 3 {
		log.Errorf("Nodes spec field is not 3, current nodes number is %d", len(nodeNamesOrdered))
		return errors.New("Nodes spec field is not 3")
	}

	daemonID := 0
	var daemonIDString string
	for _, nodeName := range nodeNamesOrdered {
		daemonIDString = k8sutil.IndexToName(daemonID)
		daemonID++
		// Construct snapclone config to pass to make deployment
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		snapConfig := &snapConfig{
			DaemonID:     daemonIDString,
			ResourceName: resourceName,
			DataPathMap: config.NewDaemonDataPathMap(
				fmt.Sprint(c.spec.DataDirHostPath, "/snapshotclone-", daemonIDString),
				fmt.Sprint(c.spec.LogDirHostPath, "/snapshotclone-", daemonIDString),
				ContainerDataDir,
				ContainerLogDir,
			),
		}

		// for debug
		// log.Infof("current node is %v", nodeName)

		// make snapshotclone deployment
		d, err := c.makeDeployment(nodeName, nodeNameIP[nodeName], snapConfig)
		if err != nil {
			return errors.Wrap(err, "failed to create snapshotclone Deployment")
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create snapshotclone deployment %s", resourceName)
			}
			log.Infof("deployment for snapshotclone %s already exists. updating if needed", resourceName)

			// TODO:Update the daemon Deployment
			// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
			// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
			// }
		} else {
			log.Infof("Deployment %s has been created , waiting for startup", newDeployment.GetName())
			// TODO:wait for the new deployment
			// deploymentsToWaitFor = append(deploymentsToWaitFor, newDeployment)
		}
		// update condition type and phase etc.
	}

	k8sutil.UpdateCondition(context.TODO(), &c.context, c.namespacedName, curvev1.ConditionTypeSnapShotCloneReady, curvev1.ConditionTrue, curvev1.ConditionSnapShotCloneClusterCreatedReason, "Snapshotclone cluster has been created")

	return nil
}

// prepareConfigMap
func (c *Cluster) prepareConfigMap() error {
	// get etcdEndpoint to create snapshotclone configMap
	etcdOverrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get %s configmap from cluster", config.EtcdOverrideConfigMapName)
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.EtcdOverrideConfigMapName)
	}
	etcdEndpoints := etcdOverrideCM.Data[config.EtcdOvverideConfigMapDataKey]

	// get mdsEndpoints and create snap_client configmap
	mdsOverrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.MdsOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get mds override endoints configmap")
	}
	mdsEndpoints := mdsOverrideCM.Data[config.MdsOvverideConfigMapDataKey]
	c.createSnapClientConfigMap(mdsEndpoints)

	log.Infof("created ConfigMap '%s' success", config.SnapClientConfigMapName)

	// get s3 configmap
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.S3ConfigMapName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get %s configmap from cluster", config.S3ConfigMapName)
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.S3ConfigMapName)
	}

	// create snapshotclone.conf configmap
	err = c.createSnapShotCloneConfigMap(etcdEndpoints)
	if err != nil {
		log.Errorf("failed to create %s configmap from cluster", config.SnapShotCloneConfigMapName)
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.SnapShotCloneConfigMapName)
	}

	log.Infof("created ConfigMap '%s' success", config.SnapShotCloneConfigMapName)

	// create nginx.conf configmap
	// err = c.createNginxConfigMap()
	// if err != nil {
	// 	log.Errorf("failed to create %s configmap from cluster", config.NginxConfigMapName)
	// 	return errors.Wrapf(err, "failed to get %s configmap from cluster", config.NginxConfigMapName)
	// }
	return nil
}
