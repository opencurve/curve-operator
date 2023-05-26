package mds

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

// createOverrideMdsCM create mds-endpoints-override configmap to record mds endpoints
func (c *Cluster) createOverrideMdsCM(mdsEndpoints, clusterMdsDummyAddr, clusterMdsDummyPort string) error {

	mdsConfigMapData := map[string]string{
		config.MdsOvverideConfigMapDataKey: mdsEndpoints,
		config.ClusterMdsDummyAddr:         clusterMdsDummyAddr,
		config.ClusterMdsDummyPort:         clusterMdsDummyPort,
	}

	// create mds override configMap to record the endpoints of etcd
	mdsOverrideCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.MdsOverrideConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: mdsConfigMapData,
	}

	err := c.OwnerInfo.SetControllerReference(mdsOverrideCM)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to mds override configmap %q", config.MdsOverrideConfigMapName)
	}

	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(context.Background(), mdsOverrideCM, metav1.CreateOptions{})
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create override configmap %s", c.NamespacedName.Namespace)
		}
		logger.Infof("ConfigMap for override mds endpoints %s already exists. updating if needed", config.MdsOverrideConfigMapName)

		// TODO:Update the daemon Deployment
		// if err := updateDeploymentAndWait(c.Context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
		// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
		// }
	} else {
		logger.Infof("ConfigMap %s for override mds endpoints has been created", config.MdsOverrideConfigMapName)
	}

	return nil
}

// createConfigMap create mds configmap for mds server
func (c *Cluster) createMdsConfigMap(mdsConfig *mdsConfig) error {
	mdsCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(context.Background(), config.MdsConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.MdsConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.MdsConfigMapTemp)
	}

	// read configmap data (string)
	mdsCMData := mdsCMTemplate.Data[config.MdsConfigMapDataKey]
	// replace ${} to specific parameters
	replacedMdsData, err := config.ReplaceConfigVars(mdsCMData, mdsConfig)
	if err != nil {
		return errors.Wrap(err, "failed to Replace mds config template to generate a new mds configmap to start server.")
	}

	// create curve-mds-conf-[a,b,...] configmap for each one deployment
	mdsConfigMapData := map[string]string{
		config.MdsConfigMapDataKey: replacedMdsData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mdsConfig.CurrentConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: mdsConfigMapData,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to mds configmap %q", config.MdsConfigMapName)
	}

	// create mds configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create mds configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}

// makeDeployment make mds deployment to run mds daemon
func (c *Cluster) makeDeployment(nodeName string, nodeIP string, mdsConfig *mdsConfig) (*apps.Deployment, error) {
	volumes := daemon.DaemonVolumes(config.MdsConfigMapDataKey, mdsConfig.ConfigMapMountPath, mdsConfig.DataPathMap, mdsConfig.CurrentConfigMapName)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "mds", mdsConfig.DaemonID, c.Kind)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   mdsConfig.ResourceName,
			Labels: labels,
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
		return nil, errors.Wrapf(err, "failed to set owner reference to mon deployment %q", d.Name)
	}

	return d, nil
}

// makeMdsDaemonContainer create mds container
func (c *Cluster) makeMdsDaemonContainer(nodeIP string, mdsConfig *mdsConfig) v1.Container {
	port, _ := strconv.Atoi(mdsConfig.ServicePort)
	dummyPort, _ := strconv.Atoi(mdsConfig.ServiceDummyPort)
	var commandLine string
	if c.Kind == config.KIND_CURVEBS {
		commandLine = "/curvebs/mds/sbin/curvebs-mds"
	} else {
		commandLine = "/curvefs/mds/sbin/curvefs-mds"
	}

	configFileMountPath := path.Join(mdsConfig.ConfigMapMountPath, config.MdsConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--confPath=%s", configFileMountPath)

	container := v1.Container{
		Name: "mds",
		Command: []string{
			commandLine,
		},
		Args: []string{
			argsConfigFileDir,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    daemon.DaemonVolumeMounts(config.MdsConfigMapDataKey, mdsConfig.ConfigMapMountPath, mdsConfig.DataPathMap, mdsConfig.CurrentConfigMapName),
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
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
