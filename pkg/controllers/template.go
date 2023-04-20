package controllers

import (
	"io/ioutil"
	"path"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

// createEachConfigMap
func createEachConfigMap(c *daemon.Cluster) error {
	configHostPath := c.ConfDirHostPath
	rd, err := ioutil.ReadDir(configHostPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read dir %s", configHostPath)
	}
	if len(rd) == 0 {
		return errors.Errorf("read none file from %v", configHostPath)
	}

	for _, fi := range rd {
		if fi.IsDir() {
			continue
		}

		name := fi.Name()
		if name == "nebd-client.conf" ||
			name == "nebd-server.conf" ||
			name == "py_client.conf" ||
			!strings.HasSuffix(name, ".conf") {
			continue
		}
		logger.Info(name)

		data := make(map[string]string)
		var configMapName, delimiter string
		switch name {
		case "etcd.conf":
			configMapName = config.EtcdConfigTemp
			delimiter = ":"
			data, err = k8sutil.ReadEtcdTypeConfig(path.Join(configHostPath, name))
			if err != nil {
				return err
			}
		case "s3.conf", "chunkserver.conf":
			if name == "s3.conf" {
				configMapName = config.S3ConfigMapTemp
			} else {
				configMapName = config.ChunkServerConfigMapTemp
			}
			delimiter = ""
			data, err = k8sutil.ReadConf(path.Join(configHostPath, name))
			if err != nil {
				return err
			}
		case "nginx.conf":
			configMapName = config.NginxCnonfigMapTemp
			delimiter = ""
			data, err = k8sutil.ReadNginxConf(path.Join(configHostPath, name))
			if err != nil {
				return err
			}
		default:
			s := strings.Split(name, ".")
			if s[len(s)-1] != "conf" {
				continue
			}

			tmpStr := "-conf-template"
			if len(strings.Split(s[0], "_")) > 1 {
				configMapName = strings.Split(s[0], "_")[0] + tmpStr
			} else {
				configMapName = s[0] + tmpStr
			}
			delimiter = "="

			data, err = k8sutil.ReadConf(path.Join(configHostPath, name))
			if err != nil {
				return err
			}
		}

		// create configmap for each one configmap
		err = createConfigMap(c, configMapName, name, data, delimiter)
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
	if configMapDataKey != "s3.conf" && configMapDataKey != "chunkserver.conf" {
		if configMapDataKey == "nginx.conf" && delimiter == "" {
			configMapVal = data["nginx.conf"]
		} else {
			for k, v := range data {
				if delimiter == ":" { // for etcd.conf
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
