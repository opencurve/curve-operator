package mds

import (
	"context"
	"fmt"
	"path"
	"strconv"

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
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	overrideCM, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterEtcdAddr := overrideCM.Data[config.ClusterEtcdAddr]

	// create mds override configmap to record mds endpoints
	err = c.createOverrideMdsCM(nodeNameIP)
	if err != nil {
		return err
	}

	// reorder the nodeNameIP according to the order of nodes spec defined by the user
	// nodes:
	// - node1 - curve-mds-a
	// - node2  - curve-mds-b
	// - node3 - curve-mds-c
	nodeNamesOrdered := make([]string, 0)
	for _, n := range c.Nodes {
		for nodeName := range nodeNameIP {
			if n == nodeName {
				nodeNamesOrdered = append(nodeNamesOrdered, nodeName)
			}
		}
	}

	// never heppend
	if len(nodeNamesOrdered) != 3 {
		return errors.New("Nodes spec field is not 3")
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

	daemonID := 0
	var daemonIDString string
	for _, nodeName := range nodeNamesOrdered {
		daemonIDString = k8sutil.IndexToName(daemonID)
		daemonID++
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)

		mdsConfig := &mdsConfig{
			Prefix:                        prefix,
			ServiceAddr:                   nodeNameIP[nodeName],
			ServicePort:                   strconv.Itoa(c.Mds.Port),
			ServiceDummyPort:              strconv.Itoa(c.Mds.DummyPort),
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

		d, err := c.makeDeployment(nodeName, nodeNameIP[nodeName], mdsConfig)
		if err != nil {
			return err
		}

		newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(d)
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
	if err := k8sutil.WaitForDeploymentsToStart(&c.Context, deploymentsToWaitFor, k8sutil.WaitForRunningInterval, k8sutil.WaitForRunningTimeout); err != nil {
		return err
	}

	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeMdsReady, curvev1.ConditionTrue, curvev1.ConditionMdsClusterCreatedReason, "MDS cluster has been created")

	return nil
}
