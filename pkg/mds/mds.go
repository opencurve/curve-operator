package mds

import (
	"fmt"

	"github.com/coreos/pkg/capnslog"
	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	AppName = "curve-mds"

	// ContainerPath is the mount path of data and log
	ContainerDataDir = "/curvebs/mds/data"
	ContainerLogDir  = "/curvebs/mds/logs"

	DefaultMdsCount = 3
)

type Cluster struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	spec           curvev1.CurveClusterSpec
}

var log = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "mds")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec) *Cluster {
	return &Cluster{
		context:        context,
		namespacedName: namespacedName,
		spec:           spec,
	}
}

// Start begins the process of running a cluster of curve mds.
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	// 1. judge the etcd override configmap if exist
	overrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get etcd override endoints configmap")
	}

	// get etcdEndpoints data key of "etcdEndpoints" from etcd-endpoints-override
	etcdEndpoints := overrideCM.Data[config.EtcdOvverideConfigMapDataKey]

	// determine the etcd_points that pass to ConfigMap field "initial-cluster" by nodeNameIP
	curConfigMapName, err := c.createConfigMap(etcdEndpoints)
	if err != nil {
		return errors.Wrapf(err, "failed to create mds configmap for %v", config.MdsConfigMapName)
	}

	// 2. create mds configmap override to record mds endpoints
	err = c.createOverrideMdsCM(nodeNameIP)
	if err != nil {
		return err
	}

	// reorder the nodeNameIP according to the order of nodes spec defined by the user
	// nodes:
	// - 10.219.196.145 - curve-mds-a
	// - 10.219.192.90  - curve-mds-b
	// - 10.219.196.150 - curve-mds-c
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
		// Construct mds config to pass to make deployment
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		mdsConfig := &mdsConfig{
			DaemonID:     daemonIDString,
			ResourceName: resourceName,
			DataPathMap: config.NewDaemonDataPathMap(
				fmt.Sprint(c.spec.DataDirHostPath, "/mds"),
				fmt.Sprint(c.spec.LogDirHostPath, "/mds"),
				ContainerDataDir,
				ContainerLogDir,
			),
		}

		// for debug
		// log.Infof("current node is %v", nodeName)

		// make mds deployment
		d, err := c.makeDeployment(config.MdsConfigMapDataKey, config.MdsConfigMapMountPathDir, nodeName, mdsConfig, curConfigMapName, nodeNameIP[nodeName])
		if err != nil {
			return errors.Wrap(err, "failed to create mds Deployment")
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create mds deployment %s", resourceName)
			}
			log.Infof("deployment for mds %s already exists. updating if needed", resourceName)

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

	return nil
}
