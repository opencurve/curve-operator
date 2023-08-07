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
	go deleteSyncConfigDeployment(c, newDeployment.GetName())

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

type checkClusterDeployedInfo struct {
	mdsReady  int
	etcdReady int

	metaServerReady    int
	chunkServerReady   int
	snapShotCloneReady int

	grafanaReady      int
	prometheusReady   int
	nodeExporterReady int

	jobPreChunkFileCompleted    int
	jobProLogicPoolCompleted    int
	jobProPhysicalPoolCompleted int
}

// deleteSyncConfigDeployment delete the SyncConfigDeployment after the cluster is deployed.
func deleteSyncConfigDeployment(c *daemon.Cluster, syncConfigDeployment string) {

	wantChunkServer := len(c.Chunkserver.Devices) * len(c.Chunkserver.Nodes)
	nodeCount := len(c.Nodes)

	time.Sleep(1 * time.Minute)

	if c.Kind == config.KIND_CURVEBS {
		logger.Debugf("node count is %d, wanted chunk server count is %d", nodeCount, wantChunkServer)
	} else if c.Kind == config.KIND_CURVEFS {
		logger.Debugf("node count is %d", nodeCount)
	}

	checkTicker := time.NewTicker(30 * time.Second)

	for {
		isAllReadyOrCompleted := true
		info := &checkClusterDeployedInfo{}
		deploymentList, err := c.Context.Clientset.AppsV1().Deployments(c.Namespace).List(metav1.ListOptions{})
		if err != nil {
			logger.Errorf("failed to list deployment in namespace %s for delete curve-sync-config", c.Namespace)
		}

		jobs, err := c.Context.Clientset.BatchV1().Jobs(c.Namespace).List(metav1.ListOptions{})
		if err != nil {
			logger.Errorf("failed to list jobs in namespace %s for delete curve-sync-config", c.Namespace)
		}

		for _, deploy := range deploymentList.Items {
			switch {
			case strings.HasPrefix(deploy.Name, etcd.AppName):
				if isAllReplicasReady(deploy) {
					info.etcdReady++
				}
			case strings.HasPrefix(deploy.Name, mds.AppName):
				if isAllReplicasReady(deploy) {
					info.mdsReady++
				}
			case strings.HasPrefix(deploy.Name, chunkserver.AppName):
				if isAllReplicasReady(deploy) {
					info.chunkServerReady++
				}
			case strings.HasPrefix(deploy.Name, metaserver.AppName):
				if isAllReplicasReady(deploy) {
					info.metaServerReady++
				}
			case strings.HasPrefix(deploy.Name, snapshotclone.AppName):
				if isAllReplicasReady(deploy) {
					info.snapShotCloneReady++
				}
			case strings.HasPrefix(deploy.Name, monitor.GrafanaAppName):
				if isAllReplicasReady(deploy) {
					info.grafanaReady++
				}
			case strings.HasPrefix(deploy.Name, monitor.PromAppName):
				if isAllReplicasReady(deploy) {
					info.prometheusReady++
				}
			case strings.HasPrefix(deploy.Name, monitor.NodeExporterAppName):
				if isAllReplicasReady(deploy) {
					info.nodeExporterReady++
				}
			}
		}

		for _, job := range jobs.Items {
			switch {
			case strings.HasPrefix(job.Name, topology.JOB_PYHSICAL_POOL):
				if isJobCompleted(job) {
					info.jobProPhysicalPoolCompleted++
				}
			case strings.HasPrefix(job.Name, topology.JOB_LOGICAL_POOL):
				if isJobCompleted(job) {
					info.jobProLogicPoolCompleted++
				}
			case strings.HasPrefix(job.Name, chunkserver.PrepareJobName):
				if isJobCompleted(job) {
					info.jobPreChunkFileCompleted++
				}
			}
		}

		if c.SnapShotClone.Enable {
			if info.snapShotCloneReady != nodeCount {
				isAllReadyOrCompleted = false
			}
		}

		if c.Monitor.Enable {
			if info.grafanaReady == 0 ||
				info.prometheusReady == 0 ||
				info.nodeExporterReady != nodeCount {
				isAllReadyOrCompleted = false
			}
		}

		if c.Kind == config.KIND_CURVEBS && (info.chunkServerReady != wantChunkServer ||
			info.jobPreChunkFileCompleted != wantChunkServer ||
			info.jobProLogicPoolCompleted == 0 ||
			info.jobProPhysicalPoolCompleted == 0) {
			isAllReadyOrCompleted = false
		}

		if c.Kind == config.KIND_CURVEFS &&
			(info.metaServerReady != nodeCount || info.jobProLogicPoolCompleted == 0) {
			isAllReadyOrCompleted = false
		}

		if info.etcdReady != nodeCount || info.mdsReady != nodeCount {
			isAllReadyOrCompleted = false
		}

		if isAllReadyOrCompleted {
			break
		}
		<-checkTicker.C
	}

	err := c.Context.Clientset.AppsV1().Deployments(c.Namespace).Delete(syncConfigDeployment, &metav1.DeleteOptions{})
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
