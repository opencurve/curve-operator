package controllers

import (
	"context"

	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	ReadConfigJobName    = "read-config"
	ReadConfigVolumeName = "conf-volume"
	ConfigMountPath      = "/curvebs/tools"
	FSConfigMountPath    = "/curvefs/tools"
)

// makeReadConfJob
func makeReadConfJob(c *daemon.Cluster) (*batch.Job, error) {
	var nodeName string
	pods, err := c.Context.Clientset.CoreV1().Pods(c.Namespace).List(metav1.ListOptions{
		LabelSelector: "curve=operator",
	})
	if err != nil || len(pods.Items) != 1 {
		logger.Error("failed to get pod information by curve=operator label")
		// return &batch.Job{}, errors.Wrap(err, "failed to get curve-operator pod information")
		// for test
		// it will not appear because the operator must be scheduled to a certain node by kube-scheduler
		nodeName = c.Nodes[0]
	} else {
		nodeName = pods.Items[0].Spec.NodeName
	}
	logger.Infof("curve-operator has been scheduled to %q", nodeName)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ReadConfigJobName,
			Labels: getReadConfigJobLabel(c),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				makeReadConfContainer(c),
			},
			// for test set
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyOnFailure,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       makeConfigHostPathVolume(c),
		},
	}

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ReadConfigJobName,
			Namespace: c.NamespacedName.Namespace,
			Labels:    getReadConfigJobLabel(c),
		},
		Spec: batch.JobSpec{
			Template: podSpec,
		},
	}

	// set ownerReference
	err = c.OwnerInfo.SetControllerReference(job)
	if err != nil {
		return &batch.Job{}, err
	}

	err = k8sutil.RunReplaceableJob(context.TODO(), c.Context.Clientset, job, true)
	if err != nil {
		return &batch.Job{}, err
	}
	logger.Infof("starting read config job %q", job.GetName())

	return job, nil
}

func makeReadConfContainer(c *daemon.Cluster) v1.Container {
	commandLine := ""
	if c.Kind == config.KIND_CURVEBS {
		commandLine = "cp /curvebs/conf/* /curvebs/tools"
	} else if c.Kind == config.KIND_CURVEFS {
		commandLine = "cp /curvefs/conf/* /curvefs/tools"
	}
	container := v1.Container{
		Name: "readconfig",
		Args: []string{
			"-c",
			commandLine,
		},
		Command: []string{
			"/bin/bash",
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    makeConfigVolumeMount(c),
	}

	return container
}

func makeConfigHostPathVolume(c *daemon.Cluster) []v1.Volume {
	vols := []v1.Volume{}
	hostPathType := v1.HostPathDirectoryOrCreate
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: c.ConfDirHostPath, Type: &hostPathType}}
	vols = append(vols, v1.Volume{Name: ReadConfigVolumeName, VolumeSource: src})
	return vols
}

func makeConfigVolumeMount(c *daemon.Cluster) []v1.VolumeMount {
	var configMountPath string
	if c.Kind == config.KIND_CURVEBS {
		configMountPath = ConfigMountPath
	} else if c.Kind == config.KIND_CURVEFS {
		configMountPath = FSConfigMountPath
	}
	mounts := []v1.VolumeMount{}
	mounts = append(mounts, v1.VolumeMount{Name: ReadConfigVolumeName, MountPath: configMountPath})
	return mounts
}

func getReadConfigJobLabel(c *daemon.Cluster) map[string]string {
	labels := make(map[string]string)
	labels["app"] = ReadConfigJobName
	labels["curve"] = c.Kind
	return labels
}
