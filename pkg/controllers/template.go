package controllers

import (
	"context"
	"io/ioutil"
	"path"
	"strings"

	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	ConfigHostPath  = "/curvebs/conf"
	ConfigMountPath = "/curvebs/tools"
	JobName         = "read-config"
)

var conf_type = [7]string{"mds.conf", "chunkserver.conf", "snapshotclone.conf", "s3.conf", "cs_client.conf", "snap_client.conf", "tools.conf"}

// createEachConfigMap
func (c *cluster) createEachConfigMap() error {
	rd, err := ioutil.ReadDir(ConfigHostPath)
	if err != nil {
		logger.Errorf("failed to read dir %s", ConfigHostPath)
		return errors.Wrapf(err, "failed to read dir %s", ConfigHostPath)
	}

	if len(rd) == 0 {
		logger.Errorf("Read none config file from %s is an error", ConfigHostPath)
		return errors.New("Read none file")
	}

	for _, fi := range rd {
		if !fi.IsDir() {
			name := fi.Name()

			if name == "nebd-client.conf" ||
				name == "nebd-server.conf" ||
				name == "py_client.conf" ||
				!strings.HasSuffix(name, ".conf") {
				continue
			}

			if name != "etcd.conf" && name != "s3.conf" && name != "chunkserver.conf" && name != "nginx.conf" {
				s := strings.Split(name, ".")
				if s[len(s)-1] != "conf" {
					continue
				}

				var configMapName string
				if len(strings.Split(s[0], "_")) > 1 {
					configMapName = strings.Split(s[0], "_")[0] + "-conf-template"
				} else {
					configMapName = s[0] + "-conf-template"
				}

				m, err := k8sutil.ReadConf(path.Join(ConfigHostPath, name))
				if err != nil {
					return err
				}
				logger.Info(name)

				// create configmap for each one configmap
				err = c.createConfigMap(configMapName, name, m, "=")
				if err != nil {
					return err
				}
			} else if name == "etcd.conf" {
				configMapName := config.EtcdConfigTemp
				data, err := k8sutil.ReadEtcdTypeConfig(path.Join(ConfigHostPath, name))
				if err != nil {
					return err
				}
				logger.Info(name)

				// create configmap for each etcd.conf configmap
				err = c.createConfigMap(configMapName, name, data, ":")
				if err != nil {
					return err
				}
			} else if name == "s3.conf" || name == "chunkserver.conf" {
				data, err := k8sutil.ReadConf(path.Join(ConfigHostPath, name))
				if err != nil {
					return err
				}
				logger.Info(name)

				var configMapName string
				if name == "s3.conf" {
					configMapName = config.S3ConfigMapTemp
				} else {
					configMapName = config.ChunkServerConfigMapTemp
				}

				// create configmap for s3 configmap
				err = c.createS3OrChunkserverConfigMap(configMapName, data)
				if err != nil {
					return err
				}
			} else if name == "nginx.conf" {
				data, err := k8sutil.ReadNginxConf(path.Join(ConfigHostPath, name))
				if err != nil {
					return err
				}

				logger.Info(name)

				err = c.createNginxTemplateConfigMap(data)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *cluster) createS3OrChunkserverConfigMap(configMapName string, data map[string]string) error {
	configMapData := make(map[string]string)
	for k, v := range data {
		configMapData[k] = v
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: configMapData,
	}

	err := c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to configmap %q", config.S3ConfigMapTemp)
	}

	// create configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", configMapName)
	}

	return nil
}

// createConfigMap create configmap template
func (c *cluster) createConfigMap(configMapName string, configMapDataKey string, configMapData map[string]string, delimiter string) error {
	// generate configmap data with only one key of "mds.conf"
	var configVal string
	for k, v := range configMapData {
		if delimiter == ":" {
			configVal = configVal + k + delimiter + " " + v + "\n"
		} else {
			configVal = configVal + k + delimiter + v + "\n"
		}
	}

	ConfigMapVal := map[string]string{
		configMapDataKey: configVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: ConfigMapVal,
	}

	err := c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to configmap %q", configMapName)
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// create configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", configMapName)
	}

	return nil
}

func (c *cluster) createNginxTemplateConfigMap(data string) error {
	ConfigMapVal := map[string]string{
		config.NginxConfigMapDataKey: data,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.NginxCnonfigMapTemp,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: ConfigMapVal,
	}

	err := c.ownerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to configmap %q", config.NginxCnonfigMapTemp)
	}

	// for debug
	// log.Infof("namespace=%v", c.namespacedName.Namespace)

	// create configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", config.NginxCnonfigMapTemp)
	}
	return nil
}

// makeReadConfJob
func (c *cluster) makeReadConfJob() (*batch.Job, error) {
	var nodeName string
	pods, err := c.context.Clientset.CoreV1().Pods(c.NameSpace).List(metav1.ListOptions{
		LabelSelector: "curve=operator",
	})
	if err != nil || len(pods.Items) != 1 {
		logger.Error("failed to get curve-operator pod information")
		// return &batch.Job{}, errors.Wrap(err, "failed to get curve-operator pod information")
		nodeName = c.Spec.Nodes[0]
	} else {
		nodeName = pods.Items[0].Spec.NodeName
	}

	logger.Infof("curve-operator has been scheduled to %v", nodeName)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   JobName,
			Labels: c.getReadConfigJobLabel(),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeReadConfContainer(),
			},
			// for test set
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyOnFailure,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       c.makeConfigHostPathVolume(),
		},
	}

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName,
			Namespace: c.NamespacedName.Namespace,
			Labels:    c.getReadConfigJobLabel(),
		},
		Spec: batch.JobSpec{
			Template: podSpec,
		},
	}

	// set ownerReference
	err = c.ownerInfo.SetControllerReference(job)
	if err != nil {
		return &batch.Job{}, errors.Wrapf(err, "failed to set owner reference to mon deployment %q", job.Name)
	}

	err = k8sutil.RunReplaceableJob(context.TODO(), c.context.Clientset, job, true)
	if err != nil {
		logger.Errorf("failed to run read config job %s", JobName)
		return &batch.Job{}, errors.Wrapf(err, "failed to run read config job %s", JobName)
	}

	logger.Infof("read config job %s has started", JobName)

	return job, nil
}

func (c *cluster) makeReadConfContainer() v1.Container {
	container := v1.Container{
		Name: "readconfig",
		Args: []string{
			"-c",
			"cp /curvebs/conf/* /curvebs/tools",
		},
		Command: []string{
			"/bin/bash",
		},
		Image:           c.Spec.CurveVersion.Image,
		ImagePullPolicy: c.Spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    c.makeConfigMountVolume(),
	}

	return container
}

func (c *cluster) makeConfigHostPathVolume() []v1.Volume {
	vols := []v1.Volume{}

	hostPathType := v1.HostPathDirectoryOrCreate
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: ConfigHostPath, Type: &hostPathType}}
	vols = append(vols, v1.Volume{Name: "conf-volume", VolumeSource: src})

	return vols
}

func (c *cluster) makeConfigMountVolume() []v1.VolumeMount {
	mounts := []v1.VolumeMount{}

	mounts = append(mounts, v1.VolumeMount{Name: "conf-volume", MountPath: ConfigMountPath})

	return mounts
}

func (c *cluster) getReadConfigJobLabel() map[string]string {
	labels := make(map[string]string)
	labels["app"] = JobName
	return labels
}
