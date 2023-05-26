package snapshotclone

import (
	"context"
	"fmt"
	"path"
	"strconv"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
)

// prepareConfigMap
func (c *Cluster) prepareConfigMap(snapConfig *snapConfig) error {
	// 1. get s3 configmap that must has been created by chunkserver
	_, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(context.Background(), config.S3ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.S3ConfigMapName)
	}
	logger.Infof("check %s configmap has been exist", config.S3ConfigMapName)

	// 2. create snap_client.conf configmap
	err = c.createSnapClientConfigMap(snapConfig)
	if err != nil {
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.SnapShotCloneConfigMapName)
	}
	logger.Infof("creat ConfigMap '%s' successed", config.SnapClientConfigMapName)

	// 3. create snapshotclone.conf configmap
	err = c.createSnapShotCloneConfigMap(snapConfig)
	if err != nil {
		return errors.Wrapf(err, "failed to get %s configmap from cluster", config.SnapShotCloneConfigMapName)
	}
	logger.Infof("creat ConfigMap '%s' successed", config.SnapShotCloneConfigMapName)

	// 4. create nginx.conf configmap
	err = c.createNginxConfigMap(snapConfig)
	if err != nil {
		logger.Error("failed to create nginx.conf configMap")
	}

	return nil
}

// createSnapClientConf
func (c *Cluster) createSnapClientConfigMap(snapConfig *snapConfig) error {
	// 1. get ...-conf-template from cluster
	snapClientCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(context.Background(), config.SnapClientConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.SnapClientConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.SnapClientConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.SnapClientConfigMapTemp)
	}

	// 2. read configmap data (string)
	snapClientCMData := snapClientCMTemplate.Data[config.SnapClientConfigMapDataKey]
	// 3. replace ${} to specific parameters
	replacedSnapClientData, err := config.ReplaceConfigVars(snapClientCMData, snapConfig)

	// for debug
	// logger.Info(replacedSnapClientData)

	if err != nil {
		return errors.Wrap(err, "failed to Replace snap_client config template to generate a new snap_client configmap to start server.")
	}

	snapClientConfigMap := map[string]string{
		config.SnapClientConfigMapDataKey: replacedSnapClientData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.SnapClientConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: snapClientConfigMap,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to snap_client.conf configmap %q", config.SnapClientConfigMapName)
	}

	// Create cs_client configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create snap_client configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}

func (c *Cluster) createSnapShotCloneConfigMap(snapConfig *snapConfig) error {
	// 1. get snapshotclone-conf-template from cluster
	snapShotCloneCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(context.Background(), config.SnapShotCloneConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.SnapShotCloneConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.SnapShotCloneConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.SnapShotCloneConfigMapTemp)
	}

	// 2. read configmap data (string)
	snapShotCloneCMData := snapShotCloneCMTemplate.Data[config.SnapShotCloneConfigMapDataKey]
	// 3. replace ${} to specific parameters
	replacedSnapShotCloneData, err := config.ReplaceConfigVars(snapShotCloneCMData, snapConfig)
	if err != nil {
		logger.Error("failed to Replace snapshotclone config template to generate %s to start server.", snapConfig.CurrentConfigMapName)
		return errors.Wrap(err, "failed to Replace snapshotclone config template to generate a new snapshotclone configmap to start server.")
	}

	snapCloneConfigMap := map[string]string{
		config.SnapShotCloneConfigMapDataKey: replacedSnapShotCloneData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapConfig.CurrentConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: snapCloneConfigMap,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to snapshotclone.conf configmap %q", config.SnapShotCloneConfigMapName)
	}

	// Create cs_client configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create snap_client configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}

// makeDeployment make snapshotclone deployment to run snapshotclone daemon
func (c *Cluster) makeDeployment(nodeName string, nodeIP string, snapConfig *snapConfig) (*apps.Deployment, error) {
	volumes := SnapDaemonVolumes(snapConfig)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "snapshotclone", snapConfig.DaemonID, c.Kind)

	// for debug
	// log.Infof("snapConfig %+v", snapConfig)

	runAsUser := int64(0)
	runAsNonRoot := false

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   snapConfig.ResourceName,
			Labels: labels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeSnapshotDaemonContainer(nodeIP, snapConfig),
			},
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:    &runAsUser,
				RunAsNonRoot: &runAsNonRoot,
			},
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapConfig.ResourceName,
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

// makeSnapshotDaemonContainer create snapshotclone container
func (c *Cluster) makeSnapshotDaemonContainer(nodeIP string, snapConfig *snapConfig) v1.Container {
	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	argsNginxConf := path.Join(config.NginxConfigMapMountPath, config.NginxConfigMapDataKey)
	configFileMountPath := path.Join(config.SnapShotCloneConfigMapMountPath, config.SnapShotCloneConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--conf=%s", configFileMountPath)

	port, _ := strconv.Atoi(snapConfig.ServicePort)
	dummyPort, _ := strconv.Atoi(snapConfig.ServiceDummyPort)
	proxyPort, _ := strconv.Atoi(snapConfig.ServiceProxyPort)

	container := v1.Container{
		Name: "snapshotclone",
		Command: []string{
			"/bin/bash",
			config.StartSnapConfigMapMountPath,
		},
		Args: []string{
			argsNginxConf,
			argsConfigFileDir,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    SnapDaemonVolumeMounts(snapConfig),
		SecurityContext: &v1.SecurityContext{
			Privileged:             &privileged,
			RunAsUser:              &runAsUser,
			RunAsNonRoot:           &runAsNonRoot,
			ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		},
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(port),
				HostPort:      int32(port),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "dummy-port",
				ContainerPort: int32(dummyPort),
				HostPort:      int32(dummyPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "proxy-port",
				ContainerPort: int32(proxyPort),
				HostPort:      int32(proxyPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
