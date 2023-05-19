package metaserver

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/coreos/pkg/capnslog"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
)

const (
	AppName             = "curve-metaserver"
	ConfigMapNamePrefix = "curve-metaserver"

	FSPrefix           = "/curvefs/metaserver"
	FSContainerDataDir = "/curvefs/metaserver/data"
	FSContainerLogDir  = "/curvefs/metaserver/logs"
)

type Cluster struct {
	*daemon.Cluster
}

func New(c *daemon.Cluster) *Cluster {
	return &Cluster{Cluster: c}
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "metaserver")

func (c *Cluster) Start(nodesInfo []daemon.NodeInfo) error {
	msConfigs, dcs, err := c.buildConfigs(nodesInfo)
	if err != nil {
		return err
	}

	// create tool ConfigMap
	if err := c.createToolConfigMap(msConfigs); err != nil {
		return err
	}

	// create topology ConfigMap
	if err := topology.CreateTopoConfigMap(c.Cluster, dcs); err != nil {
		return err
	}

	// create logic pool
	_, err = topology.RunCreatePoolJob(c.Cluster, dcs, topology.LOGICAL_POOL)
	if err != nil {
		return err
	}

	var deploymentsToWaitFor []*appsv1.Deployment
	for _, msConfig := range msConfigs {
		if err := c.createMetaserverConfigMap(msConfig); err != nil {
			return err
		}
		d, err := c.makeDeployment(msConfig, msConfig.NodeName, msConfig.NodeIP)
		if err != nil {
			return err
		}

		newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create mds deployment %s", msConfig.ResourceName)
			}
			logger.Infof("deployment for mds %s already exists. updating if needed", msConfig.ResourceName)

			// TODO:Update the daemon Deployment
			// if err := updateDeploymentAndWait(c.Context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
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

	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeMetaServerReady, curvev1.ConditionTrue, curvev1.ConditionMetaServerClusterCreatedReason, "MetaServer cluster has been created")
	return nil
}

// Start Curve metaserver daemon
func (c *Cluster) buildConfigs(nodesInfo []daemon.NodeInfo) ([]*metaserverConfig, []*topology.DeployConfig, error) {
	logger.Infof("starting to run metaserver in namespace %q", c.NamespacedName.Namespace)

	// get ClusterEtcdAddr
	etcdOverrideCM, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get etcd override endoints configmap")
	}
	clusterEtcdAddr := etcdOverrideCM.Data[config.ClusterEtcdAddr]

	// get ClusterMdsAddr
	mdsOverrideCM, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.MdsOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get mds override endoints configmap")
	}
	clusterMdsAddr := mdsOverrideCM.Data[config.MdsOvverideConfigMapDataKey]
	clusterMdsDummyAddr := mdsOverrideCM.Data[config.ClusterMdsDummyAddr]

	// get clusterMetaserverAddr
	metaserveraddr := []string{}
	for _, node := range nodesInfo {
		metaserveraddr = append(metaserveraddr, fmt.Sprint(node.NodeIP, ":", strconv.Itoa(node.MetaserverPort)))
	}
	clusterMetaserverAddr := strings.Join(metaserveraddr, ",")
	logger.Info("clusterMetaserverAddr is ", clusterMetaserverAddr)

	metaserverConfigs := []*metaserverConfig{}
	dcs := []*topology.DeployConfig{}
	for _, node := range nodesInfo {
		daemonIDString := k8sutil.IndexToName(node.HostID)
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)

		metaserverConfig := &metaserverConfig{
			Prefix:                FSPrefix,
			ServiceAddr:           node.NodeIP,
			ServicePort:           strconv.Itoa(node.MetaserverPort),
			ServiceExternalAddr:   node.NodeIP,
			ServiceExternalPort:   strconv.Itoa(node.MetaserverExternalPort),
			ClusterEtcdAddr:       clusterEtcdAddr,
			ClusterMdsAddr:        clusterMdsAddr,
			ClusterMdsDummyAddr:   clusterMdsDummyAddr,
			ClusterMetaserverAddr: clusterMetaserverAddr,

			DaemonID:             daemonIDString,
			ResourceName:         resourceName,
			CurrentConfigMapName: currentConfigMapName,
			DataPathMap: config.NewDaemonDataPathMap(
				path.Join(c.DataDirHostPath, fmt.Sprint("metaserver-", daemonIDString)),
				path.Join(c.LogDirHostPath, fmt.Sprint("metaserver-", daemonIDString)),
				FSContainerDataDir,
				FSContainerLogDir,
			),
			NodeName: node.NodeName,
			NodeIP:   node.NodeIP,
		}

		dc := &topology.DeployConfig{
			Kind:             c.Kind,
			Role:             "metaserver",
			Copysets:         c.Metaserver.CopySets,
			NodeName:         node.NodeName,
			NodeIP:           node.NodeIP,
			Port:             node.MetaserverPort,
			ReplicasSequence: node.ReplicasSequence,
			Replicas:         len(c.Nodes),
			StandAlone:       node.StandAlone,
		}
		metaserverConfigs = append(metaserverConfigs, metaserverConfig)
		dcs = append(dcs, dc)
	}

	return metaserverConfigs, dcs, nil
}
