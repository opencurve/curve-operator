package daemon

import (
	"emperror.dev/errors"
	"github.com/coreos/pkg/capnslog"
	"github.com/opencurve/curve-operator/pkg/config"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "daemon")

// CreateSpecRoleAllConfigMap create configmap of role to store all config need by start role server.
func (c *Cluster) CreateSpecRoleAllConfigMap(role, configMapName string) error {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: c.Namespace,
		},
		Data: map[string]string{
			"role": role,
		},
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Create(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to create configmap %q", configMapName)
	}

	return nil
}

// updateSpecRoleAllConfigMap update configmap of role to store all config need by start role server.
func (c *Cluster) UpdateSpecRoleAllConfigMap(configMapName, configMapDataKey, configMapDataVal string, conf config.ConfigInterface) error {
	var value string
	if configMapDataVal != "" || len(configMapDataVal) != 0 {
		value = configMapDataVal
	} else {
		// get curve-conf-default configmap from cluster
		defaultConfigMap, err := c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Get(config.DefaultConfigMapName, metav1.GetOptions{})
		if err != nil {
			logger.Errorf("failed to get configmap [ %s ] from cluster", config.DefaultConfigMapName)
			if kerrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", config.DefaultConfigMapName)
			}
			return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", config.DefaultConfigMapName)
		}

		defaultDataVal := defaultConfigMap.Data[configMapDataKey]
		// replace ${} to specific parameters
		value, err = config.ReplaceConfigVars(defaultDataVal, conf)
		if err != nil {
			return err
		}
	}

	// update the data of configmap 'chunkserver-all-config' or snapshot-all-config
	cm, err := c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap [ %s ] from cluster", configMapName)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", configMapName)
		}
		return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", configMapName)
	}
	data := cm.Data
	data[configMapDataKey] = value

	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Update(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to update configmap %q", configMapName)
	}
	logger.Infof("add key %q to configmap %q successed", configMapDataKey, configMapName)

	return nil
}

// createConfigMap create each configmap
func (c *Cluster) CreateEachConfigMap(configMapDataKey string, conf config.ConfigInterface, currentConfigMapName string) error {
	// get curve-conf-default configmap from cluster
	defaultConfigMap, err := c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Get(config.DefaultConfigMapName, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap [ %s ] from cluster", config.DefaultConfigMapName)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", config.DefaultConfigMapName)
		}
		return errors.Wrapf(err, "failed to get configmap [ %s ] from cluster", config.DefaultConfigMapName)
	}

	// get configmap data
	configData := defaultConfigMap.Data[configMapDataKey]
	// replace ${} to specific parameters
	replacedData, err := config.ReplaceConfigVars(configData, conf)
	if err != nil {
		return err
	}

	// create curve-(role)-conf-[a,b,...] configmap for each one deployment
	configMapData := map[string]string{
		configMapDataKey: replacedData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      currentConfigMapName,
			Namespace: c.Namespace,
		},
		Data: configMapData,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	// create configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", currentConfigMapName)
	}

	logger.Infof("create configmap %q successed", currentConfigMapName)

	return nil
}
