package etcd

import (
	"fmt"
	"strings"

	"github.com/coreos/pkg/capnslog"
	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	AppName       = "curve-etcd"
	configMapName = "curve-etcd-config"

	// ContainerPath is the mount path of data and log
	ContainerDataDir = "/curvebs/etcd/data"
	ContainerLogDir  = "/curvebs/etcd/logs"

	DefaultEtcdCount = 3
)

type Cluster struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	spec           curvev1.CurveClusterSpec
}

var log = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "etcd")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec) *Cluster {
	return &Cluster{
		context:        context,
		namespacedName: namespacedName,
		spec:           spec,
	}
}

// Start begins the process of running a cluster of curve etcds.
func (c *Cluster) Start() error {
	// get node name and internal ip map
	nodeNameIP, err := c.getNodeInfoMap()
	if err != nil {
		return errors.Wrap(err, "failed get node with app=etcd label")
	}

	var etcd_endpoints string
	for nodeName, ipAddr := range nodeNameIP {
		etcd_endpoints = fmt.Sprint(etcd_endpoints, nodeName, "=", `http://`, ipAddr, ":", c.spec.Etcd.Port, ",")
	}
	etcd_endpoints = strings.TrimRight(etcd_endpoints, ",")

	// Create configmap template for etcd server

	daemonID := 0
	var daemonIDString string
	for nodeName, ipAddr := range nodeNameIP {
		daemonIDString = k8sutil.IndexToName(daemonID)
		daemonID++
		// Construct etcd config to pass to make deployment
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		etcdConfig := &etcdConfig{
			DaemonID:     daemonIDString,
			ResourceName: resourceName,
			DataPathMap: config.NewDaemonDataPathMap(
				c.spec.DataDirHostPath,
				c.spec.LogDirHostPath,
				ContainerDataDir,
				ContainerLogDir,
			),
		}
		// for debug
		// log.Infof("current node is %v", nodeName)

		// determine the etcd_points that pass to ConfigMap field "initial-cluster" by nodeNameIP
		curConfigMapName, err := c.createConfigMap(daemonIDString, nodeName, ipAddr, etcd_endpoints)
		if err != nil {
			return errors.Wrapf(err, "failed to create etcd configmap for %v", nodeName)
		}

		// make etcd deployment
		d, err := c.makeDeployment(config.EtcdConfigMapDataKey, config.EtcdConfigMapMountPathDir, nodeName, etcdConfig, curConfigMapName)
		if err != nil {
			return errors.Wrap(err, "failed to create etcd Deployment")
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create etcd deployment %s", resourceName)
			}
			log.Infof("deployment for mgr %s already exists. updating if needed", resourceName)

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
