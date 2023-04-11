package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/chunkserver"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/mds"
)

const (
	CleanupAppName                    = "curve-cleanup"
	clusterCleanUpPolicyRetryInterval = 5 //seconds

	dataVolumeName     = "data-cleanup-volume"
	dataDirHostPathEnv = "CURVE_DATA_DIR_HOST_PATH"
)

// startClusterCleanUp start job to clean hostpath
func (c *ClusterController) startClusterCleanUp(ctx clusterd.Context, cluster *curvev1.CurveCluster, nodesForJob []v1.Node) {
	if len(nodesForJob) == 0 {
		logger.Info("No nodes to cleanup")
		return
	}

	logger.Infof("starting clean up for cluster %q", cluster.Name)

	err := c.waitForCurveDaemonCleanUp(context.TODO(), cluster, time.Duration(clusterCleanUpPolicyRetryInterval)*time.Second)
	if err != nil {
		logger.Errorf("failed to wait till curve daemons are destroyed. %v", err)
		return
	}

	c.startCleanUpJobs(cluster, nodesForJob)
}

func (c *ClusterController) startCleanUpJobs(cluster *curvev1.CurveCluster, nodesForJob []v1.Node) {
	for _, node := range nodesForJob {
		logger.Infof("starting clean up job on node %q", node.Name)
		jobName := k8sutil.TruncateNodeNameForJob("cluster-cleanup-job-%s", node.Name)
		labels := getCleanupLabels("cleanup", c.namespacedName.Namespace)
		podSpec := c.cleanUpJobTemplateSpec(cluster)
		podSpec.Spec.NodeName = node.Name
		job := &batch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: cluster.Namespace,
				Labels:    labels,
			},
			Spec: batch.JobSpec{
				Template: podSpec,
			},
		}

		if err := k8sutil.RunReplaceableJob(context.TODO(), c.context.Clientset, job, true); err != nil {
			logger.Errorf("failed to run cluster clean up job on node %q. %v", node.Name, err)
		}

		logger.Infof("cleanup job %s has started", jobName)
	}
}

func (c *ClusterController) cleanUpJobTemplateSpec(cluster *curvev1.CurveCluster) v1.PodTemplateSpec {
	volumes := []v1.Volume{}
	dataHostPathVolume := v1.Volume{Name: dataVolumeName, VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: cluster.Spec.HostDataDir}}}
	volumes = append(volumes, dataHostPathVolume)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: CleanupAppName,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.cleanUpJobContainer(cluster),
			},
			Volumes:       volumes,
			RestartPolicy: v1.RestartPolicyOnFailure,
		},
	}

	return podSpec
}

func (c *ClusterController) cleanUpJobContainer(cluster *curvev1.CurveCluster) v1.Container {
	volumeMounts := []v1.VolumeMount{}
	envVars := []v1.EnvVar{}

	dataHhostPathVolumeMount := v1.VolumeMount{Name: dataVolumeName, MountPath: cluster.Spec.HostDataDir}
	volumeMounts = append(volumeMounts, dataHhostPathVolumeMount)

	securityContext := k8sutil.PrivilegedContext(true)

	envVars = append(envVars, []v1.EnvVar{
		{Name: dataDirHostPathEnv, Value: strings.TrimRight(cluster.Spec.HostDataDir, "/")},
	}...)

	commandLine := `rm -rf $(CURVE_DATA_DIR_HOST_PATH)/*;`
	return v1.Container{
		Name:            "host-cleanup",
		Image:           cluster.Spec.CurveVersion.Image,
		ImagePullPolicy: cluster.Spec.CurveVersion.ImagePullPolicy,
		Command: []string{
			"/bin/bash",
			"-c",
		},
		Args: []string{
			commandLine,
		},
		Env:             envVars,
		VolumeMounts:    volumeMounts,
		SecurityContext: securityContext,
	}
}

func (c *ClusterController) waitForCurveDaemonCleanUp(context context.Context, cluster *curvev1.CurveCluster, retryInterval time.Duration) error {
	logger.Infof("waiting for all the curve daemons to be cleaned up in the cluster %q", cluster.Namespace)
	// 3 minutes(5s * 60)
	maxRetryTime := 60
	retryCount := 0
	for {
		retryCount++
		select {
		case <-time.After(retryInterval):
			curveHosts, err := c.getCurveNodes(cluster.Namespace)
			if err != nil {
				return errors.Wrap(err, "failed to list curve daemon nodes")
			}

			if len(curveHosts) == 0 {
				logger.Info("all curve daemons are cleaned up")
				return nil
			}

			// always exit finally
			if retryCount > maxRetryTime {
				return errors.Errorf("cancelling the host cleanup job because of timeout")
			}

			logger.Debugf("waiting for curve daemons in cluster %q to be cleaned up. Retrying in %q",
				cluster.Namespace, retryInterval.String())
		case <-context.Done():
			return errors.Errorf("cancelling the host cleanup job. %s", context.Err())
		}
	}
}

// getCurveNodes get all the node names where curve daemons are running
func (c *ClusterController) getCurveNodes(namespace string) ([]string, error) {
	curveAppNames := []string{etcd.AppName, mds.AppName, chunkserver.AppName, chunkserver.PrepareJobName, chunkserver.RegisterJobName}
	nodeNameList := sets.NewString()
	hostNameList := []string{}
	var b strings.Builder

	for _, app := range curveAppNames {
		appLabelSelector := fmt.Sprintf("app=%s", app)
		podList, err := c.context.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: appLabelSelector})
		if err != nil {
			return hostNameList, errors.Wrapf(err, "could not list the %q pods", app)
		}

		for _, curvePod := range podList.Items {
			podNodeName := curvePod.Spec.NodeName
			if podNodeName != "" && !nodeNameList.Has(podNodeName) {
				nodeNameList.Insert(podNodeName)
			}
		}
		fmt.Fprintf(&b, "%s: %d. ", app, len(podList.Items))
	}

	logger.Infof("existing curve daemons in the namespace %q. %s", namespace, b.String())

	for nodeName := range nodeNameList {
		podHostName, err := k8sutil.GetNodeHostName(context.TODO(), c.context.Clientset, nodeName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get hostname from node %q", nodeName)
		}
		hostNameList = append(hostNameList, podHostName)
	}

	return hostNameList, nil
}

func getCleanupLabels(appName, namespace string) map[string]string {
	labels := make(map[string]string)
	labels["app"] = appName
	labels["namespace"] = namespace
	return labels
}
