package snapshotclone

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createSnapClientConf
func (c *Cluster) createSnapClientConfigMap(mdsEndpoints string) error {
	configMapData, err := k8sutil.ReadConfFromTemplate("pkg/template/snap_client.conf")
	if err != nil {
		return errors.Wrap(err, "failed to read config file from template/snap_client.conf")
	}
	configMapData["mds.listen.addr"] = mdsEndpoints
	configMapData["global.logPath"] = ContainerLogDir

	var snapClientConfigVal string
	for k, v := range configMapData {
		snapClientConfigVal = snapClientConfigVal + k + "=" + v + "\n"
	}

	snapClientConfigMap := map[string]string{
		config.SnapClientConfigMapDataKey: snapClientConfigVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.SnapClientConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: snapClientConfigMap,
	}

	err = c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to snap_client.conf configmap %q", config.SnapClientConfigMapName)
	}

	// Create cs_client configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create snap_client configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

func (c *Cluster) createSnapShotCloneConfigMap(etcdEndpoints string) error {
	configMapData, err := k8sutil.ReadConfFromTemplate("pkg/template/snapshotclone.conf")
	if err != nil {
		return errors.Wrap(err, "failed to read config file from template/snapshotclone.conf")
	}
	configMapData["client.config_path"] = config.SnapClientConfigMapMountPath + "/snap_client.conf"
	configMapData["log.dir"] = ContainerLogDir
	configMapData["s3.config_path"] = config.S3ConfigMapMountSnapPathDir + "/s3.conf"
	// configMapData["server.address"] =
	configMapData["server.port"] = strconv.Itoa(c.spec.SnapShotClone.Port)
	var etcdListenAddr string
	s := strings.Split(etcdEndpoints, ",")
	for _, ipAddr := range s {
		ip := strings.Split(ipAddr, ":")[0]
		etcdListenAddr += ip + ":" + strconv.Itoa(c.spec.Etcd.ListenPort) + ","
	}
	etcdListenAddr = strings.TrimRight(etcdListenAddr, ",")
	// for test
	log.Info(etcdListenAddr)
	configMapData["etcd.endpoint"] = etcdListenAddr
	configMapData["server.dummy.listen.port"] = strconv.Itoa(c.spec.SnapShotClone.DummyPort)

	var snapCloneConfigVal string
	for k, v := range configMapData {
		snapCloneConfigVal = snapCloneConfigVal + k + "=" + v + "\n"
	}

	snapCloneConfigMap := map[string]string{
		config.SnapShotCloneConfigMapDataKey: snapCloneConfigVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.SnapShotCloneConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: snapCloneConfigMap,
	}

	err = c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to snapshotclone.conf configmap %q", config.SnapShotCloneConfigMapName)
	}

	// Create cs_client configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create snap_client configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

// func (c *Cluster) createNginxConfigMap() error {
// 	return nil
// }

// makeDeployment make snapshotclone deployment to run snapshotclone daemon
func (c *Cluster) makeDeployment(nodeName string, nodeIP string, snapConfig *snapConfig) (*apps.Deployment, error) {
	volumes := SnapDaemonVolumes(snapConfig.DataPathMap)

	// for debug
	// log.Infof("snapConfig %+v", snapConfig)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   snapConfig.ResourceName,
			Labels: c.getPodLabels(snapConfig),
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
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapConfig.ResourceName,
			Namespace: c.namespacedName.Namespace,
			Labels:    c.getPodLabels(snapConfig),
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: c.getPodLabels(snapConfig),
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

// makeSnapshotDaemonContainer create snapshotclone container
func (c *Cluster) makeSnapshotDaemonContainer(nodeIP string, snapConfig *snapConfig) v1.Container {

	argsNginxConf := path.Join(config.NginxConfigMapMountPath, config.NginxConfigMapDataKey)
	// define two args(--addr and --conf) to start 'curvebs-mds'
	listenPort := strconv.Itoa(c.spec.SnapShotClone.Port)
	argsSnapShotCloneAddr := fmt.Sprintf("--addr=%s:%s", nodeIP, listenPort)

	configFileMountPath := path.Join(config.SnapShotCloneConfigMapMountPath, config.SnapShotCloneConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--conf=%s", configFileMountPath)

	container := v1.Container{
		Name: "snapshotclone",
		Command: []string{
			"/bin/bash",
			config.StartSnapConfigMapMountPath,
		},
		Args: []string{
			argsNginxConf,
			argsSnapShotCloneAddr,
			argsConfigFileDir,
		},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    SnapDaemonVolumeMounts(snapConfig.DataPathMap),
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(c.spec.SnapShotClone.Port),
				HostPort:      int32(c.spec.SnapShotClone.Port),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "dummy-port",
				ContainerPort: int32(c.spec.SnapShotClone.DummyPort),
				HostPort:      int32(c.spec.SnapShotClone.DummyPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "proxy-port",
				ContainerPort: int32(c.spec.SnapShotClone.ProxyPort),
				HostPort:      int32(c.spec.SnapShotClone.ProxyPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
