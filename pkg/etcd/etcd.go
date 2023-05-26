package etcd

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

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

const (
	AppName             = "curve-etcd"
	ConfigMapNamePrefix = "curve-etcd-conf"

	// ContainerPath is the mount path of data and log
	Prefix           = "/curvebs/etcd"
	ContainerDataDir = "/curvebs/etcd/data"
	ContainerLogDir  = "/curvebs/etcd/logs"

	FSPrefix           = "/curvefs/etcd"
	FSContainerDataDir = "/curvefs/etcd/data"
	FSContainerLogDir  = "/curvefs/etcd/logs"
)

type Cluster struct {
	*daemon.Cluster
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "etcd")

func New(c *daemon.Cluster) *Cluster {
	return &Cluster{Cluster: c}
}

// Start begins the process of running a cluster of curve etcds.
func (c *Cluster) Start(nodesInfo []daemon.NodeInfo) ([]*topology.DeployConfig, error) {
	var etcdEndpoints, clusterEtcdAddr, initialCluster string
	for _, node := range nodesInfo {
		etcdEndpoints = fmt.Sprint(etcdEndpoints, node.NodeIP, ":", node.PeerPort, ",")
		clusterEtcdAddr = fmt.Sprint(clusterEtcdAddr, node.NodeIP, ":", node.ClientPort, ",")
		initialCluster = fmt.Sprint(initialCluster, "etcd", strconv.Itoa(node.HostID), strconv.Itoa(node.ReplicasSequence), "=http://", node.NodeIP, ":", node.PeerPort, ",")
	}
	etcdEndpoints = strings.TrimRight(etcdEndpoints, ",")
	clusterEtcdAddr = strings.TrimRight(clusterEtcdAddr, ",")
	initialCluster = strings.TrimRight(initialCluster, ",")
	logger.Infof("initialCluster %v", initialCluster)

	// Create etcd override configmap
	if err := c.createOverrideConfigMap(etcdEndpoints, clusterEtcdAddr); err != nil {
		return nil, err
	}

	// create ConfigMap and referred Deployment by travel all nodes that have been labeled - "app=etcd"
	var configMapMountPath, prefix, containerDataDir, containerLogDir string
	if c.Kind == config.KIND_CURVEBS {
		prefix = Prefix
		containerDataDir = ContainerDataDir
		containerLogDir = ContainerLogDir
		configMapMountPath = config.EtcdConfigMapMountPathDir
	} else {
		prefix = FSPrefix
		containerDataDir = FSContainerDataDir
		containerLogDir = FSContainerLogDir
		configMapMountPath = config.FSEtcdConfigMapMountPathDir
	}

	var deploymentsToWaitFor []*appsv1.Deployment
	var dcs []*topology.DeployConfig

	for _, node := range nodesInfo {
		daemonIDString := k8sutil.IndexToName(node.HostID)
		// Construct etcd config to pass to make deployment
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)
		etcdConfig := &etcdConfig{
			Prefix:                 prefix,
			ServiceHostSequence:    strconv.Itoa(node.HostID),
			ServiceReplicaSequence: strconv.Itoa(node.ReplicasSequence),
			ServiceAddr:            node.NodeIP,
			ServicePort:            strconv.Itoa(node.PeerPort),
			ServiceClientPort:      strconv.Itoa(node.ClientPort),
			ClusterEtcdHttpAddr:    initialCluster,

			DaemonID:             daemonIDString,
			CurrentConfigMapName: currentConfigMapName,
			ResourceName:         resourceName,
			DataPathMap: config.NewDaemonDataPathMap(
				path.Join(c.DataDirHostPath, fmt.Sprint("etcd-", daemonIDString)),
				path.Join(c.LogDirHostPath, fmt.Sprint("etcd-", daemonIDString)),
				containerDataDir,
				containerLogDir,
			),
			ConfigMapMountPath: configMapMountPath,
		}
		dc := &topology.DeployConfig{
			Kind:             c.Kind,
			Role:             config.ETCD_ROLE,
			NodeName:         node.NodeName,
			NodeIP:           node.NodeIP,
			Port:             node.ClientPort,
			ReplicasSequence: node.ReplicasSequence,
			Replicas:         len(c.Nodes),
			StandAlone:       node.StandAlone,
		}

		dcs = append(dcs, dc)

		// create each etcd configmap for each deployment
		if err := c.createEtcdConfigMap(etcdConfig); err != nil {
			return nil, err
		}

		// make etcd deployment
		d, err := c.makeDeployment(node.NodeName, node.NodeIP, etcdConfig)
		if err != nil {
			return nil, err
		}

		newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return nil, errors.Wrapf(err, "failed to create etcd deployment %s", resourceName)
			}
			logger.Infof("deployment for etcd %s already exists. updating if needed", resourceName)

			// if err := k8sutil.UpdateDeploymentAndWait(context.TODO(), &c.Context, d, c.Namespace, nil); err != nil {
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
			return nil, err
		}
	}

	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeEtcdReady, curvev1.ConditionTrue, curvev1.ConditionEtcdClusterCreatedReason, "Etcd cluster has been created")

	return dcs, nil
}
