package snapshotclone

import (
	"context"
	"fmt"
	"path"
	"strconv"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	AppName             = "curve-snapshotclone"
	ConfigMapNamePrefix = "curve-snapshotclone-conf"

	// ContainerPath is the mount path of data and log
	Prefix           = "/curvebs/snapshotclone"
	ContainerDataDir = "/curvebs/snapshotclone/data"
	ContainerLogDir  = "/curvebs/snapshotclone/logs"
)

type Cluster struct {
	context         clusterd.Context
	namespacedName  types.NamespacedName
	spec            curvev1.CurveClusterSpec
	dataDirHostPath string
	logDirHostPath  string
	confDirHostPath string
	ownerInfo       *k8sutil.OwnerInfo
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "snapshotclone")

func New(context clusterd.Context,
	namespacedName types.NamespacedName,
	spec curvev1.CurveClusterSpec,
	ownerInfo *k8sutil.OwnerInfo,
	dataDirHostPath string,
	logDirHostPath string,
	confDirHostPath string) *Cluster {
	return &Cluster{
		context:         context,
		namespacedName:  namespacedName,
		spec:            spec,
		dataDirHostPath: dataDirHostPath,
		logDirHostPath:  logDirHostPath,
		confDirHostPath: confDirHostPath,
		ownerInfo:       ownerInfo,
	}
}

// Start Curve snapshotclone daemon
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	logger.Info("starting snapshotclone server")

	// get clusterEtcdAddr
	etcdOverrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.EtcdOverrideConfigMapName)
	}
	clusterEtcdAddr := etcdOverrideCM.Data[config.ClusterEtcdAddr]

	// get clusterMdsAddr
	mdsOverrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.MdsOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get mds override endoints configmap")
	}
	clusterMdsAddr := mdsOverrideCM.Data[config.MdsOvverideConfigMapDataKey]

	err = c.createStartSnapConfigMap()
	if err != nil {
		return errors.Wrap(err, "failed to create start snapshotclone configMap")
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
		logger.Errorf("Nodes spec field is not 3, current nodes number is %d", len(nodeNamesOrdered))
		return errors.New("Nodes spec field is not 3")
	}

	daemonID := 0
	var daemonIDString string
	for _, nodeName := range nodeNamesOrdered {
		daemonIDString = k8sutil.IndexToName(daemonID)
		daemonID++
		// Construct snapclone config to pass to make deployment
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)
		snapConfig := &snapConfig{
			Prefix:           Prefix,
			ServiceAddr:      nodeNameIP[nodeName],
			ServicePort:      strconv.Itoa(c.spec.SnapShotClone.Port),
			ServiceDummyPort: strconv.Itoa(c.spec.SnapShotClone.DummyPort),
			ServiceProxyPort: strconv.Itoa(c.spec.SnapShotClone.ProxyPort),
			ClusterEtcdAddr:  clusterEtcdAddr,
			ClusterMdsAddr:   clusterMdsAddr,

			DaemonID:             daemonIDString,
			ResourceName:         resourceName,
			CurrentConfigMapName: currentConfigMapName,
			DataPathMap: config.NewDaemonDataPathMap(
				path.Join(c.dataDirHostPath, fmt.Sprint("snapshotclone-", daemonIDString)),
				path.Join(c.logDirHostPath, fmt.Sprint("snapshotclone-", daemonIDString)),
				ContainerDataDir,
				ContainerLogDir,
			),
		}

		// for debug
		// log.Infof("current node is %v", nodeName)

		err = c.prepareConfigMap(snapConfig)
		if err != nil {
			return errors.Wrap(err, "failed to prepare all ConfigMaps of snapshotclone")
		}

		// make snapshotclone deployment
		d, err := c.makeDeployment(nodeName, nodeNameIP[nodeName], snapConfig)
		if err != nil {
			return errors.Wrapf(err, "failed to create snapshotclone Deployment %q object", snapConfig.ResourceName)
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create snapshotclone deployment %q in cluster", snapConfig.ResourceName)
			}
			logger.Infof("deployment %v for snapshotclone already exists. updating if needed", snapConfig.ResourceName)

			// TODO:Update the daemon Deployment
			// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
			// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
			// }
		} else {
			logger.Infof("Deployment %q has been created, waiting for startup", newDeployment.GetName())
			// TODO:wait for the new deployment
			// deploymentsToWaitFor = append(deploymentsToWaitFor, newDeployment)
		}
		// update condition type and phase etc.
	}

	k8sutil.UpdateCondition(context.TODO(), &c.context, c.namespacedName, curvev1.ConditionTypeSnapShotCloneReady, curvev1.ConditionTrue, curvev1.ConditionSnapShotCloneClusterCreatedReason, "Snapshotclone cluster has been created")

	return nil
}

func (c *Cluster) createStartSnapConfigMap() error {
	startSnapShotConfigMap := map[string]string{
		config.StartSnapConfigMapDataKey: START,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.StartSnapConfigMap,
			Namespace: c.namespacedName.Namespace,
		},
		Data: startSnapShotConfigMap,
	}

	err := c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to start_snapshot.sh configmap %q", config.StartSnapConfigMap)
	}
	// create nginx configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create start snapshotclone configmap %s", c.namespacedName.Namespace)
	}
	return nil
}
