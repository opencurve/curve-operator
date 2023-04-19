package mds

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	AppName             = "curve-mds"
	ConfigMapNamePrefix = "curve-mds-conf"

	// ContainerPath is the mount path of data and log
	Prefix           = "/curvebs/mds"
	ContainerDataDir = "/curvebs/mds/data"
	ContainerLogDir  = "/curvebs/mds/logs"
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

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "mds")

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

// Start Curve mds daemon
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	// check if the etcd override configmap exist
	overrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.EtcdOverrideConfigMapName)
	}

	// get etcd endpoints from key of "clusterEtcdAddr" of etcd-endpoints-override
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
	for _, n := range c.spec.Nodes {
		for nodeName := range nodeNameIP {
			if n == nodeName {
				nodeNamesOrdered = append(nodeNamesOrdered, nodeName)
			}
		}
	}

	if len(nodeNamesOrdered) != 3 {
		return errors.New("Nodes spec field is not 3")
	}

	daemonID := 0
	var daemonIDString string
	deploymentsToWaitFor := make([]*appsv1.Deployment, 0)
	for _, nodeName := range nodeNamesOrdered {
		daemonIDString = k8sutil.IndexToName(daemonID)
		daemonID++
		// Construct mds config
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)
		mdsConfig := &mdsConfig{
			ServiceAddr:                   nodeNameIP[nodeName],
			ServicePort:                   strconv.Itoa(c.spec.Mds.Port),
			ServiceDummyPort:              strconv.Itoa(c.spec.Mds.DummyPort),
			ClusterEtcdAddr:               clusterEtcdAddr,
			ClusterSnapshotcloneProxyAddr: "",

			DaemonID:             daemonIDString,
			ResourceName:         resourceName,
			CurrentConfigMapName: currentConfigMapName,
			DataPathMap: config.NewDaemonDataPathMap(
				path.Join(c.dataDirHostPath, fmt.Sprint("mds-", daemonIDString)),
				path.Join(c.logDirHostPath, fmt.Sprint("mds-", daemonIDString)),
				ContainerDataDir,
				ContainerLogDir,
			),
		}

		// for debug
		// log.Infof("current node is %v", nodeName)

		// create each mds configmap for each deployment
		err = c.createMdsConfigMap(mdsConfig)
		if err != nil {
			return errors.Wrapf(err, "failed to create mds configmap %q", config.MdsConfigMapName)
		}

		// make mds deployment
		d, err := c.makeDeployment(nodeName, nodeNameIP[nodeName], mdsConfig)
		if err != nil {
			return errors.Wrapf(err, "failed to create mds Deployment %q", mdsConfig.ResourceName)
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
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
		// update condition type and phase etc.
	}

	logger.Info("starting mds server")
	if err := k8sutil.WaitForDeploymentsToStart(c.context.Clientset, 3*time.Second, 30*time.Second,
		deploymentsToWaitFor); err != nil {
		return err
	}

	k8sutil.UpdateCondition(context.TODO(), &c.context, c.namespacedName, curvev1.ConditionTypeMdsReady, curvev1.ConditionTrue, curvev1.ConditionMdsClusterCreatedReason, "MDS cluster has been created")

	return nil
}
