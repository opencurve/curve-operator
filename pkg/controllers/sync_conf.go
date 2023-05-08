package controllers

import (
	"bytes"
	"context"
	"fmt"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	SyncConfigDeployment = "curve-sync-config"
)

var BSConfigs = []string{
	"etcd.conf",
	"mds.conf",
	"chunkserver.conf",
	"snapshotclone.conf",
	"snap_client.conf",
	"cs_client.conf",
	"s3.conf",
	"nginx.conf",
	"tools.conf",
}

var FSConfigs = []string{
	"etcd.conf",
	"mds.conf",
	"metaserver.conf",
	"tools.conf",
}

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

func readConfigFromContainer(c *daemon.Cluster, pod v1.Pod, configName string) (string, error) {
	var configPath string
	if c.Kind == config.KIND_CURVEBS {
		configPath = "/curvebs/conf/" + configName
	} else {
		configPath = "/curvefs/conf/" + configName
	}
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
