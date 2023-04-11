package controllers

import (
	"context"

	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

const (
	ReadConfigJobName    = "read-config"
	ReadConfigVolumeName = "conf-volume"
	ConfigMountPath      = "/curvebs/tools"
)

// makeReadConfJob
func (c *cluster) makeReadConfJob() (*batch.Job, error) {
	var nodeName string
	pods, err := c.context.Clientset.CoreV1().Pods(c.NameSpace).List(metav1.ListOptions{
		LabelSelector: "curve=operator",
	})
	if err != nil || len(pods.Items) != 1 {
		logger.Error("failed to get pod information by curve=operator label")
		// return &batch.Job{}, errors.Wrap(err, "failed to get curve-operator pod information")
		// for test, it will not appear because the operator must be dispatched to a certain ground
		nodeName = c.Spec.Nodes[0]
	} else {
		nodeName = pods.Items[0].Spec.NodeName
	}
	logger.Infof("curve-operator has been scheduled to %q", nodeName)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ReadConfigJobName,
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
			Name:      ReadConfigJobName,
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
		return &batch.Job{}, errors.Wrapf(err, "failed to set owner reference to %q job", job.GetName())
	}

	err = k8sutil.RunReplaceableJob(context.TODO(), c.context.Clientset, job, true)
	if err != nil {
		return &batch.Job{}, errors.Wrapf(err, "failed to run read config job %s", job.GetName())
	}
	logger.Infof("starting read config job %q", job.GetName())

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
		VolumeMounts:    c.makeConfigVolumeMount(),
	}

	return container
}

func (c *cluster) makeConfigHostPathVolume() []v1.Volume {
	vols := []v1.Volume{}
	hostPathType := v1.HostPathDirectoryOrCreate
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: c.confDirHostPath, Type: &hostPathType}}
	vols = append(vols, v1.Volume{Name: ReadConfigVolumeName, VolumeSource: src})
	return vols
}

func (c *cluster) makeConfigVolumeMount() []v1.VolumeMount {
	mounts := []v1.VolumeMount{}
	mounts = append(mounts, v1.VolumeMount{Name: ReadConfigVolumeName, MountPath: ConfigMountPath})
	return mounts
}

func (c *cluster) getReadConfigJobLabel() map[string]string {
	labels := make(map[string]string)
	labels["app"] = ReadConfigJobName
	return labels
}
