package mds

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
	AppName = "curve-mds"

	// ContainerPath is the mount path of data and log
	ContainerDataDir = "/curvebs/mds/data"
	ContainerLogDir  = "/curvebs/mds/logs"
)

type Cluster struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	spec           curvev1.CurveClusterSpec
	ownerInfo      *k8sutil.OwnerInfo
}

var log = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "mds")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec, ownerInfo *k8sutil.OwnerInfo) *Cluster {
	return &Cluster{
		context:        context,
		namespacedName: namespacedName,
		spec:           spec,
		ownerInfo:      ownerInfo,
	}
}

// Start Curve mds daemon
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	// check if the etcd override configmap exist
	overrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get %s configmap from cluster", config.EtcdOverrideConfigMapName)
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.EtcdOverrideConfigMapName)
	}

	// get etcd endpoints from key of "etcdEndpoints" of etcd-endpoints-override
	etcdEndpoints := overrideCM.Data[config.EtcdOvverideConfigMapDataKey]

	// create mds configmap
	err = c.createMdsConfigMap(etcdEndpoints)
	if err != nil {
		return errors.Wrapf(err, "failed to create mds configmap for %v", config.MdsConfigMapName)
	}

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
				fmt.Sprint(c.spec.DataDirHostPath, "/mds-", daemonIDString),
				fmt.Sprint(c.spec.LogDirHostPath, "/mds-", daemonIDString),
				ContainerDataDir,
				ContainerLogDir,
			),
		}

		// for debug
		// log.Infof("current node is %v", nodeName)

		// make mds deployment
		d, err := c.makeDeployment(nodeName, nodeNameIP[nodeName], mdsConfig)
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

	k8sutil.UpdateCondition(context.TODO(), &c.context, c.namespacedName, curvev1.ConditionTypeMdsReady, curvev1.ConditionTrue, curvev1.ConditionMdsClusterCreatedReason, "MDS cluster has been created")

	return nil
}
