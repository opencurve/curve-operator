package etcd

import (
	"fmt"
	"strconv"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createOverrideConfigMap create configMap override to record the endpoints of etcd for mds use
func (c *Cluster) createOverrideConfigMap(etcd_endpoints string) error {
	etcdConfigMapData := map[string]string{
		config.EtcdOvverideConfigMapDataKey: etcd_endpoints,
	}

	// etcd-endpoints-override configmap only has one "etcdEndpoints" key that the value is etcd cluster endpoints
	overrideCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EtcdOverrideConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: etcdConfigMapData,
	}

	_, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(overrideCM)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create override configmap %s", c.namespacedName.Namespace)
		}
		log.Infof("ConfigMap for override etcd endpoints %s already exists. updating if needed", config.EtcdOverrideConfigMapName)

		// TODO:Update the daemon Deployment
		// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
		// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
		// }
	} else {
		log.Infof("ConfigMap %s for override etcd endpoints has been created", config.EtcdOverrideConfigMapName)
	}

	return nil
}

// Comment it because we start etcd now using command line flag but not config file
// so etcd.conf is not exist in container(pod)
/**
func (c *Cluster) createEtcdConfigMap(etcd_endpoints string) error {
	var configMapData = map[string]string{
		"name":                        "default",
		"data-dir":                    ContainerDataDir,
		"wal-dir":                     ContainerDataDir + "/wal",
		"listen-peer-urls":            "http://0.0.0.0", // 23800
		"listen-client-urls":          "http://0.0.0.0", // 23790
		"initial-advertise-peer-urls": "http://",        // 23800
		"advertise-client-urls":       "http://",        // 23790
		"initial-cluster":             "",
	}

	// modify name
	// configMapData["name"] = nodeName

	// modify port perr-urls and client-urls by port that user setting
	configMapData["listen-peer-urls"] = configMapData["listen-peer-urls"] + ":" + strconv.Itoa(c.spec.Etcd.Port)
	configMapData["listen-client-urls"] = configMapData["listen-client-urls"] + ":" + strconv.Itoa(c.spec.Etcd.ListenPort)

	// modify port initial-advertise-peer-urls and advertise-client-urls by port that user setting
	configMapData["initial-advertise-peer-urls"] = configMapData["initial-advertise-peer-urls"] + ":" + strconv.Itoa(c.spec.Etcd.Port)
	configMapData["advertise-client-urls"] = configMapData["advertise-client-urls"] + ":" + strconv.Itoa(c.spec.Etcd.ListenPort)

	// modify initial-cluster field config
	configMapData["initial-cluster"] = etcd_endpoints

	// generate configmap data with only one key of "etcd.conf"
	var etcdConfigVal string
	for k, v := range configMapData {
		etcdConfigVal = etcdConfigVal + k + ": " + v + "\n"
	}

	etcdConfigMap := map[string]string{
		config.EtcdConfigMapDataKey: etcdConfigVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EtcdConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: etcdConfigMap,
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// Create etcd config in cluster
	_, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create override configmap %s", c.namespacedName.Namespace)
	}

	return nil
}
*/

// makeDeployment make etcd deployment to run etcd server
func (c *Cluster) makeDeployment(nodeName string, ip string, etcdConfig *etcdConfig, init_cluster string) (*apps.Deployment, error) {
	// TODO:
	volumes := daemon.DaemonVolumes("", "", etcdConfig.DataPathMap, "")

	// for debug
	// log.Infof("etcdConfig %+v", etcdConfig)

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
				c.makeEtcdDaemonContainer(nodeName, ip, etcdConfig, init_cluster),
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
		VolumeMounts:    daemon.DaemonVolumeMounts("", "", etcdConfig.DataPathMap, ""),
		Ports: []v1.ContainerPort{
			{
				Name:          "peer-port",
				ContainerPort: int32(c.spec.Etcd.Port),
				HostPort:      int32(c.spec.Etcd.Port),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "listen-port",
				ContainerPort: int32(c.spec.Etcd.ListenPort),
				HostPort:      int32(c.spec.Etcd.ListenPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}
	return container
}

// makeEtcdDaemonContainer create etcd container
func (c *Cluster) makeEtcdDaemonContainer(nodeName string, ip string, etcdConfig *etcdConfig, init_cluster string) v1.Container {
	// Start etcd using command line flag but not config file
	argsEtcdName := fmt.Sprintf("--name=%s", nodeName)

	argsDataDir := fmt.Sprintf("--data-dir=%s", ContainerDataDir)
	walDir := ContainerDataDir + "/wal"
	argsWalDir := fmt.Sprintf("--wal-dir=%s", walDir)

	argsLPU := fmt.Sprintf("--listen-peer-urls=http://%s:%s", ip, strconv.Itoa(c.spec.Etcd.Port))
	argsLCU := fmt.Sprintf("--listen-client-urls=http://%s:%s", ip, strconv.Itoa(c.spec.Etcd.ListenPort))
	argsInitialAPU := fmt.Sprintf("--initial-advertise-peer-urls=http://%s:%s", ip, strconv.Itoa(c.spec.Etcd.Port))
	argsACU := fmt.Sprintf("--advertise-client-urls=http://%s:%s", ip, strconv.Itoa(c.spec.Etcd.ListenPort))

	argsInitialCluster := fmt.Sprintf("--initial-cluster=%s", init_cluster)

	container := v1.Container{
		Name: "etcd",
		Command: []string{
			"/curvebs/etcd/sbin/etcd",
		},
		Args: []string{
			argsEtcdName,
			argsDataDir,
			argsWalDir,
			argsLPU,
			argsLCU,
			argsInitialAPU,
			argsACU,
			argsInitialCluster,
		},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts("", "", etcdConfig.DataPathMap, ""),
		Ports: []v1.ContainerPort{
			{
				Name:          "peer-port",
				ContainerPort: int32(c.spec.Etcd.Port),
				HostPort:      int32(c.spec.Etcd.Port),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "listen-port",
				ContainerPort: int32(c.spec.Etcd.ListenPort),
				HostPort:      int32(c.spec.Etcd.ListenPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
