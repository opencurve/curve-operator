package mds

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	AppName             = "curve-mds"
	ConfigMapNamePrefix = "curve-mds-conf"

	// Prefix is the mount path of data and log
	Prefix           = "/curvebs/mds"
	ContainerDataDir = "/curvebs/mds/data"
	ContainerLogDir  = "/curvebs/mds/logs"

	FSPrefix           = "/curvefs/mds"
	FSContainerDataDir = "/curvefs/mds/data"
	FSContainerLogDir  = "/curvefs/mds/logs"
)

type Cluster struct {
	*daemon.Cluster
}

func New(c *daemon.Cluster) *Cluster {
	return &Cluster{Cluster: c}
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "mds")

// Start Curve mds daemon
func (c *Cluster) Start(nodesInfo []daemon.NodeInfo) error {
	overrideCM, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(context.Background(), config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterEtcdAddr := overrideCM.Data[config.ClusterEtcdAddr]

	var mdsEndpoints, clusterMdsDummyAddr, clusterMdsDummyPort string
	for _, node := range nodesInfo {
		mdsEndpoints = fmt.Sprint(mdsEndpoints, node.NodeIP, ":", node.MdsPort, ",")
		clusterMdsDummyAddr = fmt.Sprint(clusterMdsDummyAddr, node.NodeIP, ":", node.DummyPort, ",")
		clusterMdsDummyPort = fmt.Sprint(clusterMdsDummyPort, node.DummyPort, ",")
	}
	mdsEndpoints = strings.TrimRight(mdsEndpoints, ",")
	clusterMdsDummyAddr = strings.TrimRight(clusterMdsDummyAddr, ",")
	clusterMdsDummyPort = strings.TrimRight(clusterMdsDummyPort, ",")

	// create mds override configmap to record mds endpoints
	err = c.createOverrideMdsCM(mdsEndpoints, clusterMdsDummyAddr, clusterMdsDummyPort)
	if err != nil {
		return err
	}

	var configMapMountPath, prefix, containerDataDir, containerLogDir string
	if c.Kind == config.KIND_CURVEBS {
		prefix = Prefix
		containerDataDir = ContainerDataDir
		containerLogDir = ContainerLogDir
		configMapMountPath = config.MdsConfigMapMountPathDir
	} else {
		prefix = FSPrefix
		containerDataDir = FSContainerDataDir
		containerLogDir = FSContainerLogDir
		configMapMountPath = config.FSMdsConfigMapMountPathDir
	}

	var deploymentsToWaitFor []*appsv1.Deployment

	var daemonIDString string
	for _, node := range nodesInfo {
		daemonIDString = k8sutil.IndexToName(node.HostID)
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)

		mdsConfig := &mdsConfig{
			Prefix:                        prefix,
			ServiceAddr:                   node.NodeIP,
			ServicePort:                   strconv.Itoa(node.MdsPort),
			ServiceDummyPort:              strconv.Itoa(node.DummyPort),
			ClusterEtcdAddr:               clusterEtcdAddr,
			ClusterSnapshotcloneProxyAddr: "",

			DaemonID:             daemonIDString,
			ResourceName:         resourceName,
			CurrentConfigMapName: currentConfigMapName,
			DataPathMap: config.NewDaemonDataPathMap(
				path.Join(c.DataDirHostPath, fmt.Sprint("mds-", daemonIDString)),
				path.Join(c.LogDirHostPath, fmt.Sprint("mds-", daemonIDString)),
				containerDataDir,
				containerLogDir,
			),
			ConfigMapMountPath: configMapMountPath,
		}

		if err := c.createMdsConfigMap(mdsConfig); err != nil {
			return err
		}

		d, err := c.makeDeployment(node.NodeName, node.NodeIP, mdsConfig)
		if err != nil {
			return err
		}

		newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(context.Background(), d, metav1.CreateOptions{})
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create mds deployment %s", resourceName)
			}
			logger.Infof("deployment for mds %s already exists. updating if needed", resourceName)

			// TODO:Update the daemon Deployment
			// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
			// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
			// }
		} else {
			logger.Infof("Deployment %s has been created , waiting for startup", newDeployment.GetName())
			deploymentsToWaitFor = append(deploymentsToWaitFor, newDeployment)
		}
	}

	// wait all Deployments to start
	for _, d := range deploymentsToWaitFor {
		if err := k8sutil.WaitForDeploymentToStart(context.TODO(), &c.Context, d); err != nil {
			return err
		}
	}

	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeMdsReady, curvev1.ConditionTrue, curvev1.ConditionMdsClusterCreatedReason, "MDS cluster has been created")

	return nil
}
