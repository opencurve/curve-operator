package metaserver

import (
	"fmt"
	"path"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/topology"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Cluster) createToolConfigMap(msConfigs []*metaserverConfig) error {
	// get mds-conf-template from cluster
	toolsCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Get(config.ToolsConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.ToolsConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.ToolsConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.ToolsConfigMapTemp)
	}
	toolsCMData := toolsCMTemplate.Data[config.ToolsConfigMapDataKey]
	replacedToolsData, err := config.ReplaceConfigVars(toolsCMData, msConfigs[0])
	if err != nil {
		return errors.Wrap(err, "failed to Replace tools config template to generate a new mds configmap to start server.")
	}

	toolConfigMap := map[string]string{
		config.ToolsConfigMapDataKey: replacedToolsData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ToolsConfigMapName,
			Namespace: c.Namespace,
		},
		Data: toolConfigMap,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	// Create topology-json-conf configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create tools-conf configmap in namespace %s", c.Namespace)
	}

	return nil
}

func (c *Cluster) createMetaserverConfigMap(metaserverConfig *metaserverConfig) error {
	metaserverCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.MetaserverConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.MetaserverConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.MetaserverConfigMapTemp)
	}

	metaserverCMData := metaserverCMTemplate.Data[config.MetaServerConfigMapDataKey]
	replacedMetaserverData, err := config.ReplaceConfigVars(metaserverCMData, metaserverConfig)
	if err != nil {
		return errors.Wrap(err, "failed to Replace metaserver config template to generate a new metaserver configmap to start server.")
	}

	msConfigMapData := map[string]string{
		config.MetaServerConfigMapDataKey: replacedMetaserverData,
	}
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metaserverConfig.CurrentConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: msConfigMapData,
	}
	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create mds configmap %s", c.NamespacedName.Namespace)
	}
	return nil
}

// makeDeployment make metaserver deployment to run mds daemon
func (c *Cluster) makeDeployment(metaserverConfig *metaserverConfig, nodeName string, nodeIP string) (*apps.Deployment, error) {
	volumes := daemon.DaemonVolumes(config.MetaServerConfigMapDataKey, config.MetaServerConfigMapMountPath, metaserverConfig.DataPathMap, metaserverConfig.CurrentConfigMapName)
	vols, _ := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volumes = append(volumes, vols...)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   metaserverConfig.ResourceName,
			Labels: c.getPodLabels(metaserverConfig),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeMSDaemonContainer(nodeIP, metaserverConfig),
			},
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metaserverConfig.ResourceName,
			Namespace: c.NamespacedName.Namespace,
			Labels:    c.getPodLabels(metaserverConfig),
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: c.getPodLabels(metaserverConfig),
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
		},
	}

	err := c.OwnerInfo.SetControllerReference(d)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// makeMdsDaemonContainer create mds container
func (c *Cluster) makeMSDaemonContainer(nodeIP string, metaserverConfig *metaserverConfig) v1.Container {
	configFileMountPath := path.Join(config.MetaServerConfigMapMountPath, config.MetaServerConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--confPath=%s", configFileMountPath)

	volMounts := daemon.DaemonVolumeMounts(config.MetaServerConfigMapDataKey, config.MetaServerConfigMapMountPath, metaserverConfig.DataPathMap, metaserverConfig.CurrentConfigMapName)
	_, mounts := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volMounts = append(volMounts, mounts...)

	container := v1.Container{
		Name: "metaserver",
		Command: []string{
			"/curvefs/metaserver/sbin/curvefs-metaserver",
		},
		Args: []string{
			argsConfigFileDir,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    volMounts,
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(c.Metaserver.Port),
				HostPort:      int32(c.Metaserver.Port),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "external-port",
				ContainerPort: int32(c.Metaserver.ExternalPort),
				HostPort:      int32(c.Metaserver.ExternalPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}

// getLabels Add labels for mds deployment
func (c *Cluster) getPodLabels(metaserverConfig *metaserverConfig) map[string]string {
	labels := make(map[string]string)
	labels["app"] = AppName
	labels["metaserver"] = metaserverConfig.DaemonID
	labels["curve_daemon_id"] = metaserverConfig.DaemonID
	labels["curve_cluster"] = c.NamespacedName.Namespace
	return labels
}
