package controllers

import (
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
	if c.Kind == config.KIND_CURVEBS {
		configs = BSConfigs
	} else {
		configs = FSConfigs
	}
	logger.Infof("current cluster kind is %q", c.Kind)
	logger.Infof("start syncing config from container %v", configs)

	configMapData := make(map[string]string)
	for _, name := range configs {
		content, err := readConfigFromContainer(c, pod, name)
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
