package controllers

import (
	"path"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

var BSConfigs = []string{
	"etcd.conf",
	"mds.conf",
	"chunkserver.conf",
	"snapshotclone.conf",
	"snap_client.conf",
	"cs_client.conf",
	"s3.conf",
	"nginx.conf",
	"tools.conf",
}

var FSConfigs = []string{
	"etcd.conf",
	"mds.conf",
	"metaserver.conf",
	"tools.conf",
}

var GrafanaDashboardsConfigs = []string{
	"all.yml",
	"chunkserver.json",
	"client.json",
	"etcd.json",
	"mds.json",
	"report.json",
	"snapshotcloneserver.json",
	"grafana.ini",
}

var FSGrafanaDashboardsConfigs = []string{
	"all.yml",
	"client.json",
	"etcd.json",
	"mds.json",
	"metaserver.json",
	"grafana.ini",
}

// getDefaultConfigMapData
func getDefaultConfigMapData(c *daemon.Cluster) (map[string]string, error) {
	labels := getReadConfigJobLabel(c)
	selector := k8sutil.GetLabelSelector(labels)
	pods, err := k8sutil.GetPodsByLabelSelector(c.Context.Clientset, c.Namespace, selector)
	if err != nil {
		return nil, err
	}

	if len(pods.Items) != 1 {
		return nil, errors.New("app=sync-config label matches no pods")
	}
	pod := pods.Items[0]
	// for debug
	logger.Infof("sync-config pod is %q", pod.Name)

	var configs []string
	var configPath string
	if c.Kind == config.KIND_CURVEBS {
		configs = BSConfigs
		configPath = "/curvebs/conf/"
	} else {
		configs = FSConfigs
		configPath = "/curvefs/conf/"
	}
	logger.Infof("current cluster kind is %q", c.Kind)
	logger.Infof("start syncing config from container %v", configs)

	configMapData := make(map[string]string)
	for _, name := range configs {
		configName := path.Join(configPath, name)
		content, err := readConfigFromContainer(c, pod, configName)
		if err != nil {
			return nil, err
		}
		configMapData[name] = content
	}

	return configMapData, nil
}

// createDefaultConfigMap
func createDefaultConfigMap(c *daemon.Cluster) error {
	configMapData, err := getDefaultConfigMapData(c)
	if err != nil {
		return err
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.DefaultConfigMapName,
			Namespace: c.Namespace,
		},
		Data: configMapData,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to configmap %q", config.DefaultConfigMapName)
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// create configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", config.DefaultConfigMapName)
	}

	logger.Infof("create configmap %q successed", config.DefaultConfigMapName)
	return nil
}

// createGrafanaConfigMapTemplate copy grafana dashborads source to grafana container
func createGrafanaConfigMapTemplate(c *daemon.Cluster) error {
	labels := getReadConfigJobLabel(c)
	selector := k8sutil.GetLabelSelector(labels)
	pods, err := k8sutil.GetPodsByLabelSelector(c.Context.Clientset, c.Namespace, selector)
	if err != nil {
		return err
	}

	if len(pods.Items) != 1 {
		return errors.New("app=sync-config label matches no pods")
	}
	pod := pods.Items[0]

	configMapData := make(map[string]string)

	var pathPrefix string
	var dashboards []string
	if c.Kind == config.KIND_CURVEBS {
		pathPrefix = "/curvebs/monitor/grafana"
		dashboards = GrafanaDashboardsConfigs
	} else {
		pathPrefix = "/curvefs/monitor/grafana"
		dashboards = FSGrafanaDashboardsConfigs
	}

	for _, name := range dashboards {
		configPath := pathPrefix
		if name != "grafana.ini" {
			configPath = path.Join(pathPrefix, "/provisioning/dashboards")
		}
		configPath = path.Join(configPath, name)
		content, err := readConfigFromContainer(c, pod, configPath)
		if err != nil {
			return err
		}

		configMapData[name] = content
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.GrafanaDashboardsTemp,
			Namespace: c.Namespace,
		},
		Data: configMapData,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to configmap %q", config.GrafanaDashboardsTemp)
	}

	// create configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", config.GrafanaDashboardsTemp)
	}

	return nil
}
