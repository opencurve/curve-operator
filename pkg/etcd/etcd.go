package etcd

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	AppName             = "curve-etcd"
	ConfigMapNamePrefix = "curve-etcd-conf"

	// ContainerPath is the mount path of data and log
	Prefix           = "/curvebs/etcd"
	ContainerDataDir = "/curvebs/etcd/data"
	ContainerLogDir  = "/curvebs/etcd/logs"
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

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "etcd")

func New(context clusterd.Context, namespacedName types.NamespacedName, spec curvev1.CurveClusterSpec, ownerInfo *k8sutil.OwnerInfo, dataDirHostPath, logDirHostPath, confDirHostPath string) *Cluster {
	return &Cluster{
		context:         context,
		namespacedName:  namespacedName,
		spec:            spec,
		ownerInfo:       ownerInfo,
		dataDirHostPath: dataDirHostPath,
		logDirHostPath:  logDirHostPath,
		confDirHostPath: confDirHostPath,
	}
}

// Start begins the process of running a cluster of curve etcds.
func (c *Cluster) Start(nodeNameIP map[string]string) error {
	var etcdEndpoints string
	var clusterEtcdAddr string

	for _, ipAddr := range nodeNameIP {
		etcdEndpoints = fmt.Sprint(etcdEndpoints, ipAddr, ":", c.spec.Etcd.PeerPort, ",")
		clusterEtcdAddr = fmt.Sprint(clusterEtcdAddr, ipAddr, ":", c.spec.Etcd.ClientPort, ",")
	}
	etcdEndpoints = strings.TrimRight(etcdEndpoints, ",")
	clusterEtcdAddr = strings.TrimRight(clusterEtcdAddr, ",")

	// Create etcd override configmap
	err := c.createOverrideConfigMap(etcdEndpoints, clusterEtcdAddr)
	if err != nil {
		return errors.Wrap(err, "failed to create etcd override configmap")
	}

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
		return errors.New("Nodes spec field is not 3")
	}

	hostId := 0
	var initial_cluster string
	for _, nodeName := range nodeNamesOrdered {
		initial_cluster = fmt.Sprint(initial_cluster, "etcd", strconv.Itoa(hostId), "0", "=http://", nodeNameIP[nodeName], ":", c.spec.Etcd.PeerPort, ",")
		hostId++
	}
	initial_cluster = strings.TrimRight(initial_cluster, ",")

	// create ConfigMap and referred Deployment by travel all nodes that have been labeled - "app=etcd"
	daemonID := 0
	replicasSequence := 0
	var daemonIDString string

	deploymentsToWaitFor := make([]*appsv1.Deployment, 0)
	for _, nodeName := range nodeNamesOrdered {
		daemonIDString = k8sutil.IndexToName(daemonID)
		// Construct etcd config to pass to make deployment
		resourceName := fmt.Sprintf("%s-%s", AppName, daemonIDString)
		currentConfigMapName := fmt.Sprintf("%s-%s", ConfigMapNamePrefix, daemonIDString)
		etcdConfig := &etcdConfig{
			Prefix:                 Prefix,
			ServiceHostSequence:    strconv.Itoa(daemonID),
			ServiceReplicaSequence: strconv.Itoa(replicasSequence),
			ServiceAddr:            nodeNameIP[nodeName],
			ServicePort:            strconv.Itoa(c.spec.Etcd.PeerPort),
			ServiceClientPort:      strconv.Itoa(c.spec.Etcd.ClientPort),
			ClusterEtcdHttpAddr:    initial_cluster,

			DaemonID:             daemonIDString,
			CurrentConfigMapName: currentConfigMapName,
			ResourceName:         resourceName,
			DataPathMap: config.NewDaemonDataPathMap(
				path.Join(c.dataDirHostPath, fmt.Sprint("etcd-", daemonIDString)),
				path.Join(c.logDirHostPath, fmt.Sprint("etcd-", daemonIDString)),
				ContainerDataDir,
				ContainerLogDir,
			),
		}
		daemonID++

		// for debug
		// logger.Infof("current node is %v", nodeName)

		// create each etcd configmap for each deployment
		err = c.createEtcdConfigMap(etcdConfig)
		if err != nil {
			return errors.Wrapf(err, "failed to create etcd configmap [ %v ]", config.MdsConfigMapName)
		}

		// make etcd deployment
		d, err := c.makeDeployment(nodeName, nodeNameIP[nodeName], etcdConfig)
		if err != nil {
			return errors.Wrap(err, "failed to create etcd Deployment")
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
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
			deploymentsToWaitFor = append(deploymentsToWaitFor, newDeployment)
		}
		// update condition type and phase etc.
	}

	logger.Info("starting etcd")
	if err := k8sutil.WaitForDeploymentsToStart(c.context.Clientset, 3*time.Second, 30*time.Second,
		deploymentsToWaitFor); err != nil {
		return err
	}
	k8sutil.UpdateCondition(context.TODO(), &c.context, c.namespacedName, curvev1.ConditionTypeEtcdReady, curvev1.ConditionTrue, curvev1.ConditionEtcdClusterCreatedReason, "Etcd cluster has been created")
	return nil
}
