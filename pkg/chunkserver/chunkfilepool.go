package chunkserver

import (
	"fmt"
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

	formatConfigMapName     = "format-chunkfile-conf"
	formatScriptFileDataKey = "format.sh"
	formatScriptMountPath   = "/curvebs/tools/sbin/format.sh"
)

// global variables
var jobsArr []string
var chunkserverConfigs []chunkserverConfig

// startProvisioningOverNodes format device and provision chunk files
func (c *Cluster) startProvisioningOverNodes(nodeNameIP map[string]string) error {
	if !c.spec.Storage.UseSelectedNodes {

		// clear slice
		jobsArr = []string{}
		chunkserverConfigs = []chunkserverConfig{}

		hostnameMap, err := k8sutil.GetNodeHostNames(c.context.Clientset)
		if err != nil {
			log.Error("failed to get node hostnames")
			return errors.Wrap(err, "failed to get node hostnames")
		}

		var storageNodes []string
		for _, nodeName := range c.spec.Storage.Nodes {
			storageNodes = append(storageNodes, hostnameMap[nodeName])
		}

		// get valid nodes that ready status and is schedulable
		validNodes, _ := k8sutil.GetValidNodes(c.context, storageNodes)
		if len(validNodes) == 0 {
			log.Warningf("no valid nodes available to run osds on nodes in namespace %q", c.namespacedName.Namespace)
			return nil
		}

		log.Infof("%d of the %d storage nodes are valid", len(validNodes), len(c.spec.Storage.Nodes))

		err = c.createFormatConfigMap()
		if err != nil {
			log.Errorf("failed to create format configmap")
			return err
		}

		hostSequence := 0
		// travel all valid nodes to start job to prepare chunkfiles
		for _, node := range validNodes {
			nodeIP := nodeNameIP[node.Name]
			// port
			portBase := c.spec.Storage.Port
			// replicas number
			replicasSequence := 0

			for _, device := range c.spec.Storage.Devices {
				name := strings.TrimSpace(device.Name)
				name = strings.TrimRight(name, "/")
				nameArr := strings.Split(name, "/")
				name = nameArr[len(nameArr)-1]
				resourceName := fmt.Sprintf("%s-%s-%s", AppName, node.Name, name)

				job, err := c.runPrepareJob(node.Name, device)
				if err != nil {
					log.Errorf("failed to create job for device %s on %s", device.Name, node.Name)
					continue // do not record the failed job in jobsArr and do not create chunkserverConfig for this device
				}

				log.Infof("created job for device %s on %s", device.Name, node.Name)

				// jobsArr record all the job that have started, to determine whether the format is completed
				jobsArr = append(jobsArr, job.Name)

				// create chunkserver config for each device of every node
				chunkserverConfig := chunkserverConfig{
					ResourceName: resourceName,
					DataPathMap: &chunkserverDataPathMap{
						HostDevice:       device.Name,
						HostLogDir:       fmt.Sprint(c.spec.LogDirHostPath, "/chunkserver"),
						ContainerDataDir: ChunkserverContainerDataDir,
						ContainerLogDir:  ChunkserverContainerLogDir,
					},
					NodeName:         node.Name,
					NodeIP:           nodeIP,
					DeviceName:       device.Name,
					Port:             portBase,
					HostSequence:     hostSequence,
					ReplicasSequence: replicasSequence,
					Replicas:         len(c.spec.Storage.Devices),
				}
				chunkserverConfigs = append(chunkserverConfigs, chunkserverConfig)
				portBase++
				replicasSequence++
			}
			hostSequence++
		}
	}

	return nil
}

// createConfigMap create configmap to store format.sh script
func (c *Cluster) createFormatConfigMap() error {
	// create configmap data with only one key of "format.sh"
	formatConfigMapData := map[string]string{
		formatScriptFileDataKey: FORMAT,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      formatConfigMapName,
			Namespace: c.namespacedName.Namespace,
		},
		Data: formatConfigMapData,
	}

	// Create format.sh configmap in cluster
	_, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create override configmap %s", c.namespacedName.Namespace)
	}

	return nil
}

// runPrepareJob create job and run job
func (c *Cluster) runPrepareJob(nodeName string, device curvev1.DevicesSpec) (*batch.Job, error) {
	job, _ := c.makeJob(nodeName, device)

	// check whether prepare job is exist
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

	return job, err
}

func (c *Cluster) makeJob(nodeName string, device curvev1.DevicesSpec) (*batch.Job, error) {
	volumes, volumeMounts := c.createFormatVolumeAndMount(device)

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

	argsPercent := strconv.Itoa(device.Percentage)
	argsFileSize := strconv.Itoa(DEFAULT_CHUNKFILE_SIZE)
	argsFilePoolDir := ChunkserverContainerDataDir + "/chunkfilepool"
	argsFilePoolMetaPath := ChunkserverContainerDataDir + "/chunkfilepool.meta"

	container := v1.Container{
		Name: "format",
		Args: []string{
			device.Name,
			ChunkserverContainerDataDir,
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

func (c *Cluster) getPodLabels(nodeName string) map[string]string {
	labels := make(map[string]string)
	labels["app"] = PrepareJobName
	labels["node"] = nodeName
	labels["curve_cluster"] = c.namespacedName.Namespace
	return labels
}
