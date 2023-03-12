package mds

import (
	"fmt"
	"path"
	"strconv"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createConfigMap create mds configmap for mds server
func (c *Cluster) createConfigMap(etcd_endpoints string) (string, error) {
	configMapData, err := k8sutil.ReadConfFromTemplate("pkg/template/mds.conf")
	if err != nil {
		return "", errors.Wrap(err, "failed to read config file from template/mds.conf")
	}

	// modify part field config
	configMapData["mds.listen.addr"] = "127.0.0.1" + ":" + strconv.Itoa(c.spec.Mds.Port)
	configMapData["mds.dummy.listen.port"] = strconv.Itoa(c.spec.Mds.DummyPort)
	configMapData["global.port"] = strconv.Itoa(c.spec.Mds.Port)
	configMapData["mds.etcd.endpoint"] = etcd_endpoints
	configMapData["mds.snapshotcloneclient.addr"] = ""
	configMapData["mds.common.logDir"] = "/curvebs/mds/logs"

	curConfigMapName := configMapName

	// generate configmap data with only one key of "mds.conf"
	var mdsConfigVal string
	for k, v := range configMapData {
		mdsConfigVal = mdsConfigVal + k + "=" + v + "\n"
	}

	mdsConfigMap := map[string]string{
		config.MdsConfigMapDataKey: mdsConfigVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      curConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: mdsConfigMap,
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// Create mds config in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return "", errors.Wrapf(err, "failed to create mds configmap %s", c.namespacedName.Namespace)
	}

	return curConfigMapName, nil
}

// makeDeployment make etcd deployment to run etcd server
func (c *Cluster) makeDeployment(configMapDataKey string, configMapMountPathDir string, nodeName string, mdsConfig *mdsConfig, curConfigMapName string) (*apps.Deployment, error) {
	// TODO:
	volumes := daemon.DaemonVolumes(configMapDataKey, configMapMountPathDir, mdsConfig.DataPathMap, curConfigMapName)

	// for debug
	// log.Infof("mdsConfig %+v", mdsConfig)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   mdsConfig.ResourceName,
			Labels: c.getPodLabels(mdsConfig),
		},
		Spec: v1.PodSpec{
			// InitContainers: []v1.Container{
			// 	c.makeChmodDirInitContainer(configMapDataKey, configMapMountPathDir, mdsConfig, curConfigMapName),
			// },
			Containers: []v1.Container{
				c.makeMdsDaemonContainer(configMapDataKey, configMapMountPathDir, mdsConfig, curConfigMapName),
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
			Name:      mdsConfig.ResourceName,
			Namespace: c.namespacedName.Namespace,
			Labels:    c.getPodLabels(mdsConfig),
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: c.getPodLabels(mdsConfig),
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

// makeMdsDaemonContainer create mds container
func (c *Cluster) makeMdsDaemonContainer(configMapDataKey string, configMapMountPathDir string, mdsConfig *mdsConfig, curConfigMapName string) v1.Container {
	configFileMountPath := path.Join(config.MdsConfigMapMountPathDir, "mds.conf")
	argsConfigFileDir := fmt.Sprintf("--confPath=%s", configFileMountPath)

	container := v1.Container{
		Name: "mds",
		Command: []string{
			"/curvebs/mds/sbin/curvebs-mds",
		},
		Args:            []string{argsConfigFileDir},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(configMapDataKey, configMapMountPathDir, mdsConfig.DataPathMap, curConfigMapName),
		Ports: []v1.ContainerPort{
			{
				Name:          "server-port",
				ContainerPort: int32(c.spec.Mds.Port),
				HostPort:      int32(c.spec.Mds.Port),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "listen-port",
				ContainerPort: int32(c.spec.Mds.DummyPort),
				HostPort:      int32(c.spec.Mds.DummyPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
