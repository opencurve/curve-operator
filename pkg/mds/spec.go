package mds

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

// createOverrideMdsCM create mds-endpoints-override configmap to record mds endpoints
func (c *Cluster) createOverrideMdsCM(nodeNameIP map[string]string) error {
	var mds_endpoints string
	for _, ipAddr := range nodeNameIP {
		mds_endpoints = fmt.Sprint(mds_endpoints, ipAddr, ":", c.spec.Mds.Port, ",")
	}
	mds_endpoints = strings.TrimRight(mds_endpoints, ",")

	mdsConfigMapData := map[string]string{
		config.MdsOvverideConfigMapDataKey: mds_endpoints,
	}

	// create mds override configMap to record the endpoints of etcd
	// mds-endpoints-override configmap only has one "mdsEndpoints" key that the value is mds cluster endpoints
	mdsOverrideCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.MdsOverrideConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: mdsConfigMapData,
	}

	err := c.ownerInfo.SetControllerReference(mdsOverrideCM)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to mds override configmap %q", config.MdsOverrideConfigMapName)
	}

	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(mdsOverrideCM)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create override configmap %s", c.namespacedName.Namespace)
		}
		log.Infof("ConfigMap for override mds endpoints %s already exists. updating if needed", config.MdsOverrideConfigMapName)

		// TODO:Update the daemon Deployment
		// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
		// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
		// }
	} else {
		log.Infof("ConfigMap %s for override mds endpoints has been created", config.MdsOverrideConfigMapName)
	}

	return nil
}

// createConfigMap create mds configmap for mds server
func (c *Cluster) createMdsConfigMap(etcd_endpoints string) error {
	configMapData, err := k8sutil.ReadConfFromTemplate("pkg/template/mds.conf")
	if err != nil {
		return errors.Wrap(err, "failed to read config file from template/mds.conf")
	}

	// modify part field of config file
	// configMapData["mds.listen.addr"] = "127.0.0.1" + ":" + strconv.Itoa(c.spec.Mds.Port)
	configMapData["mds.dummy.listen.port"] = strconv.Itoa(c.spec.Mds.DummyPort)
	configMapData["global.port"] = strconv.Itoa(c.spec.Mds.Port)
	configMapData["mds.etcd.endpoint"] = etcd_endpoints
	configMapData["mds.snapshotcloneclient.addr"] = ""
	configMapData["mds.common.logDir"] = ContainerLogDir

	curConfigMapName := config.MdsConfigMapName

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

	err = c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to mds configmap %q", config.MdsConfigMapName)
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// create mds configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create mds configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

// makeDeployment make mds deployment to run mds daemon
func (c *Cluster) makeDeployment(nodeName string, nodeIP string, mdsConfig *mdsConfig) (*apps.Deployment, error) {
	volumes := daemon.DaemonVolumes(config.MdsConfigMapDataKey, config.MdsConfigMapMountPathDir, mdsConfig.DataPathMap, config.MdsConfigMapName)

	// for debug
	// log.Infof("mdsConfig %+v", mdsConfig)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   mdsConfig.ResourceName,
			Labels: c.getPodLabels(mdsConfig),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeMdsDaemonContainer(nodeIP, mdsConfig),
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

	// set ownerReference
	err := c.ownerInfo.SetControllerReference(d)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set owner reference to mon deployment %q", d.Name)
	}

	return d, nil
}

// makeMdsDaemonContainer create mds container
func (c *Cluster) makeMdsDaemonContainer(nodeIP string, mdsConfig *mdsConfig) v1.Container {

	// define two args(--mdsAddr and --confPath) to start 'curvebs-mds'
	listenPort := strconv.Itoa(c.spec.Mds.Port)
	argsMdsAddr := fmt.Sprintf("--mdsAddr=%s:%s", nodeIP, listenPort)

	configFileMountPath := path.Join(config.MdsConfigMapMountPathDir, config.MdsConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--confPath=%s", configFileMountPath)

	container := v1.Container{
		Name: "mds",
		Command: []string{
			"/curvebs/mds/sbin/curvebs-mds",
		},
		Args: []string{
			argsMdsAddr,
			argsConfigFileDir,
		},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(config.MdsConfigMapDataKey, config.MdsConfigMapMountPathDir, mdsConfig.DataPathMap, config.MdsConfigMapName),
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(c.spec.Mds.Port),
				HostPort:      int32(c.spec.Mds.Port),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "dummy-port",
				ContainerPort: int32(c.spec.Mds.DummyPort),
				HostPort:      int32(c.spec.Mds.DummyPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
