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
)

// startChunkServers start all chunkservers for each device of every node
func (c *Cluster) startChunkServers() error {
	if len(jobsArr) == 0 {
		logger.Errorf("no job to format device and provision chunk file")
		return nil
	}

	if len(chunkserverConfigs) == 0 {
		logger.Errorf("no device need to start chunkserver")
		return nil
	}

	if len(jobsArr) != len(chunkserverConfigs) {
		logger.Errorf("no device need to start chunkserver")
		return errors.New("failed to start chunkserver because of job numbers is not equal with chunkserver config")
	}

	_ = c.createStartCSConfigMap()

	_ = c.createCSClientConfigMap()

	_ = c.CreateS3ConfigMap()

	for _, csConfig := range chunkserverConfigs {

		err := c.createConfigMap(csConfig)
		if err != nil {
			return errors.Wrapf(err, "failed to create chunkserver configmap for %v", config.ChunkserverConfigMapName)
		}

		d, err := c.makeDeployment(&csConfig)
		if err != nil {
			return errors.Wrap(err, "failed to create chunkserver Deployment")
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create chunkserver deployment %s", csConfig.ResourceName)
			}
			logger.Infof("deployment for chunkserver %s already exists. updating if needed", csConfig.ResourceName)

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

	return nil
}

// createCSClientConfigMap create cs_client configmap
func (c *Cluster) createCSClientConfigMap() error {
	// 1. get mds-conf-template from cluster
	csClientCMTemplate, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.CsClientConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.CsClientConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.CsClientConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.CsClientConfigMapTemp)
	}

	// 2. read configmap data (string)
	csClientCMData := csClientCMTemplate.Data[config.CSClientConfigMapDataKey]
	// 3. replace ${} to specific parameters
	replacedCsClientData, err := config.ReplaceConfigVars(csClientCMData, &chunkserverConfigs[0])
	if err != nil {
		return errors.Wrap(err, "failed to Replace cs_client config template to generate a new cs_client configmap to start server.")
	}

	csClientConfigMap := map[string]string{
		config.CSClientConfigMapDataKey: replacedCsClientData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.CSClientConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: csClientConfigMap,
	}

	err = c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to cs_client.conf configmap %q", config.CSClientConfigMapName)
	}

	// Create cs_client configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create cs_client configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

// createS3ConfigMap create s3 configmap
func (c *Cluster) CreateS3ConfigMap() error {
	s3CMTemplate, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.S3ConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.S3ConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.S3ConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.S3ConfigMapTemp)
	}

	data := s3CMTemplate.Data
	// if true
	if c.spec.SnapShotClone.Enable {
		data["s3.ak"] = c.spec.SnapShotClone.S3Config.AK
		data["s3.sk"] = c.spec.SnapShotClone.S3Config.SK
		data["s3.nos_address"] = c.spec.SnapShotClone.S3Config.NosAddress
		data["s3.snapshot_bucket_name"] = c.spec.SnapShotClone.S3Config.SnapShotBucketName
	}

	var configMapData string
	for k, v := range data {
		configMapData = configMapData + k + "=" + v + "\n"
	}

	s3ConfigMap := map[string]string{
		config.S3ConfigMapDataKey: configMapData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.S3ConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: s3ConfigMap,
	}

	err = c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to s3.conf configmap %q", config.S3ConfigMapName)
	}

	// Create s3 configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create s3 configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

// createConfigMap create configmap to run start_chunkserver.sh script
func (c *Cluster) createStartCSConfigMap() error {
	// generate configmap data with only one key of "format.sh"
	startCSConfigMap := map[string]string{
		startChunkserverScriptFileDataKey: START,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      startChunkserverConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: startCSConfigMap,
	}

	err := c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to cs.conf configmap %q", startChunkserverConfigMapName)
	}

	// Create format.sh configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create override configmap %s", c.namespacedName.Namespace)
	}
	return nil
}

// createConfigMap create chunkserver configmap for chunkserver server
func (c *Cluster) createConfigMap(csConfig chunkserverConfig) error {
	// 1. get mds-conf-template from cluster
	chunkserverCMTemplate, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.ChunkServerConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.ChunkServerConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.ChunkServerConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.ChunkServerConfigMapTemp)
	}

	// 2. read configmap data (string)
	var chunkserverData string
	for k, v := range chunkserverCMTemplate.Data {
		chunkserverData += k + "=" + v + "\n"
	}

	// 3. replace ${} to specific parameters
	replacedChunkServerData, err := config.ReplaceConfigVars(chunkserverData, &csConfig)
	if err != nil {
		return errors.Wrap(err, "failed to Replace chunkserver config template to generate a new chunkserver configmap to start server.")
	}

	// for debug
	// log.Info(chunkserverConfigVal)

	chunkserverConfigMap := map[string]string{
		config.ChunkserverConfigMapDataKey: replacedChunkServerData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csConfig.CurrentConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: chunkserverConfigMap,
	}

	err = c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to chunkserverconfig configmap %q", config.ChunkserverConfigMapName)
	}

	// Create chunkserver config in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create chunkserver configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

func (c *Cluster) makeDeployment(csConfig *chunkserverConfig) (*apps.Deployment, error) {
	volumes := CSDaemonVolumes(csConfig)
	vols, _ := c.createTopoAndToolVolumeAndMount()
	volumes = append(volumes, vols...)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   csConfig.ResourceName,
			Labels: c.getChunkServerPodLabels(csConfig),
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
			Namespace: c.namespacedName.Namespace,
			Labels:    c.getChunkServerPodLabels(csConfig),
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: c.getChunkServerPodLabels(csConfig),
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
		},
	}

	// set ownerReference
	err := c.ownerInfo.SetControllerReference(d)
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
	_, mounts := c.createTopoAndToolVolumeAndMount()
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
			// "/curvebs/chunkserver/sbin/curvebs-chunkserver",
		},
		Args: []string{
			argsDeviceName,
			argsMountPath,
			argsDataDir,
			argsChunkServerIp,
			argsChunkserverPort,
			argsConfigFileMountPath,
		},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
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

// getChunkServerPodLabels
func (c *Cluster) getChunkServerPodLabels(csConfig *chunkserverConfig) map[string]string {
	labels := make(map[string]string)
	labels["app"] = AppName
	labels["chunkserver"] = csConfig.ResourceName
	labels["curve_cluster"] = c.namespacedName.Namespace
	return labels
}
