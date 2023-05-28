package chunkserver

import (
	"strings"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Cluster) createToolConfigMap() error {
	// get mds-conf-template from cluster
	toolsCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Get(config.DefaultConfigMapName, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.DefaultConfigMapName)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.DefaultConfigMapName)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.DefaultConfigMapName)
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

func (c *Cluster) getS3ConfigMapData() (string, error) {
	s3CMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Get(config.DefaultConfigMapName, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get configmap %s from cluster", config.DefaultConfigMapName)
		if kerrors.IsNotFound(err) {
			return "", errors.Wrapf(err, "failed to get configmap %s from cluster", config.DefaultConfigMapName)
		}
		return "", errors.Wrapf(err, "failed to get configmap %s from cluster", config.DefaultConfigMapName)
	}

	s3Data := s3CMTemplate.Data[config.S3ConfigMapDataKey]
	s3MapData := translateS3StringToMap(s3Data)
	s3MapData["s3.ak"] = c.SnapShotClone.S3Config.AK
	s3MapData["s3.sk"] = c.SnapShotClone.S3Config.SK
	s3MapData["s3.nos_address"] = c.SnapShotClone.S3Config.NosAddress
	s3MapData["s3.snapshot_bucket_name"] = c.SnapShotClone.S3Config.SnapShotBucketName

	var configMapData string
	for k, v := range s3MapData {
		configMapData = configMapData + k + "=" + v + "\n"
	}

	return configMapData, nil
}

func translateS3StringToMap(data string) map[string]string {
	lines := strings.Split(data, "\n")
	config := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" { // skip the comment lines and blank lines
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // ignore invalid line
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		config[key] = value
	}
	return config

}
