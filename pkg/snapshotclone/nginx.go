package snapshotclone

import (
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
)

func (c *Cluster) createNginxConfigMap(snapConfig *snapConfig) error {
	// 1. get mds-conf-template from cluster
	nginxCMTemplate, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.NginxCnonfigMapTemp, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get configmap %s from cluster", config.NginxCnonfigMapTemp)
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get configmap %s from cluster", config.NginxCnonfigMapTemp)
		}
		return errors.Wrapf(err, "failed to get configmap %s from cluster", config.NginxCnonfigMapTemp)
	}

	// 2. read configmap data (string)
	mdsCMData := nginxCMTemplate.Data[config.NginxConfigMapDataKey]
	// 3. replace ${} to specific parameters
	replacedNginxData, err := config.ReplaceConfigVars(mdsCMData, snapConfig)
	if err != nil {
		log.Error("failed to Replace mds config template to generate %s to start server.", snapConfig.CurrentConfigMapName)
		return errors.Wrap(err, "failed to Replace mds config template to generate a new mds configmap to start server.")
	}

	nginxConfigMap := map[string]string{
		config.NginxConfigMapDataKey: replacedNginxData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.NginxConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: nginxConfigMap,
	}

	err = c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to nginx.conf configmap %q", config.NginxConfigMapName)
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// create nginx configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create nginx configmap %s", c.namespacedName.Namespace)
	}

	return nil
}
