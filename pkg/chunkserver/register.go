package chunkserver

import (
	"fmt"
	"path"

	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
)

const RegisterJobName = "register-topo"

// runCreatePoolJob create Job to register topology.json
func (c *Cluster) runCreatePoolJob(nodeNameIP map[string]string, poolType string) (*batch.Job, error) {
	// 1. create topology-json-conf configmap in cluster
	err := c.createTopoConfigMap()
	if err != nil {
		return &batch.Job{}, errors.Wrap(err, "failed to create topology-json-conf configmap in cluster")
	}
	logger.Infof("created ConfigMap %s success", config.TopoJsonConfigMapName)

	// 2. create tool-conf configmap in cluster
	err = c.createToolConfigMap(nodeNameIP)
	if err != nil {
		return &batch.Job{}, errors.Wrap(err, "failed to create tool-conf configmap in cluster")
	}
	logger.Infof("created ConfigMap %s success", config.ToolsConfigMapName)

	// 3. make job to register topology.json to curve cluster
	job := &batch.Job{}
	if poolType == "physical_pool" {
		job, _ = c.makeCreatePoolJob(poolType, "gen-physical-pool")
	} else if poolType == "logical_pool" {
		job, _ = c.makeCreatePoolJob(poolType, "gen-logical-pool")
	}

	// check whether job is exist
	existingJob, err := c.Context.Clientset.BatchV1().Jobs(job.Namespace).Get(job.Name, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		logger.Warningf("failed to detect job %s. %+v", job.Name, err)
	} else if err == nil {
		// if the job is still running
		if existingJob.Status.Active > 0 {
			logger.Infof("Found previous job %s. Status=%+v", job.Name, existingJob.Status)
			return existingJob, nil
		}
	}

	// job is not found or job is not active status, so create or recreate it here
	_, err = c.Context.Clientset.BatchV1().Jobs(job.Namespace).Create(job)

	logger.Infof("creaded job to generate %s", poolType)

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
			Namespace: c.NamespacedName.Namespace,
			Labels:    c.getRegisterJobLabel(poolType),
		},
		Spec: batch.JobSpec{
			Template: podSpec,
		},
	}

	// set ownerReference
	err := c.OwnerInfo.SetControllerReference(job)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set owner reference to mon deployment %q", job.Name)
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
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
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
			Namespace: c.NamespacedName.Namespace,
		},
		Data: topoConfigMap,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to topology.json configmap %q", config.TopoJsonConfigMapName)
	}

	// Create topology-json-conf configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create topology-json-conf configmap in namespace %s", c.NamespacedName.Namespace)
	}
	return nil
}

// create tools.conf configmap
func (c *Cluster) createToolConfigMap(nodeNameIP map[string]string) error {
	// 1. get mds-conf-template from cluster
	toolsCMTemplate, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.ToolsConfigMapTemp, metav1.GetOptions{})
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
		return errors.Wrap(err, "failed to Replace tools config template to generate a new mds configmap to start server.")
	}

	toolConfigMap := map[string]string{
		config.ToolsConfigMapDataKey: replacedToolsData,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ToolsConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: toolConfigMap,
	}

	err = c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to tools.conf configmap %q", config.ToolsConfigMapName)
	}

	// Create topology-json-conf configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create tools-conf configmap in namespace %s", c.NamespacedName.Namespace)
	}

	return nil
}
