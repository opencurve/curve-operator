package etcd

import (
	"fmt"
	"path"
	"strconv"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createConfigMap create etcd configmap for etcd server
func (c *Cluster) createConfigMap(daemonIDString string, nodeName string, ipAddr string, etcd_endpoints string) (string, error) {
	var configMapData = map[string]string{
		"name":                        "default",
		"data-dir":                    "/curvebs/etcd/data",
		"wal-dir":                     "/curvebs/etcd/data/wal",
		"listen-peer-urls":            "http://" + ipAddr, // 23800
		"listen-client-urls":          "http://" + ipAddr, // 23790
		"initial-advertise-peer-urls": "http://" + ipAddr, // 23800
		"advertise-client-urls":       "http://" + ipAddr, // 23790
		"initial-cluster":             "",
	}

	// modify name
	configMapData["name"] = nodeName

	// modify port perr-urls and client-urls by port that user setting
	configMapData["listen-peer-urls"] = configMapData["listen-peer-urls"] + ":" + strconv.Itoa(c.spec.Etcd.Port)
	configMapData["listen-client-urls"] = configMapData["listen-client-urls"] + ":" + strconv.Itoa(c.spec.Etcd.ListenPort)

	// modify port initial-advertise-peer-urls and advertise-client-urls by port that user setting
	configMapData["initial-advertise-peer-urls"] = configMapData["initial-advertise-peer-urls"] + ":" + strconv.Itoa(c.spec.Etcd.Port)
	configMapData["advertise-client-urls"] = configMapData["advertise-client-urls"] + ":" + strconv.Itoa(c.spec.Etcd.ListenPort)

	// modify initial-cluster field config
	configMapData["initial-cluster"] = etcd_endpoints

	curConfigMapName := fmt.Sprintf("%s-%s", configMapName, daemonIDString)

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
			Name:      curConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: etcdConfigMap,
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// Create etcd config in cluster
	_, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return "", errors.Wrapf(err, "failed to create override configmap %s", c.namespacedName.Namespace)
	}
	return curConfigMapName, nil
}

// makeDeployment make etcd deployment to run etcd server
func (c *Cluster) makeDeployment(configMapDataKey string, configMapMountPathDir string, nodeName string, etcdConfig *etcdConfig, curConfigMapName string) (*apps.Deployment, error) {
	// TODO:
	volumes := daemon.DaemonVolumes(configMapDataKey, configMapMountPathDir, etcdConfig.DataPathMap, curConfigMapName)

	// for debug
	// log.Infof("etcdConfig %+v", etcdConfig)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   etcdConfig.ResourceName,
			Labels: c.getPodLabels(etcdConfig),
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				c.makeChmodDirInitContainer(configMapDataKey, configMapMountPathDir, etcdConfig, curConfigMapName),
			},
			Containers: []v1.Container{
				c.makeEtcdDaemonContainer(configMapDataKey, configMapMountPathDir, etcdConfig, curConfigMapName),
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
func (c *Cluster) makeChmodDirInitContainer(configMapDataKey string, configMapMountPathDir string, etcdConfig *etcdConfig, curConfigMapName string) v1.Container {
	container := v1.Container{
		Name: "chmod",
		// Args:            args,
		Command:         []string{"chmod", "700", etcdConfig.DataPathMap.ContainerDataDir},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(configMapDataKey, configMapMountPathDir, etcdConfig.DataPathMap, curConfigMapName),
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
func (c *Cluster) makeEtcdDaemonContainer(configMapDataKey string, configMapMountPathDir string, etcdConfig *etcdConfig, curConfigMapName string) v1.Container {
	configFileMountPath := path.Join(config.EtcdConfigMapMountPathDir, "etcd.conf")
	argsConfigFileDir := fmt.Sprintf("--config-file=%s", configFileMountPath)

	container := v1.Container{
		Name: "etcd",
		Command: []string{
			"/curvebs/etcd/sbin/etcd",
		},
		Args:            []string{argsConfigFileDir},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(configMapDataKey, configMapMountPathDir, etcdConfig.DataPathMap, curConfigMapName),
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
