package etcd

import (
	"fmt"
	"path"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
)

// createOverrideConfigMap create configMap override to record the endpoints of etcd for mds use
func (c *Cluster) createOverrideConfigMap(etcd_endpoints string, clusterEtcdAddr string) error {
	etcdConfigMapData := map[string]string{
		config.EtcdOvverideConfigMapDataKey: etcd_endpoints,
		config.ClusterEtcdAddr:              clusterEtcdAddr,
	}

	// etcd-endpoints-override configmap only has one "etcdEndpoints" key that the value is etcd cluster endpoints
	overrideCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EtcdOverrideConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: etcdConfigMapData,
	}
	err := c.ownerInfo.SetControllerReference(overrideCM)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to etcd override configmap %q", config.EtcdConfigMapName)
	}

	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(overrideCM)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create override configmap %s", c.namespacedName.Namespace)
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
	// 1. get etcd-conf-template from cluster
	etcdCMTemplate, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.EtcdConfigTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap [ %s ] from cluster", config.MdsConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", config.MdsConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", config.MdsConfigMapTemp)
	}

	// 2. read configmap data (string)
	etcdCMData := etcdCMTemplate.Data[config.EtcdConfigMapDataKey]
	// 3. replace ${} to specific parameters
	EtcdConfigTemp, err := config.ReplaceConfigVars(etcdCMData, etcdConfig)
	if err != nil {
		logger.Error("failed to Replace etcd config template to generate %s to start server.", etcdConfig.CurrentConfigMapName)
		return errors.Wrap(err, "failed to Replace etcd config template to generate a new etcd configmap to start server.")
	}

	// for debug
	// log.Info(replacedMdsData)

	// 4. create curve-etcd-conf-[a,b,...] configmap for each one deployment
	etcdConfigMapData := map[string]string{
		config.EtcdConfigMapDataKey: EtcdConfigTemp,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdConfig.CurrentConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: etcdConfigMapData,
	}

	err = c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		logger.Errorf("failed to set owner reference for etcd configmap [ %v ]", etcdConfig.CurrentConfigMapName)
		return errors.Wrapf(err, "failed to set owner reference for etcd configmap [ %v ]", etcdConfig.CurrentConfigMapName)
	}

	// 5. create etcd configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create etcd configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

// makeDeployment make etcd deployment to run etcd server
func (c *Cluster) makeDeployment(nodeName string, ip string, etcdConfig *etcdConfig) (*apps.Deployment, error) {
	volumes := daemon.DaemonVolumes(config.EtcdConfigMapDataKey, config.EtcdConfigMapMountPathDir, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName)

	// for debug
	// logger.Infof("etcdConfig %+v", etcdConfig)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   etcdConfig.ResourceName,
			Labels: c.getPodLabels(etcdConfig),
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
			Namespace: c.namespacedName.Namespace,
			Labels:    c.getPodLabels(etcdConfig),
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: c.getPodLabels(etcdConfig),
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
		return nil, errors.Wrapf(err, "failed to set owner reference to etcd deployment %q", d.Name)
	}

	return d, nil
}

// makeChmodDirInitContainer make init container to chmod 700 of ContainerDataDir('/curvebs/etcd/data')
func (c *Cluster) makeChmodDirInitContainer(etcdConfig *etcdConfig) v1.Container {
	container := v1.Container{
		Name: "chmod",
		// Args:            args,
		Command:         []string{"chmod", "700", etcdConfig.DataPathMap.ContainerDataDir},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(config.EtcdConfigMapDataKey, config.EtcdConfigMapMountPathDir, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName),
		Env:             []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(c.spec.Etcd.ClientPort),
				HostPort:      int32(c.spec.Etcd.ClientPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "peer-port",
				ContainerPort: int32(c.spec.Etcd.PeerPort),
				HostPort:      int32(c.spec.Etcd.PeerPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
	}
	return container
}

func (c *Cluster) makeReplaceVarContainer(nodeName string, ip string, etcdConfig *etcdConfig, init_cluster string) v1.Container {
	container := v1.Container{
		Name: "replace-variables",
		// Args:            args,
		Command:         []string{"chmod", "700", etcdConfig.DataPathMap.ContainerDataDir},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts("", "", etcdConfig.DataPathMap, ""),
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(c.spec.Etcd.ClientPort),
				HostPort:      int32(c.spec.Etcd.ClientPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "peer-port",
				ContainerPort: int32(c.spec.Etcd.PeerPort),
				HostPort:      int32(c.spec.Etcd.PeerPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}
	return container
}

// makeEtcdDaemonContainer create etcd container
func (c *Cluster) makeEtcdDaemonContainer(nodeName string, ip string, etcdConfig *etcdConfig, init_cluster string) v1.Container {
	configFileMountPath := path.Join(config.EtcdConfigMapMountPathDir, config.EtcdConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--config-file=%s", configFileMountPath)
	container := v1.Container{
		Name: "etcd",
		Command: []string{
			// "/bin/bash",
			"/curvebs/etcd/sbin/etcd",
		},
		Args: []string{
			argsConfigFileDir,
			// "-c",
			// "while true; do echo hello; sleep 10; done",
		},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(config.EtcdConfigMapDataKey, config.EtcdConfigMapMountPathDir, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName),
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(c.spec.Etcd.ClientPort),
				HostPort:      int32(c.spec.Etcd.ClientPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "peer-port",
				ContainerPort: int32(c.spec.Etcd.PeerPort),
				HostPort:      int32(c.spec.Etcd.PeerPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
