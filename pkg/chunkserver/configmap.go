package chunkserver

import (
	"github.com/opencurve/curve-operator/pkg/chunkserver/script"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Cluster) createToolConfigMap() error {
	// get mds-conf-template from cluster
	toolsCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Get(config.ToolsConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.ToolsConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.ToolsConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.ToolsConfigMapTemp)
	}
	toolsCMData := toolsCMTemplate.Data[config.ToolsConfigMapDataKey]
	replacedToolsData, err := config.ReplaceConfigVars(toolsCMData, &chunkserverConfigs[0])
	if err != nil {
		return err
	}

	toolConfigMap := map[string]string{
		config.ToolsConfigMapDataKey: replacedToolsData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ToolsConfigMapName,
			Namespace: c.Namespace,
		},
		Data: toolConfigMap,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	// Create topology-json-conf configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create tools-conf configmap in namespace %s", c.Namespace)
	}

	return nil
}

// createCSClientConfigMap create cs_client configmap
func (c *Cluster) createCSClientConfigMap() error {
	// get mds-conf-template from cluster
	csClientCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.CsClientConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.CsClientConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.CsClientConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.CsClientConfigMapTemp)
	}

	// read configmap data (string)
	csClientCMData := csClientCMTemplate.Data[config.CSClientConfigMapDataKey]
	// replace ${} to specific parameters
	replacedCsClientData, err := config.ReplaceConfigVars(csClientCMData, &chunkserverConfigs[0])
	if err != nil {
		return err
	}

	csClientConfigMap := map[string]string{
		config.CSClientConfigMapDataKey: replacedCsClientData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.CSClientConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: csClientConfigMap,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	// Create cs_client configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create cs_client configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}

// CreateS3ConfigMap creates s3 configmap
func (c *Cluster) CreateS3ConfigMap() error {
	s3CMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.S3ConfigMapTemp, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.S3ConfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.S3ConfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.S3ConfigMapTemp)
	}

	data := s3CMTemplate.Data
	if c.SnapShotClone.Enable {
		data["s3.ak"] = c.SnapShotClone.S3Config.AK
		data["s3.sk"] = c.SnapShotClone.S3Config.SK
		data["s3.nos_address"] = c.SnapShotClone.S3Config.NosAddress
		data["s3.endpoint"] = c.SnapShotClone.S3Config.NosAddress
		data["s3.snapshot_bucket_name"] = c.SnapShotClone.S3Config.SnapShotBucketName
		data["s3.bucket_name"] = c.SnapShotClone.S3Config.SnapShotBucketName
	}

	var configMapData string
	for k, v := range data {
		configMapData = configMapData + k + "=" + v + "\n"
	}

	s3ConfigMap := map[string]string{
		config.S3ConfigMapDataKey: configMapData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.S3ConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: s3ConfigMap,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	// Create s3 configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create s3 configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}

// createConfigMap create configmap to run start_chunkserver.sh script
func (c *Cluster) createStartCSConfigMap() error {
	// generate configmap data with only one key of "format.sh"
	startCSConfigMap := map[string]string{
		startChunkserverScriptFileDataKey: script.START,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      startChunkserverConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: startCSConfigMap,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to cs.conf configmap %q", startChunkserverConfigMapName)
	}

	// Create format.sh configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create override configmap %s", c.NamespacedName.Namespace)
	}
	return nil
}
