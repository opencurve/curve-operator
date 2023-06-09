package etcd

import (
	"fmt"
	"path"
	"strconv"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
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
		return err
	}

	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(overrideCM)
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

// makeDeployment make etcd deployment to run etcd server
func (c *Cluster) makeDeployment(nodeName string, ip string, etcdConfig *etcdConfig) (*apps.Deployment, error) {
	volumes := daemon.DaemonVolumes(config.EtcdConfigMapDataKey, etcdConfig.ConfigMapMountPath, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "etcd", etcdConfig.DaemonID, c.Kind)

	initContainers := []v1.Container{c.makeChmodDirInitContainer(etcdConfig)}
	containers := []v1.Container{c.makeEtcdDaemonContainer(nodeName, ip, etcdConfig, etcdConfig.ClusterEtcdHttpAddr)}
	deploymentConfig := k8sutil.DeploymentConfig{Name: etcdConfig.ResourceName, NodeName: nodeName, Namespace: c.NamespacedName.Namespace,
		Labels: labels, Volumes: volumes, Containers: containers, InitContainers: initContainers}
	d, err := k8sutil.MakeDeployment(deploymentConfig)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// makeChmodDirInitContainer make init container to chmod 700 of ContainerDataDir('/curvebs/etcd/data')
func (c *Cluster) makeChmodDirInitContainer(etcdConfig *etcdConfig) v1.Container {
	container := v1.Container{
		Name: "chmod",
		// Args:            args,
		Command:         []string{"chmod", "700", etcdConfig.DataPathMap.ContainerDataDir},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(config.EtcdConfigMapDataKey, etcdConfig.ConfigMapMountPath, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName),
		Env:             []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}
	return container
}

// makeEtcdDaemonContainer create etcd container
func (c *Cluster) makeEtcdDaemonContainer(nodeName string, ip string, etcdConfig *etcdConfig, initCluster string) v1.Container {
	clientPort, _ := strconv.Atoi(etcdConfig.ServiceClientPort)
	peerPort, _ := strconv.Atoi(etcdConfig.ServicePort)
	var commandLine string
	if c.Kind == config.KIND_CURVEBS {
		commandLine = "/curvebs/etcd/sbin/etcd"
	} else {
		commandLine = "/curvefs/etcd/sbin/etcd"
	}

	configFileMountPath := path.Join(etcdConfig.ConfigMapMountPath, config.EtcdConfigMapDataKey)
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
		VolumeMounts:    daemon.DaemonVolumeMounts(config.EtcdConfigMapDataKey, etcdConfig.ConfigMapMountPath, etcdConfig.DataPathMap, etcdConfig.CurrentConfigMapName),
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
