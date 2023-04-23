package etcd

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
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
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	var etcdEndpoints string
	var clusterEtcdAddr string
	for _, ipAddr := range nodeNameIP {
		etcdEndpoints = fmt.Sprint(etcdEndpoints, ipAddr, ":", c.Etcd.PeerPort, ",")
		clusterEtcdAddr = fmt.Sprint(clusterEtcdAddr, ipAddr, ":", c.Etcd.ClientPort, ",")
	}
	etcdEndpoints = strings.TrimRight(etcdEndpoints, ",")
	clusterEtcdAddr = strings.TrimRight(clusterEtcdAddr, ",")

	// Create etcd override configmap
	if err := c.createOverrideConfigMap(etcdEndpoints, clusterEtcdAddr); err != nil {
		return err
	}

	// reorder the nodeNameIP according to the order of nodes spec defined by the user
	// nodes:
	// - node1 - curve-etcd-a
	// - node2  - curve-etcd-b
	// - node3 - curve-etcd-c
	nodeNamesOrdered := make([]string, 0)
	for _, n := range c.Nodes {
		for nodeName := range nodeNameIP {
			if n == nodeName {
				nodeNamesOrdered = append(nodeNamesOrdered, nodeName)
			}
		}
	}

	// never happen
	if len(nodeNamesOrdered) != 3 {
		return errors.New("Nodes spec field is not 3")
	}

	hostId := 0
	var initial_cluster string
	for _, nodeName := range nodeNamesOrdered {
		initial_cluster = fmt.Sprint(initial_cluster, "etcd", strconv.Itoa(hostId), "0", "=http://", nodeNameIP[nodeName], ":", c.Etcd.PeerPort, ",")
		hostId++
	}
	initial_cluster = strings.TrimRight(initial_cluster, ",")

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

	daemonID := 0
	replicasSequence := 0
	var daemonIDString string
	for _, nodeName := range nodeNamesOrdered {
		daemonIDString = k8sutil.IndexToName(daemonID)
		// Construct etcd config to pass to make deployment
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)
		etcdConfig := &etcdConfig{
			Prefix:                 prefix,
			ServiceHostSequence:    strconv.Itoa(daemonID),
			ServiceReplicaSequence: strconv.Itoa(replicasSequence),
			ServiceAddr:            nodeNameIP[nodeName],
			ServicePort:            strconv.Itoa(c.Etcd.PeerPort),
			ServiceClientPort:      strconv.Itoa(c.Etcd.ClientPort),
			ClusterEtcdHttpAddr:    initial_cluster,

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
		daemonID++

		// create each etcd configmap for each deployment
		if err := c.createEtcdConfigMap(etcdConfig); err != nil {
			return err
		}

		// make etcd deployment
		d, err := c.makeDeployment(nodeName, nodeNameIP[nodeName], etcdConfig)
		if err != nil {
			return err
		}

		newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create etcd deployment %s", resourceName)
			}
			logger.Infof("deployment for etcd %s already exists. updating if needed", resourceName)

			// TODO:Update the daemon Deployment
			// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
			// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
			// }
		} else {
			logger.Infof("Deployment %s has been created , waiting for startup", newDeployment.GetName())
			// TODO:wait for the new deployment
			// deploymentsToWaitFor = append(deploymentsToWaitFor, newDeployment)
		}
		// update condition type and phase etc.
	}

	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeEtcdReady, curvev1.ConditionTrue, curvev1.ConditionEtcdClusterCreatedReason, "Etcd cluster has been created")

	return nil
}
