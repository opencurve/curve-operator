package chunkserver

import (
	"context"
	"time"

	"emperror.dev/errors"
	"github.com/coreos/pkg/capnslog"
	apps "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/chunkserver/script"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

const (
	AppName             = "curve-chunkserver"
	ConfigMapNamePrefix = "curve-chunkserver-conf"

	Prefix                      = "/curvebs/chunkserver"
	ChunkserverContainerDataDir = "/curvebs/chunkserver/data"
	ChunkserverContainerLogDir  = "/curvebs/chunkserver/logs"

	// start.sh
	startChunkserverConfigMapName     = "start-chunkserver-conf"
	startChunkserverScriptFileDataKey = "start_chunkserver.sh"
	startChunkserverMountPath         = "/curvebs/tools/sbin/start_chunkserver.sh"
)

type Cluster struct {
	*daemon.Cluster
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "chunkserver")

func New(c *daemon.Cluster) *Cluster {
	return &Cluster{Cluster: c}
}

// Start begins the chunkserver daemon
func (c *Cluster) Start(nodesInfo []daemon.NodeInfo, globalDCs []*topology.DeployConfig) ([]*topology.DeployConfig, error) {
	logger.Infof("start running chunkserver in namespace %q", c.Namespace)

	err := c.CreateSpecRoleAllConfigMap(config.ROLE_CHUNKSERVER, config.ChunkserverAllConfigMapName)
	if err != nil {
		return nil, err
	}

	err = c.CreateSpecRoleAllConfigMap(config.ROLE_SNAPSHOTCLONE, config.SnapShotCloneAllConfigMapName)
	if err != nil {
		return nil, err
	}
	// startProvisioningOverNodes format device and prepare chunk files
	dcs, globalDCs, err := c.startProvisioningOverNodes(nodesInfo, globalDCs)
	if err != nil {
		return nil, err
	}

	err = c.WaitForForamtJobCompletion(context.TODO(), 24*time.Hour)
	if err != nil {
		return nil, err
	}

	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeFormatedReady, curvev1.ConditionTrue, curvev1.ConditionFormatChunkfilePoolReason, "Formating chunkfilepool successed")
	logger.Info("all jobs run completed in 24 hours")

	// create tool ConfigMap
	err = c.createToolConfigMap()
	if err != nil {
		return nil, err
	}

	// create topology ConfigMap
	err = topology.CreateTopoConfigMap(c.Cluster, dcs)
	if err != nil {
		return nil, err
	}

	// create physical pool
	_, err = topology.RunCreatePoolJob(c.Cluster, dcs, topology.PYHSICAL_POOL)
	if err != nil {
		return nil, err
	}
	logger.Info("The physical pool was created successfully")

	// start all chunkservers for each device of every node
	err = c.startChunkServers()
	if err != nil {
		return nil, err
	}

	// wait all chunkservers online before create logical pool
	logger.Info("starting all chunkserver")
	k8sutil.UpdateStatusCondition(c.Kind, context.TODO(), &c.Context, c.NamespacedName, curvev1.ConditionTypeChunkServerReady, curvev1.ConditionTrue, curvev1.ConditionChunkServerClusterCreatedReason, "Chunkserver cluster has been created")
	time.Sleep(30 * time.Second)

	// create logical pool
	_, err = topology.RunCreatePoolJob(c.Cluster, dcs, topology.LOGICAL_POOL)
	if err != nil {
		return nil, err
	}
	logger.Info("create logical pool successed")

	return globalDCs, nil
}

// startChunkServers start all chunkservers for each device of every node
func (c *Cluster) startChunkServers() error {
	err := c.preStart()
	if err != nil {
		return err
	}

	var deploymentsToWaitFor []*apps.Deployment
	for _, csConfig := range chunkserverConfigs {
		err := c.CreateEachConfigMap(config.ChunkserverConfigMapDataKey, &csConfig, csConfig.CurrentConfigMapName)
		if err != nil {
			return err
		}

		d, err := c.makeDeployment(&csConfig)
		if err != nil {
			return err
		}

		newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create chunkserver deployment %s", csConfig.ResourceName)
			}
			logger.Infof("deployment for chunkserver %s already exists. updating if needed", csConfig.ResourceName)

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

	return nil
}

// preStart
func (c *Cluster) preStart() error {
	if len(job2DeviceInfos) == 0 {
		logger.Errorf("no job to format device and provision chunk file")
		return nil
	}

	if len(chunkserverConfigs) == 0 {
		logger.Errorf("no device need to start chunkserver")
		return nil
	}

	if len(job2DeviceInfos) != len(chunkserverConfigs) {
		return errors.New("failed to start chunkserver because of job numbers is not equal with chunkserver config")
	}

	err := c.UpdateSpecRoleAllConfigMap(config.ChunkserverAllConfigMapName, startChunkserverScriptFileDataKey, script.START, nil)
	if err != nil {
		return err
	}

	if c.SnapShotClone.Enable {
		s3Data, err := c.getS3ConfigMapData()
		if err != nil {
			return err
		}

		err = c.UpdateSpecRoleAllConfigMap(config.SnapShotCloneAllConfigMapName, config.S3ConfigMapDataKey, s3Data, nil)
		if err != nil {
			return err
		}
	}

	err = c.UpdateSpecRoleAllConfigMap(config.ChunkserverAllConfigMapName, config.CSClientConfigMapDataKey, "", &chunkserverConfigs[0])
	if err != nil {
		return err
	}

	return nil
}
