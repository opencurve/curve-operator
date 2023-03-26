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
	AppName = "curve-etcd"

	// ContainerPath is the mount path of data and log
	ContainerDataDir = "/curvebs/etcd/data"
	ContainerLogDir  = "/curvebs/etcd/logs"
)

type Cluster struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	spec           curvev1.CurveClusterSpec
	ownerInfo      *k8sutil.OwnerInfo
}

var log = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "etcd")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec, ownerInfo *k8sutil.OwnerInfo) *Cluster {
	return &Cluster{
		context:        context,
		namespacedName: namespacedName,
		spec:           spec,
		ownerInfo:      ownerInfo,
	}
}

// Start begins the process of running a cluster of curve etcds.
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	var etcdEndpoints string
	var initial_cluster string
	for nodeName, ipAddr := range nodeNameIP {
		initial_cluster = fmt.Sprint(initial_cluster, nodeName, "=http://", ipAddr, ":", c.spec.Etcd.Port, ",")
		etcdEndpoints = fmt.Sprint(etcdEndpoints, ipAddr, ":", c.spec.Etcd.Port, ",")
	}
	etcdEndpoints = strings.TrimRight(etcdEndpoints, ",")
	initial_cluster = strings.TrimRight(initial_cluster, ",")

	// Create etcd override configmap
	err := c.createOverrideConfigMap(etcdEndpoints)
	if err != nil {
		return errors.Wrap(err, "failed to create etcd override configmap")
	}

	// Not use config file here otherwise etcd command only use configfile and ignore command line flags.
	// Create general curve-etcd-conf configmap for each etcd member
	// err = c.createEtcdConfigMap(initial_cluster)
	// if err != nil {
	// 	return errors.Wrapf(err, "failed to create %s configmap", config.EtcdConfigMapName)
	// }

	// reorder the nodeNameIP according to the order of nodes spec defined by the user
	// nodes:
	// - node1 - curve-etcd-a
	// - node2  - curve-etcd-b
	// - node3 - curve-etcd-c
	nodeNamesOrdered := make([]string, 0)
	for _, n := range c.spec.Nodes {
		for nodeName := range nodeNameIP {
			if n == nodeName {
				nodeNamesOrdered = append(nodeNamesOrdered, nodeName)
			}
		}
	}

	// Won't appear generally
	if len(nodeNamesOrdered) != 3 {
		log.Errorf("Nodes spec field is not 3, current nodes is %d", len(nodeNamesOrdered))
		return errors.New("Nodes spec field is not 3")
	}

	// create ConfigMap and referred Deployment by travel all nodes that have been labeled - "app=etcd"
	daemonID := 0
	var daemonIDString string
	for _, nodeName := range nodeNamesOrdered {
		daemonIDString = k8sutil.IndexToName(daemonID)
		daemonID++
		// Construct etcd config to pass to make deployment
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		etcdConfig := &etcdConfig{
			DaemonID:     daemonIDString,
			ResourceName: resourceName,
			DataPathMap: config.NewDaemonDataPathMap(
				fmt.Sprint(c.spec.DataDirHostPath, "/etcd-", daemonIDString),
				fmt.Sprint(c.spec.LogDirHostPath, "/etcd-", daemonIDString),
				ContainerDataDir,
				ContainerLogDir,
			),
		}

		// for debug
		// log.Infof("current node is %v", nodeName)

		// make etcd deployment
		d, err := c.makeDeployment(nodeName, nodeNameIP[nodeName], etcdConfig, initial_cluster)
		if err != nil {
			return errors.Wrap(err, "failed to create etcd Deployment")
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create etcd deployment %s", resourceName)
			}
			log.Infof("deployment for etcd %s already exists. updating if needed", resourceName)

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
