package chunkserver

import (
	"strconv"
	"strings"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PrepareJobName         = "prepare-chunkfile"
	DEFAULT_CHUNKFILE_SIZE = 16 * 1024 * 1024 // 16MB

	formatConfigMapName     = "format-chunkfilepool-conf"
	formatScriptFileDataKey = "format.sh"
	formatScriptMountPath   = "/curvebs/tools/sbin/format.sh"
)

func (c *Cluster) startProvisioningOverNodes() error {
	if !c.spec.Storage.UseSelectedNodes {
		hostnameMap, err := k8sutil.GetNodeHostNames(c.context.Clientset)
		if err != nil {
			return errors.Wrap(err, "failed to get node hostnames")
		}

		var storageNodes []string
		for _, nodeName := range c.spec.Storage.Nodes {
			storageNodes = append(storageNodes, hostnameMap[nodeName])
		}

		// get valid nodes that ready status and is schedulable
		validNodes, err := k8sutil.GetValidNodes(c.context, storageNodes)
		if err != nil {
			return errors.Wrap(err, "failed to valid spec filed nodes")
		}

		log.Infof("%d of the %d storage nodes are valid", len(validNodes), len(c.spec.Storage.Nodes))

		err = c.createConfigMap()
		if err != nil {
			return err
		}

		// travel all valid nodes to start job to prepare chunkfilepool
		for _, node := range validNodes {
			for _, device := range c.spec.Storage.Devices {
				err = c.runPrepareJob(node.Name, device)
				if err != nil {
					log.Errorf("failed to create job for device %s on %s", device.Name, node.Name)
					return errors.Wrapf(err, "failed to create job for device %s on %s", device.Name, node.Name)
				}
				log.Infof("create job for device %s on %s", device.Name, node.Name)
			}
		}
	}

	return nil
}

// createConfigMap create etcd configmap for etcd server
func (c *Cluster) createConfigMap() error {
	// generate configmap data with only one key of "format.sh"
	etcdConfigMap := map[string]string{
		formatScriptFileDataKey: FORMAT,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      formatConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: etcdConfigMap,
	}

	// Create format.sh configmap in cluster
	_, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create override configmap %s", c.namespacedName.Namespace)
	}
	return nil
}

// runPrepareJob create job and run job
func (c *Cluster) runPrepareJob(nodeName string, device curvev1.DevicesSpec) error {
	job, err := c.makeJob(nodeName, device)
	if err != nil {
		return errors.Wrapf(err, "failed to create prepare job for %s", nodeName)
	}

	existingJob, err := c.context.Clientset.BatchV1().Jobs(job.Namespace).Get(job.Name, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		log.Warningf("failed to detect job %s. %+v", job.Name, err)
	} else if err == nil {
		// if the job is still running
		if existingJob.Status.Active > 0 {
			log.Infof("Found previous job %s. Status=%+v", job.Name, existingJob.Status)
			return nil
		}
	}
	_, err = c.context.Clientset.BatchV1().Jobs(job.Namespace).Create(job)
	return err
}

func (c *Cluster) makeJob(nodeName string, device curvev1.DevicesSpec) (*batch.Job, error) {
	volumes, volumeMounts := c.createDevVolumeAndMount()

	name := strings.TrimSpace(device.Name)
	name = strings.TrimRight(name, "/")
	nameArr := strings.Split(name, "/")
	name = nameArr[len(nameArr)-1]

	jobName := PrepareJobName + "-" + nodeName + "-" + name
	podName := PrepareJobName + "-" + nodeName

	runAsUser := int64(0)
	runAsNonRoot := false

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: c.getPodLabels(nodeName),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeFormatContainer(device, volumeMounts),
			},
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyOnFailure,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:    &runAsUser,
				RunAsNonRoot: &runAsNonRoot,
			},
		},
	}

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: c.namespacedName.Namespace,
			Labels:    c.getPodLabels(nodeName),
		},
		Spec: batch.JobSpec{
			Template: podSpec,
		},
	}
	return job, nil
}

func (c *Cluster) makeFormatContainer(device curvev1.DevicesSpec, volumeMounts []v1.VolumeMount) v1.Container {
	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	// erase last '/' of mountpath
	chunkfileDir := strings.TrimRight(device.MountPath, "/")

	argsPercent := strconv.Itoa(device.Percentage)
	argsFileSize := strconv.Itoa(DEFAULT_CHUNKFILE_SIZE)
	argsFilePoolDir := chunkfileDir + "/chunkfilepool"
	argsFilePoolMetaPath := chunkfileDir + "/chunkfilepool.meta"

	container := v1.Container{
		Name: "format",
		Args: []string{
			device.Name,
			device.MountPath,
			argsPercent,
			argsFileSize,
			argsFilePoolDir,
			argsFilePoolMetaPath,
		},
		Command: []string{
			"/bin/bash",
			formatScriptMountPath,
		},
		Image:           c.spec.CurveVersion.Image,
		ImagePullPolicy: c.spec.CurveVersion.ImagePullPolicy,
		VolumeMounts:    volumeMounts,
		SecurityContext: &v1.SecurityContext{
			Privileged:             &privileged,
			RunAsUser:              &runAsUser,
			RunAsNonRoot:           &runAsNonRoot,
			ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		},
	}

	return container
}
