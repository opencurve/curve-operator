package controllers

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/opencurve/curve-operator/pkg/chunkserver"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/etcd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/mds"
	"github.com/opencurve/curve-operator/pkg/metaserver"
	"github.com/opencurve/curve-operator/pkg/monitor"
	"github.com/opencurve/curve-operator/pkg/snapshotclone"
	"github.com/opencurve/curve-operator/pkg/topology"
)

const (
	SyncConfigDeployment = "curve-sync-config"
)

// createSyncDeployment create a deployment for read config file
func createSyncDeployment(c *daemon.Cluster) error {
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   SyncConfigDeployment,
			Labels: getReadConfigJobLabel(c),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				createSyncContainer(c),
			},
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			NodeName:      c.Nodes[0],
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SyncConfigDeployment,
			Namespace: c.NamespacedName.Namespace,
			Labels:    getReadConfigJobLabel(c),
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getReadConfigJobLabel(c),
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
		},
	}
	// set ownerReference
	err := c.OwnerInfo.SetControllerReference(d)
	if err != nil {
		return err
	}
	var deploymentsToWaitFor []*apps.Deployment

	newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.Namespace).Create(d)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create deployment %s", SyncConfigDeployment)
		}
		logger.Infof("deployment for %s already exists. updating if needed", SyncConfigDeployment)

		// TODO:Update the daemon Deployment
		// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
		// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
		// }
	} else {
		logger.Infof("Deployment %s has been created , waiting for startup", newDeployment.GetName())
		deploymentsToWaitFor = append(deploymentsToWaitFor, newDeployment)
	}

	// wait all Deployments to start
	for _, d := range deploymentsToWaitFor {
		if err := k8sutil.WaitForDeploymentToStart(context.TODO(), &c.Context, d); err != nil {
			return err
		}
	}

	// delete the SyncConfigDeployment after the cluster is deployed.
	go deleteSyncDeployment(c, newDeployment.GetName())

	// update condition type and phase etc.
	return nil
}

func createSyncContainer(c *daemon.Cluster) v1.Container {
	container := v1.Container{
		Name: "helper",
		Command: []string{
			"/bin/bash",
		},
		Args: []string{
			"-c",
			"while true; do echo sync pod to read various config file from it; sleep 10;done",
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		Env:             []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}
	return container
}

func readConfigFromContainer(c *daemon.Cluster, pod v1.Pod, configPath string) (string, error) {
	logger.Infof("syncing %v", configPath)
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)
	req := c.Context.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Container: pod.Spec.Containers[0].Name,
		Command:   []string{"cat", configPath},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.Context.KubeConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to init executor: %v", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})

	if err != nil {
		return "", fmt.Errorf("could not execute: %v", err)
	}

	if execErr.Len() > 0 {
		return "", fmt.Errorf("stderr: %v", execErr.String())
	}

	cmdOutput := execOut.String()
	return cmdOutput, nil
}

func getReadConfigJobLabel(c *daemon.Cluster) map[string]string {
	labels := make(map[string]string)
	labels["app"] = SyncConfigDeployment
	labels["curve"] = c.Kind
	return labels
}

// deleteSyncDeployment delete the SyncConfigDeployment after the cluster is deployed.
func deleteSyncDeployment(c *daemon.Cluster, deployName string) {

	time.Sleep(1 * time.Minute)

	clusterKind := c.Kind
	nodesCount := len(c.Nodes)
	chunkServerCount := len(c.Chunkserver.Nodes) * len(c.Chunkserver.Devices)

	if clusterKind == config.KIND_CURVEBS {
		logger.Infof("node count is %d, wanted chunk server count is %d", nodesCount, chunkServerCount)
	}

	checkTicker := time.NewTicker(30 * time.Second)
	for {
		isAllReady := true
		chunkSrvReady, etcdReady, mdsReady, metaSrvReady, snapShotCloneReady, prometheusReady, grafanaReady, nodeExporterReady :=
			0, 0, 0, 0, 0, 0, 0, 0

		jobPreChunkFileCompleted, jobProPhysicalPoolCompleted, jobProLogicPoolCompleted := 0, 0, 0

		deploymentList, err := c.Context.Clientset.AppsV1().Deployments(c.Namespace).List(metav1.ListOptions{})
		if err != nil {
			logger.Errorf("failed to list deployment in namespace %s for delete curve-sync-config", c.Namespace)
		}

		jobs, err := c.Context.Clientset.BatchV1().Jobs(c.Namespace).List(metav1.ListOptions{})
		if err != nil {
			logger.Errorf("failed to list jobs in namespace %s for delete curve-sync-config", c.Namespace)
		}

		for _, job := range jobs.Items {
			switch {
			case strings.HasPrefix(job.Name, topology.JOB_PYHSICAL_POOL):
				if isJobCompleted(job) {
					jobProPhysicalPoolCompleted++
				}
			case strings.HasPrefix(job.Name, topology.JOB_LOGICAL_POOL):
				if isJobCompleted(job) {
					jobProLogicPoolCompleted++
				}
			case strings.HasPrefix(job.Name, chunkserver.PrepareJobName):
				if isJobCompleted(job) {
					jobPreChunkFileCompleted++
				}
			}
		}

		for _, deploy := range deploymentList.Items {
			switch {
			case strings.HasPrefix(deploy.Name, chunkserver.AppName):
				if isAllReplicasReady(deploy) {
					chunkSrvReady++
				}
			case strings.HasPrefix(deploy.Name, etcd.AppName):
				if isAllReplicasReady(deploy) {
					etcdReady++
				}
			case strings.HasPrefix(deploy.Name, monitor.GrafanaAppName):
				if isAllReplicasReady(deploy) {
					grafanaReady++
				}
			case strings.HasPrefix(deploy.Name, mds.AppName):
				if isAllReplicasReady(deploy) {
					mdsReady++
				}
			case strings.HasPrefix(deploy.Name, metaserver.AppName):
				if isAllReplicasReady(deploy) {
					metaSrvReady++
				}
			case strings.HasPrefix(deploy.Name, monitor.PromAppName):
				if isAllReplicasReady(deploy) {
					prometheusReady++
				}
			case strings.HasPrefix(deploy.Name, snapshotclone.AppName):
				if isAllReplicasReady(deploy) {
					snapShotCloneReady++
				}
			case strings.HasPrefix(deploy.Name, monitor.NodeExporterAppName):
				if isAllReplicasReady(deploy) {
					nodeExporterReady++
				}
			}
		}

		if c.SnapShotClone.Enable {
			if snapShotCloneReady != nodesCount {
				isAllReady = false
			}
		}

		if c.Monitor.Enable {
			if grafanaReady == 0 || prometheusReady == 0 || nodeExporterReady != nodesCount {
				isAllReady = false
			}
		}

		if clusterKind == config.KIND_CURVEBS &&
			(chunkSrvReady != chunkServerCount || jobPreChunkFileCompleted != chunkServerCount ||
				jobProLogicPoolCompleted == 0 || jobProPhysicalPoolCompleted == 0) {
			isAllReady = false
		}

		if clusterKind == config.KIND_CURVEFS &&
			(metaSrvReady != nodesCount || jobProLogicPoolCompleted == 0) {
			isAllReady = false
		}

		if etcdReady != nodesCount || mdsReady != nodesCount {
			isAllReady = false
		}

		if isAllReady {
			break
		}
		<-checkTicker.C
	}

	err := c.Context.Clientset.AppsV1().Deployments(c.Namespace).Delete(deployName, &metav1.DeleteOptions{})
	if err != nil {
		logger.Errorf("failed to delete deployment about \"curve-sync-config\", error: %s", err)
	}

	logger.Infof("cluster is deployed, deployment about \"curve-sync-config\" will be deleted")
}

func isAllReplicasReady(deployment apps.Deployment) bool {
	if deployment.Status.Replicas == deployment.Status.ReadyReplicas {
		return true
	}
	return false
}

func isJobCompleted(job batch.Job) bool {
	if *job.Spec.Completions == job.Status.Succeeded {
		return true
	}
	return false
}
