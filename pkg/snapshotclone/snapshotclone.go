package snapshotclone

import (
	"context"
	"fmt"
	"path"
	"strconv"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
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
	*daemon.Cluster
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "snapshotclone")

func New(c *daemon.Cluster) *Cluster {
	return &Cluster{Cluster: c}
}

// Start Curve snapshotclone daemon
func (c *Cluster) Start(nodesInfo []daemon.NodeInfo) error {
	logger.Info("starting snapshotclone server")

	// get clusterEtcdAddr
	etcdOverrideCM, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(context.Background(), config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.EtcdOverrideConfigMapName)
	}
	clusterEtcdAddr := etcdOverrideCM.Data[config.ClusterEtcdAddr]

	// get clusterMdsAddr
	mdsOverrideCM, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(context.Background(), config.MdsOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get mds override endoints configmap")
	}
	clusterMdsAddr := mdsOverrideCM.Data[config.MdsOvverideConfigMapDataKey]

	if err := c.createStartSnapConfigMap(); err != nil {
		return err
	}

	var deploymentsToWaitFor []*appsv1.Deployment
	for _, node := range nodesInfo {
		daemonIDString := k8sutil.IndexToName(node.HostID)
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)

		snapConfig := &snapConfig{
			Prefix:           Prefix,
			ServiceAddr:      node.NodeIP,
			ServicePort:      strconv.Itoa(node.SnapshotClonePort),
			ServiceDummyPort: strconv.Itoa(node.SnapshotCloneDummyPort),
			ServiceProxyPort: strconv.Itoa(node.SnapshotCloneProxyPort),
			ClusterEtcdAddr:  clusterEtcdAddr,
			ClusterMdsAddr:   clusterMdsAddr,

			DaemonID:             daemonIDString,
			ResourceName:         resourceName,
			CurrentConfigMapName: currentConfigMapName,
			DataPathMap: config.NewDaemonDataPathMap(
				path.Join(c.DataDirHostPath, fmt.Sprint("snapshotclone-", daemonIDString)),
				path.Join(c.LogDirHostPath, fmt.Sprint("snapshotclone-", daemonIDString)),
				ContainerDataDir,
				ContainerLogDir,
			),
		}

		err = c.prepareConfigMap(snapConfig)
		if err != nil {
			return err
		}

		// make snapshotclone deployment
		d, err := c.makeDeployment(node.NodeName, node.NodeIP, snapConfig)
		if err != nil {
			return err
		}

		newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(context.Background(), d, metav1.CreateOptions{})
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create snapshotclone deployment %q in cluster", snapConfig.ResourceName)
			}
			logger.Infof("deployment %v for snapshotclone already exists. updating if needed", snapConfig.ResourceName)

			// TODO:Update the daemon Deployment
			// if err := updateDeploymentAndWait(c.Context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
			// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
			// }
		} else {
			logger.Infof("Deployment %q has been created, waiting for startup", newDeployment.GetName())
			deploymentsToWaitFor = append(deploymentsToWaitFor, newDeployment)
		}
	}

	// wait all Deployments to start
	for _, d := range deploymentsToWaitFor {
		if err := k8sutil.WaitForDeploymentToStart(context.TODO(), &c.Context, d); err != nil {
			return err
		}
	}

	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeSnapShotCloneReady, curvev1.ConditionTrue, curvev1.ConditionSnapShotCloneClusterCreatedReason, "Snapshotclone cluster has been created")

	return nil
}

func (c *Cluster) createStartSnapConfigMap() error {
	startSnapShotConfigMap := map[string]string{
		config.StartSnapConfigMapDataKey: START,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.StartSnapConfigMap,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: startSnapShotConfigMap,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to start_snapshot.sh configmap %q", config.StartSnapConfigMap)
	}
	// create nginx configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create start snapshotclone configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}
