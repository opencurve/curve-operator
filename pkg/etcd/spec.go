package etcd

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

// createOverrideConfigMap create configMap override to record the endpoints of etcd for mds use
func (c *Cluster) createOverrideConfigMap(etcdEndpoints string, clusterEtcdAddr string) error {
	etcdConfigMapData := map[string]string{
		config.EtcdOvverideConfigMapDataKey: etcdEndpoints,
		config.ClusterEtcdAddr:              clusterEtcdAddr,
	}

	// etcd-endpoints-override configmap only has one "etcdEndpoints" key that the value is etcd cluster endpoints
	overrideCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EtcdOverrideConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: etcdConfigMapData,
	}
	err := c.OwnerInfo.SetControllerReference(overrideCM)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to etcd override configmap %q", config.EtcdConfigMapName)
	}

	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(context.Background(), overrideCM, metav1.CreateOptions{})
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create override configmap %s", c.NamespacedName.Namespace)
		}
		logger.Infof("ConfigMap for override etcd endpoints %s already exists. updating if needed", config.EtcdOverrideConfigMapName)

		// TODO:Update the daemon Deployment
		// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
		// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
		// }
	} else {
		logger.Infof("ConfigMap %s for override etcd endpoints has been created", config.EtcdOverrideConfigMapName)
	}

	return nil
}

// createConfigMap create etcd configmap for etcd server
func (c *Cluster) createEtcdConfigMap(etcdConfig *etcdConfig) error {
	// get etcd-conf-template from cluster
	etcdCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(context.Background(), config.EtcdConfigTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap [ %s ] from cluster", config.MdsConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", config.MdsConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", config.MdsConfigMapTemp)
	}

	// read configmap data (string)
	etcdCMData := etcdCMTemplate.Data[config.EtcdConfigMapDataKey]
	// replace ${} to specific parameters
	EtcdConfigTemp, err := config.ReplaceConfigVars(etcdCMData, etcdConfig)
	if err != nil {
		return errors.Wrap(err, "failed to Replace etcd config template to generate a new etcd configmap to start server.")
	}

	// create curve-etcd-conf-[a,b,...] configmap for each one deployment
	etcdConfigMapData := map[string]string{
		config.EtcdConfigMapDataKey: EtcdConfigTemp,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdConfig.CurrentConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: etcdConfigMapData,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference for etcd configmap [ %v ]", etcdConfig.CurrentConfigMapName)
	}

	// create etcd configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create etcd configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}

// makeDeployment make etcd deployment to run etcd server
func (c *Cluster) makeDeployment(nodeName string, ip string, etcdConfig *etcdConfig) (*apps.Deployment, error) {
	volumes := daemon.DaemonVolumes(config.EtcdConfigMapDataKey, etcdConfig.ConfigMapMountPath, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "etcd", etcdConfig.DaemonID, c.Kind)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   etcdConfig.ResourceName,
			Labels: labels,
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				c.makeChmodDirInitContainer(etcdConfig),
			},
			Containers: []v1.Container{
				c.makeEtcdDaemonContainer(nodeName, ip, etcdConfig, etcdConfig.ClusterEtcdHttpAddr),
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
			Name:      etcdConfig.ResourceName,
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
		return nil, errors.Wrapf(err, "failed to set owner reference to etcd deployment %q", d.Name)
	}

	return d, nil
}

// makeChmodDirInitContainer make init container to chmod 700 of ContainerDataDir('/curvebs/etcd/data')
func (c *Cluster) makeChmodDirInitContainer(etcdConfig *etcdConfig) v1.Container {
	// clientPort, _ := strconv.Atoi(etcdConfig.ServiceClientPort)
	// peerPort, _ := strconv.Atoi(etcdConfig.ServicePort)
	container := v1.Container{
		Name: "chmod",
		// Args:            args,
		Command:         []string{"chmod", "700", etcdConfig.DataPathMap.ContainerDataDir},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(config.EtcdConfigMapDataKey, config.EtcdConfigMapMountPathDir, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName),
		Env:             []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: 9898,
				HostPort:      9898,
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "peer-port",
				ContainerPort: int32(9899),
				HostPort:      int32(9899),
				Protocol:      v1.ProtocolTCP,
			},
		},
	}
	return container
}

// makeEtcdDaemonContainer create etcd container
func (c *Cluster) makeEtcdDaemonContainer(nodeName string, ip string, etcdConfig *etcdConfig, init_cluster string) v1.Container {
	clientPort, _ := strconv.Atoi(etcdConfig.ServiceClientPort)
	peerPort, _ := strconv.Atoi(etcdConfig.ServicePort)
	var commandLine string
	if c.Kind == config.KIND_CURVEBS {
		commandLine = "/curvebs/etcd/sbin/etcd"
	} else {
		commandLine = "/curvefs/etcd/sbin/etcd"
	}

	configFileMountPath := path.Join(config.EtcdConfigMapMountPathDir, config.EtcdConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--config-file=%s", configFileMountPath)

	container := v1.Container{
		Name: "etcd",
		Command: []string{
			commandLine,
		},
		Args: []string{
			argsConfigFileDir,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(config.EtcdConfigMapDataKey, config.EtcdConfigMapMountPathDir, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName),
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(clientPort),
				HostPort:      int32(clientPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "peer-port",
				ContainerPort: int32(peerPort),
				HostPort:      int32(peerPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}
	return container
}
