package snapshotclone

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
)

func (c *Cluster) replaceNginxVar(snapEndpoints string) (string, error) {
	content, err := os.ReadFile("pkg/template/nginx.conf")
	if err != nil {
		log.Error("failed to read config file from template/nginx.conf")
		return "", errors.Wrap(err, "failed to read config file from template/nginx.conf")
	}
	nginxStr := string(content)

	// regexp replace
	regexpFix, err := regexp.Compile(`\$\{prefix\}`)
	if err != nil {
		return "", errors.Wrap(err, "failed to compile ${prefix}")
	}
	match := regexpFix.ReplaceAllString(nginxStr, "/curvebs/snapshotclone")
	// for test
	// log.Info(match)

	// 2.
	regexpAddr, err := regexp.Compile(`\$\{service_addr\}`)
	if err != nil {
		return "", errors.Wrap(err, "failed to compile ${service_addr}")
	}
	match = regexpAddr.ReplaceAllString(match, "127.0.0.1")

	// 3.
	regexpProxyPort, err := regexp.Compile(`\$\{service_proxy_port\}`)
	if err != nil {
		return "", errors.Wrap(err, "failed to compile ${service_proxy_port}")
	}
	match = regexpProxyPort.ReplaceAllString(match, strconv.Itoa(c.spec.SnapShotClone.ProxyPort))

	// 4.
	regexSnapshot, err := regexp.Compile(`\$\{cluster_snapshotclone_nginx_upstream\}`)
	if err != nil {
		return "", errors.Wrap(err, "failed to compile ${cluster_snapshotclone_nginx_upstream}")
	}
	match = regexSnapshot.ReplaceAllString(match, snapEndpoints)
	// log.Infof("%v", nginxStr)
	fmt.Println(match)

	return match, nil
}

func (c *Cluster) createNginxConfigMap(snapEndpoints string) error {
	nginxStr, err := c.replaceNginxVar(snapEndpoints)
	if err != nil {
		return errors.Wrap(err, "failed to replace nginx conf")
	}

	nginxConfigMap := map[string]string{
		config.NginxConfigMapDataKey: nginxStr,
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

func (c *Cluster) createStartSnapConfigMap() error {
	startSnapShotConfigMap := map[string]string{
		config.StartSnapConfigMapDataKey: START,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.StartSnapConfigMap,
			Namespace: c.namespacedName.Namespace,
		},
		Data: startSnapShotConfigMap,
	}

	err := c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to start_snapshot.sh configmap %q", config.StartSnapConfigMap)
	}
	// create nginx configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create start snapshotclone configmap %s", c.namespacedName.Namespace)
	}
	return nil
}
