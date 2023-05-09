package chunkserver

import (
	"path"
	"strconv"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

// startChunkServers start all chunkservers for each device of every node
func (c *Cluster) startChunkServers() error {
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

	_ = c.createStartCSConfigMap()
	_ = c.createCSClientConfigMap()
	_ = c.CreateS3ConfigMap()

	var deploymentsToWaitFor []*apps.Deployment

	for _, csConfig := range chunkserverConfigs {
		err := c.createConfigMap(csConfig)
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
	if err := k8sutil.WaitForDeploymentsToStart(&c.Context, deploymentsToWaitFor, k8sutil.WaitForRunningInterval, k8sutil.WaitForRunningTimeout); err != nil {
		return err
	}

	return nil
}

// createConfigMap create chunkserver configmap for chunkserver server
func (c *Cluster) createConfigMap(csConfig chunkserverConfig) error {
	// get mds-conf-template from cluster
	chunkserverCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.ChunkServerConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.ChunkServerConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.ChunkServerConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.ChunkServerConfigMapTemp)
	}

	// read configmap data (string)
	var chunkserverData string
	for k, v := range chunkserverCMTemplate.Data {
		chunkserverData += k + "=" + v + "\n"
	}

	// replace ${} to specific parameters
	replacedChunkServerData, err := config.ReplaceConfigVars(chunkserverData, &csConfig)
	if err != nil {
		return err
	}

	// for debug
	// log.Info(chunkserverConfigVal)

	chunkserverConfigMap := map[string]string{
		config.ChunkserverConfigMapDataKey: replacedChunkServerData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csConfig.CurrentConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: chunkserverConfigMap,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	// Create chunkserver config in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create chunkserver configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}

func (c *Cluster) makeDeployment(csConfig *chunkserverConfig) (*apps.Deployment, error) {
	volumes := CSDaemonVolumes(csConfig)
	vols, _ := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volumes = append(volumes, vols...)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "chunkserver", csConfig.DaemonId, c.Kind)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   csConfig.ResourceName,
			Labels: labels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeCSDaemonContainer(csConfig),
			},
			NodeName:      csConfig.NodeName,
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csConfig.ResourceName,
			Namespace: c.NamespacedName.Namespace,
			Labels:    labels,
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
		},
	}

	// set ownerReference
	err := c.OwnerInfo.SetControllerReference(d)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set owner reference to chunkserver deployment %q", d.Name)
	}

	return d, nil
}

// makeCSDaemonContainer create chunkserver container
func (c *Cluster) makeCSDaemonContainer(csConfig *chunkserverConfig) v1.Container {

	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	// volumemount
	volMounts := CSDaemonVolumeMounts(csConfig)
	_, mounts := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volMounts = append(volMounts, mounts...)

	argsDeviceName := csConfig.DeviceName
	argsMountPath := ChunkserverContainerDataDir

	argsDataDir := path.Join(csConfig.Prefix, "data")
	argsChunkServerIp := csConfig.NodeIP
	argsChunkserverPort := strconv.Itoa(csConfig.Port)
	argsConfigFileMountPath := path.Join(config.ChunkserverConfigMapMountPathDir, config.ChunkserverConfigMapDataKey)

	container := v1.Container{
		Name: "chunkserver",
		Command: []string{
			"/bin/bash",
			startChunkserverMountPath,
		},
		Args: []string{
			argsDeviceName,
			argsMountPath,
			argsDataDir,
			argsChunkServerIp,
			argsChunkserverPort,
			argsConfigFileMountPath,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    volMounts,
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(csConfig.Port),
				HostPort:      int32(csConfig.Port),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
		SecurityContext: &v1.SecurityContext{
			Privileged:             &privileged,
			RunAsUser:              &runAsUser,
			RunAsNonRoot:           &runAsNonRoot,
			ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		},
	}

	return container
}
