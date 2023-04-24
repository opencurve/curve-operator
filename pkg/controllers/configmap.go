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

// createConfigMapTemplate create ConfigMap template after read config from sync deployment
func createConfigMapTemplate(c *daemon.Cluster) error {
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
	// for debug
	logger.Infof("sync-config pod is %q", pod.Name)

	var configs []string
	if c.Kind == config.KIND_CURVEBS {
		configs = BSConfigs
	} else {
		configs = FSConfigs
	}
	logger.Infof("current cluster kind is %q", c.Kind)
	logger.Infof("sync config from container %v", configs)

	for _, name := range configs {
		content, err := readConfigFromContainer(c, pod, name)
		if err != nil {
			return err
		}

		var delimiter string
		dataMap := make(map[string]string)
		configMapName := truncateConfigName(name)
		switch name {
		case "nginx.conf":
			dataMap, err = parseNginxConf(content)
			if err != nil {
				return err
			}
		default:
			if name == "etcd.conf" {
				delimiter = ":"
			} else {
				delimiter = "="
			}

			dataMap, err = parseConfigByDelimiter(content, delimiter)
			if err != nil {
				return err
			}
		}
		// create configmap for every one configmap
		err = createConfigMap(c, configMapName, name, dataMap, delimiter)
		if err != nil {
			return err
		}
	}
	return nil
}

// createConfigMap create configmap template
func createConfigMap(c *daemon.Cluster, configMapName string, configMapDataKey string, data map[string]string, delimiter string) error {
	var configMapVal string
	configMapData := make(map[string]string)
	if configMapDataKey != config.S3ConfigMapDataKey && configMapDataKey != config.ChunkserverConfigMapDataKey {
		if configMapDataKey == config.NginxConfigMapDataKey {
			configMapVal = data[config.NginxConfigMapDataKey]
		} else {
			for k, v := range data {
				if delimiter == ":" { // only for etcd.conf
					configMapVal = configMapVal + k + delimiter + " " + v + "\n"
				} else if delimiter == "=" {
					configMapVal = configMapVal + k + delimiter + v + "\n"
				} else {
					return errors.New("Unknown config file")
				}
			}
		}
		configMapData[configMapDataKey] = configMapVal
	} else {
		for k, v := range data {
			configMapData[k] = v
		}
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: configMapData,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to configmap %q", configMapName)
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// create configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", configMapName)
	}

	return nil
}
