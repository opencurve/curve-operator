package topology

import (
	"fmt"
	"path"

	"github.com/coreos/pkg/capnslog"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PHYSICAL_POOL     = "physical_pool"
	LOGICAL_POOL      = "logical_pool"
	JOB_PHYSICAL_POOL = "provision-physical-pool"
	JOB_LOGICAL_POOL  = "provision-logical-pool"
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "topology")

// RunCreatePoolJob create Job to register topology.json
func RunCreatePoolJob(c *daemon.Cluster, dcs []*DeployConfig, poolType string) (*batch.Job, error) {
	job := &batch.Job{}
	if poolType == PHYSICAL_POOL {
		job, _ = makeGenPoolJob(c, poolType, "provision-physical-pool")
	} else if poolType == LOGICAL_POOL {
		job, _ = makeGenPoolJob(c, poolType, "provision-logical-pool")
	}

	existingJob, err := c.Context.Clientset.BatchV1().Jobs(job.Namespace).Get(job.Name, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		logger.Warningf("failed to detect job %s. %+v", job.Name, err)
	} else if err == nil {
		if existingJob.Status.Active > 0 {
			logger.Infof("Found previous job %s. Status=%+v", job.Name, existingJob.Status)
			return existingJob, nil
		}
	}

	_, err = c.Context.Clientset.BatchV1().Jobs(job.Namespace).Create(job)
	logger.Infof("job created to generate %s", poolType)

	return &batch.Job{}, err
}

func makeGenPoolJob(c *daemon.Cluster, poolType string, jobName string) (*batch.Job, error) {
	// topology.json and tools.conf volume and volumemount
	volumes, mounts := CreateTopoAndToolVolumeAndMount(c)
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: getRegisterJobLabel(poolType),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				makeCreatePoolContainer(c, poolType, mounts),
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
			Namespace: c.Namespace,
			Labels:    getRegisterJobLabel(poolType),
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

func makeCreatePoolContainer(c *daemon.Cluster, poolType string, mounts []v1.VolumeMount) v1.Container {
	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	toolsBinaryPath := ""
	args := []string{}
	if c.Kind == config.KIND_CURVEBS {
		toolsBinaryPath = "/curvebs/tools/sbin/curvebs-tool"
		argsOp := ""
		if poolType == PHYSICAL_POOL {
			argsOp = fmt.Sprintf("-op=%s", "create_physicalpool")
		} else if poolType == LOGICAL_POOL {
			argsOp = fmt.Sprintf("-op=%s", "create_logicalpool")
		}
		args = append(args, argsOp)

		clusterMapPath := path.Join(config.TopoJsonConfigmapMountPathDir, config.TopoJsonConfigmapDataKey)
		argsClusterMap := fmt.Sprintf("-cluster_map=%s", clusterMapPath)
		args = append(args, argsClusterMap)
	} else {
		toolsBinaryPath = "/curvefs/tools/sbin/curvefs_tool"
		args = append(args, "create-topology")
	}

	container := v1.Container{
		Name: "pool",
		Args: args,
		Command: []string{
			toolsBinaryPath,
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

// CreateTopoConfigMap create topology configmap
func CreateTopoConfigMap(c *daemon.Cluster, dcs []*DeployConfig) error {
	// get topology.json string
	if len(dcs) == 0 {
		return errors.New("deployconfigs length is 0 tp create cluster pool")
	}
	clusterPoolJson := genClusterPool(dcs)
	topoConfigMap := map[string]string{
		config.TopoJsonConfigmapDataKey: clusterPoolJson,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.TopoJsonConfigMapName,
			Namespace: c.Namespace,
		},
		Data: topoConfigMap,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to topology.json configmap %q", config.TopoJsonConfigMapName)
	}

	// Create topology-json-conf configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func getRegisterJobLabel(poolType string) map[string]string {
	labels := make(map[string]string)
	labels["pool"] = poolType
	return labels
}
