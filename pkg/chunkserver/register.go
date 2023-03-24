package chunkserver

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RegisterJobName = "register-topo"

// runCreatePoolJob create Job to register topology.json
func (c *Cluster) runCreatePoolJob(nodeNameIP map[string]string, poolType string) (*batch.Job, error) {
	// 1. create topology-json-conf configmap in cluster
	err := c.createTopoConfigMap()
	if err != nil {
		return &batch.Job{}, errors.Wrap(err, "failed to create topology-json-conf configmap in cluster")
	}
	log.Infof("created ConfigMap %s success", config.TopoJsonConfigMapName)

	// 2. create tool-conf configmap in cluster
	err = c.createToolConfigMap(nodeNameIP)
	if err != nil {
		return &batch.Job{}, errors.Wrap(err, "failed to create tool-conf configmap in cluster")
	}
	log.Infof("created ConfigMap %s success", config.ToolsConfigMapName)

	// 3. make job to register topology.json to curve cluster
	job := &batch.Job{}
	if poolType == "physical_pool" {
		job, _ = c.makeCreatePoolJob(poolType, "gen-physical-pool")
	} else if poolType == "logical_pool" {
		job, _ = c.makeCreatePoolJob(poolType, "gen-logical-pool")
	}

	// check whether job is exist
	existingJob, err := c.context.Clientset.BatchV1().Jobs(job.Namespace).Get(job.Name, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		log.Warningf("failed to detect job %s. %+v", job.Name, err)
	} else if err == nil {
		// if the job is still running
		if existingJob.Status.Active > 0 {
			log.Infof("Found previous job %s. Status=%+v", job.Name, existingJob.Status)
			return existingJob, nil
		}
	}

	// job is not found or job is not active status, so create or recreate it here
	_, err = c.context.Clientset.BatchV1().Jobs(job.Namespace).Create(job)

	log.Infof("creaded job to generate %s", poolType)

	return &batch.Job{}, err
}

func (c *Cluster) makeCreatePoolJob(poolType string, jobName string) (*batch.Job, error) {
	// topology.json and tools.conf volume and volumemount
	volumes, mounts := c.createTopoAndToolVolumeAndMount()

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: c.getRegisterJobLabel(poolType),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeCreatePoolContainer(poolType, mounts),
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
		},
	}

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: c.namespacedName.Namespace,
			Labels:    c.getRegisterJobLabel(poolType),
		},
		Spec: batch.JobSpec{
			Template: podSpec,
		},
	}

	return job, nil
}

func (c *Cluster) makeCreatePoolContainer(poolType string, mounts []v1.VolumeMount) v1.Container {
	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	argsOp := ""
	if poolType == "physical_pool" {
		argsOp = fmt.Sprintf("-op=%s", "create_physicalpool")
	} else if poolType == "logical_pool" {
		argsOp = fmt.Sprintf("-op=%s", "create_logicalpool")
	}

	clusterMapPath := path.Join(config.TopoJsonConfigmapMountPathDir, config.TopoJsonConfigmapDataKey)
	argsClusterMap := fmt.Sprintf("-cluster_map=%s", clusterMapPath)

	container := v1.Container{
		Name: "format",
		Args: []string{
			argsOp,
			argsClusterMap,
			// "-c",
			// "while true; do echo hello; sleep 10;done",
		},
		Command: []string{
			"/curvebs/tools/sbin/curvebs-tool",
			// "/bin/sh",
		},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    mounts,
		SecurityContext: &v1.SecurityContext{
			Privileged:             &privileged,
			RunAsUser:              &runAsUser,
			RunAsNonRoot:           &runAsNonRoot,
			ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		},
	}

	return container
}

// createTopoConfigMap create topology configmap
func (c *Cluster) createTopoConfigMap() error {
	// get topology.json string
	clusterPoolJson := c.genClusterPool()

	topoConfigMap := map[string]string{
		config.TopoJsonConfigmapDataKey: clusterPoolJson,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.TopoJsonConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: topoConfigMap,
	}

	// Create topology-json-conf configmap in cluster
	_, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create topology-json-conf configmap in namespace %s", c.namespacedName.Namespace)
	}
	return nil
}

// create tools.conf configmap
func (c *Cluster) createToolConfigMap(nodeNameIP map[string]string) error {
	configMapData, err := k8sutil.ReadConfFromTemplate("pkg/template/tools.conf")
	if err != nil {
		return errors.Wrap(err, "failed to read config file from template/tools.conf")
	}

	etcdOverrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get etcd override endoints configmap")
	}
	etcdEndpoints := etcdOverrideCM.Data[config.EtcdOvverideConfigMapDataKey]

	mdsOverrideCM, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Get(config.MdsOverrideConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get mds override endoints configmap")
	}
	mdsEndpoints := mdsOverrideCM.Data[config.MdsOvverideConfigMapDataKey]

	configMapData["mdsAddr"] = mdsEndpoints

	// TODO: do not consider for only one host deployment here
	dummyPort := strconv.Itoa(c.spec.Mds.DummyPort)
	mdsDummyPorts := dummyPort + "," + dummyPort + "," + dummyPort
	configMapData["mdsDummyPort"] = mdsDummyPorts

	// TODO: do not consider for only one host deployment here
	retEtcdEndpoints := ""
	etcdEndpointsArr := strings.Split(etcdEndpoints, ",")
	for _, etcdAddr := range etcdEndpointsArr {
		ip := strings.Split(etcdAddr, ":")[0]
		retEtcdEndpoints += (ip + ":" + strconv.Itoa(c.spec.Etcd.ListenPort) + ",")
	}
	retEtcdEndpoints = strings.TrimRight(retEtcdEndpoints, ",")
	configMapData["etcdAddr"] = retEtcdEndpoints

	configMapData["snapshotCloneAddr"] = ""
	configMapData["snapshotCloneDummyPort"] = ""
	// TODO: do not consider for only one host deployment here
	var snapEndpoints string
	if c.spec.SnapShotClone.Enable {
		for _, ipAddr := range nodeNameIP {
			snapEndpoints = fmt.Sprint(snapEndpoints, ipAddr, ":", c.spec.SnapShotClone.Port, ",")
		}
		snapEndpoints = strings.TrimRight(snapEndpoints, ",")
		configMapData["snapshotCloneAddr"] = snapEndpoints
		dummyPort := strconv.Itoa(c.spec.SnapShotClone.DummyPort)
		configMapData["snapshotCloneDummyPort"] = fmt.Sprintf("%s,%s,%s", dummyPort, dummyPort, dummyPort)
	}

	var toolsConfigVal string
	for k, v := range configMapData {
		toolsConfigVal = toolsConfigVal + k + "=" + v + "\n"
	}

	// for debug
	log.Info(toolsConfigVal)

	topoConfigMap := map[string]string{
		config.ToolsConfigMapDataKey: toolsConfigVal,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ToolsConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: topoConfigMap,
	}

	// Create topology-json-conf configmap in cluster
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create tools-conf configmap in namespace %s", c.namespacedName.Namespace)
	}

	return nil
}
