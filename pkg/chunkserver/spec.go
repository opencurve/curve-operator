package chunkserver

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// startChunkServers start all chunkservers for each device of every node
func (c *Cluster) startChunkServers() error {
	if len(jobsArr) == 0 {
		log.Errorf("no job to format device and provision chunk file")
		return nil
	}

	if len(chunkserverConfigs) == 0 {
		log.Errorf("no device need to start chunkserver")
		return nil
	}

	if len(jobsArr) != len(chunkserverConfigs) {
		log.Errorf("no device need to start chunkserver")
		return errors.New("failed to start chunkserver because of job numbers is not equal with chunkserver config")
	}

	// wait all job for complete process
	// create a ticker to check all jobs status
	halfMinuteTicker := time.NewTicker(10 * time.Second)
	defer halfMinuteTicker.Stop()

	chn := make(chan bool, 1)
	ctx, canf := context.WithTimeout(context.Background(), time.Duration(5*60*time.Second))
	defer canf()
	c.checkJobStatus(ctx, halfMinuteTicker, chn)

	flag := <-chn
	// not all job has completed
	if !flag {
		log.Error("All jobs have not been completed for more than 5 minutes")
		return errors.New("All jobs have not been completed for more than 5 minutes")
	}

	log.Info("all jobs has been completed in 5 mins")

	// 1. judge the mds override configmap if exist
	overrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.MdsOverrideCM, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get mds override endoints configmap")
	}

	// get mdsEndpoints data key of "mdsEndpoints" from mds-endpoints-override
	mdsEndpoints := overrideCM.Data[config.MdsOvverideCMDataKey]

	_ = c.createCSClientConfigMap(mdsEndpoints)

	_ = c.createS3ConfigMap(mdsEndpoints)

	err = c.createConfigMap(mdsEndpoints)
	if err != nil {
		return errors.Wrapf(err, "failed to create chunkserver configmap for %v", config.ChunkserverConfigMapName)
	}

	for _, csConfig := range chunkserverConfigs {
		d, err := c.runMakeDeployment(config.ChunkserverConfigMapDataKey, config.ChunkserverConfigMapMountPathDir, &csConfig)
		if err != nil {
			return errors.Wrap(err, "failed to create chunkserver Deployment")
		}

		newDeployment, err := c.context.Clientset.AppsV1().Deployments(c.namespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create chunkserver deployment %s", csConfig.ResourceName)
			}
			log.Infof("deployment for chunkserver %s already exists. updating if needed", csConfig.ResourceName)

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

// createCSClientConfigMap create cs_client configmap
func (c *Cluster) createCSClientConfigMap(mdsEndpoints string) error {
	configMapData, err := k8sutil.ReadConfFromTemplate("pkg/template/cs_client.conf")
	if err != nil {
		return errors.Wrap(err, "failed to read config file from template/cs_client.conf")
	}
	configMapData["mds.listen.addr"] = mdsEndpoints
	configMapData["global.logPath"] = ChunkserverContainerLogDir

	var csClientConfigVal string
	for k, v := range configMapData {
		csClientConfigVal = csClientConfigVal + k + "=" + v + "\n"
	}

	csClientConfigMap := map[string]string{
		config.CSClientConfigMapDataKey: csClientConfigVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.CSClientConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: csClientConfigMap,
	}

	// Create cs_client configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create cs_client configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

// createS3ConfigMap create s3 configmap
func (c *Cluster) createS3ConfigMap(mdsEndpoints string) error {
	configMapData, err := k8sutil.ReadConfFromTemplate("pkg/template/s3.conf")
	if err != nil {
		return errors.Wrap(err, "failed to read config file from template/cs_client.conf")
	}

	var s3ConfigVal string
	for k, v := range configMapData {
		s3ConfigVal = s3ConfigVal + k + "=" + v + "\n"
	}

	s3ConfigMap := map[string]string{
		config.S3ConfigMapDataKey: s3ConfigVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.S3ConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: s3ConfigMap,
	}

	// Create s3 configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create s3 configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

// createConfigMap create chunkserver configmap for chunkserver server
func (c *Cluster) createConfigMap(mdsEndpoints string) error {
	configMapData, err := k8sutil.ReadConfFromTemplate("pkg/template/chunkserver.conf")
	if err != nil {
		return errors.Wrap(err, "failed to read config file from template/chunkserver.conf")
	}

	localPrefix := fmt.Sprintf("local://%s", ChunkserverContainerDataDir)
	curvePrefix := fmt.Sprintf("curve://%s", ChunkserverContainerDataDir)
	// modify part field config
	configMapData["mds.listen.addr"] = mdsEndpoints
	configMapData["chunkserver.stor_uri"] = localPrefix
	configMapData["chunkserver.meta_uri"] = localPrefix + "/chunkserver.dat"
	configMapData["copyset.chunk_data_uri"] = localPrefix + "/copysets"
	configMapData["copyset.raft_log_uri"] = curvePrefix + "/copysets"
	configMapData["copyset.raft_meta_uri"] = localPrefix + "/copysets"
	configMapData["copyset.raft_snapshot_uri"] = curvePrefix + "/copysets"
	configMapData["copyset.recycler_uri"] = localPrefix + "/recycler"

	// # client配置文件
	// curve.config_path=${prefix}/conf/cs_client.conf
	configMapData["curve.config_path"] = ChunkserverContainerDataDir + "/conf/cs_client.conf"
	// # s3配置文件
	// s3.config_path=${prefix}/conf/s3.conf
	configMapData["s3.config_path"] = ChunkserverContainerDataDir + "/conf/s3.conf"

	configMapData["chunkfilepool.chunk_file_pool_dir"] = ChunkserverContainerDataDir
	configMapData["chunkfilepool.meta_path"] = ChunkserverContainerDataDir + "/chunkfilepool.meta"
	configMapData["walfilepool.meta_path"] = ChunkserverContainerDataDir + "/walfilepool.meta"
	configMapData["walfilepool.file_pool_dir"] = ChunkserverContainerDataDir + "/walfilepool.meta"
	configMapData["chunkserver.common.logDir"] = ChunkserverContainerLogDir

	// generate configmap data with only one key of "chunkserver.conf"
	var chunkserverConfigVal string
	for k, v := range configMapData {
		chunkserverConfigVal = chunkserverConfigVal + k + "=" + v + "\n"
	}

	chunkserverConfigMap := map[string]string{
		config.ChunkserverConfigMapDataKey: chunkserverConfigVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ChunkserverConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: chunkserverConfigMap,
	}

	// Create chunkserver config in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create chunkserver configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

func (c *Cluster) runMakeDeployment(configMapDataKey string, configMapMountPathDir string, csConfig *chunkserverConfig) (*apps.Deployment, error) {
	volumes := CSDaemonVolumes(config.ChunkserverConfigMapDataKey, config.ChunkserverConfigMapMountPathDir, config.ChunkserverConfigMapName, csConfig.DataPathMap)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   csConfig.ResourceName,
			Labels: c.getChunkServerPodLabels(csConfig),
		},
		Spec: v1.PodSpec{
			// InitContainers: []v1.Container{
			// 	c.makeChmodDirInitContainer(configMapDataKey, configMapMountPathDir, mdsConfig, curConfigMapName),
			// },
			Containers: []v1.Container{
				c.makeCSDaemonContainer(configMapDataKey, configMapMountPathDir, csConfig, config.ChunkserverConfigMapName),
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

	return d, nil
}

// makeCSDaemonContainer create chunkserver container
func (c *Cluster) makeCSDaemonContainer(configMapDataKey string, configMapMountPathDir string, csConfig *chunkserverConfig, curConfigMapName string) v1.Container {
	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	configFileMountPath := path.Join(configMapMountPathDir, configMapDataKey)

	// define two args(--chunkServerPort and --confPath) to startup 'curvebs-chunkserver'
	argsCSAddr := fmt.Sprintf("--chunkServerPort=%v ", csConfig.Port)
	argsConfigFileDir := fmt.Sprintf("--confPath=%s", configFileMountPath)

	container := v1.Container{
		Name: "chunkserver",
		Command: []string{
			"/curvebs/chunkserver/sbin/curvebs-chunkserver",
		},
		Args:            []string{argsCSAddr, argsConfigFileDir},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    CSDaemonVolumeMounts(configMapDataKey, configMapMountPathDir, curConfigMapName, csConfig.DataPathMap),
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

// checkJobStatus go routine to check all job's status
func (c *Cluster) checkJobStatus(ctx context.Context, ticker *time.Ticker, chn chan bool) {
	for {
		select {
		case <-ticker.C:
			log.Info("time is up")
			completed := 0
			for _, jobName := range jobsArr {
				job, err := c.context.Clientset.BatchV1().Jobs(c.namespacedName.Namespace).Get(jobName, metav1.GetOptions{})
				if err != nil {
					log.Errorf("failed to get job %s in cluster", jobName)
					return
				}

				if job.Status.Succeeded > 0 {
					completed++
					log.Infof("job %s has successd", job.Name)
				} else {
					log.Infof("job %s is running", job.Name)
				}

				if completed == len(jobsArr) {
					log.Info("all job has been successd, exit go routine")
					chn <- true
					return
				}
			}
		case <-ctx.Done():
			chn <- false
			log.Error("go routinue exit because check time is more than 5 mins")
			return
		}
	}
}

// getChunkServerPodLabels
func (c *Cluster) getChunkServerPodLabels(csConfig *chunkserverConfig) map[string]string {
	labels := make(map[string]string)
	labels["app"] = AppName
	labels["chunkserver"] = csConfig.ResourceName
	labels["curve_cluster"] = c.namespacedName.Namespace
	return labels
}
