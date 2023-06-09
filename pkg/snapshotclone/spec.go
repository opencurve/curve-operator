package snapshotclone

import (
	"fmt"
	"path"
	"strconv"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

// makeDeployment make snapshotclone deployment to run snapshotclone daemon
func (c *Cluster) makeDeployment(nodeName string, nodeIP string, snapConfig *snapConfig) (*apps.Deployment, error) {
	volumes := SnapDaemonVolumes(snapConfig)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "snapshotclone", snapConfig.DaemonID, c.Kind)

	containers := []v1.Container{c.makeSnapshotDaemonContainer(snapConfig)}
	deploymentConfig := k8sutil.DeploymentConfig{Name: snapConfig.ResourceName, NodeName: nodeName, Namespace: c.NamespacedName.Namespace,
		Labels: labels, Volumes: volumes, Containers: containers, OwnerInfo: c.OwnerInfo}
	d, err := k8sutil.MakeDeployment(deploymentConfig)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// makeSnapshotDaemonContainer create snapshotclone container
func (c *Cluster) makeSnapshotDaemonContainer(snapConfig *snapConfig) v1.Container {
	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	argsNginxConf := path.Join(config.NginxConfigMapMountPath, config.NginxConfigMapDataKey)
	configFileMountPath := path.Join(config.SnapShotCloneConfigMapMountPath, config.SnapShotCloneConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--conf=%s", configFileMountPath)

	port, _ := strconv.Atoi(snapConfig.ServicePort)
	dummyPort, _ := strconv.Atoi(snapConfig.ServiceDummyPort)
	proxyPort, _ := strconv.Atoi(snapConfig.ServiceProxyPort)

	container := v1.Container{
		Name: "snapshotclone",
		Command: []string{
			"/bin/bash",
			config.StartSnapConfigMapMountPath,
		},
		Args: []string{
			argsNginxConf,
			argsConfigFileDir,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    SnapDaemonVolumeMounts(snapConfig),
		SecurityContext: &v1.SecurityContext{
			Privileged:             &privileged,
			RunAsUser:              &runAsUser,
			RunAsNonRoot:           &runAsNonRoot,
			ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		},
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(port),
				HostPort:      int32(port),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "dummy-port",
				ContainerPort: int32(dummyPort),
				HostPort:      int32(dummyPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "proxy-port",
				ContainerPort: int32(proxyPort),
				HostPort:      int32(proxyPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
