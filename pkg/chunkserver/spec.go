package chunkserver

import (
	"path"
	"strconv"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

func (c *Cluster) makeDeployment(csConfig *chunkserverConfig) (*apps.Deployment, error) {
	volumes := CSDaemonVolumes(csConfig)
	vols, _ := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volumes = append(volumes, vols...)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "chunkserver", csConfig.DaemonId, c.Kind)

	containers := []v1.Container{c.makeCSDaemonContainer(csConfig)}
	deploymentConfig := k8sutil.DeploymentConfig{Name: csConfig.ResourceName, NodeName: csConfig.NodeName, Namespace: c.NamespacedName.Namespace,
		Labels: labels, Volumes: volumes, Containers: containers, OwnerInfo: c.OwnerInfo}
	d, err := k8sutil.MakeDeployment(deploymentConfig)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// makeCSDaemonContainer create chunkserver container
func (c *Cluster) makeCSDaemonContainer(csConfig *chunkserverConfig) v1.Container {

	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	// volumemount
	volMounts := CSDaemonVolumeMounts(csConfig)
	_, mounts := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volMounts = append(volMounts, mounts...)

	argsDeviceName := csConfig.DeviceName
	argsMountPath := ChunkserverContainerDataDir

	argsDataDir := path.Join(csConfig.Prefix, "data")
	argsChunkServerIp := csConfig.NodeIP
	argsChunkserverPort := strconv.Itoa(csConfig.Port)
	argsConfigFileMountPath := path.Join(config.ChunkserverConfigMapMountPathDir, config.ChunkserverConfigMapDataKey)

	container := v1.Container{
		Name: "chunkserver",
		Command: []string{
			"/bin/bash",
			startChunkserverMountPath,
		},
		Args: []string{
			argsDeviceName,
			argsMountPath,
			argsDataDir,
			argsChunkServerIp,
			argsChunkserverPort,
			argsConfigFileMountPath,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    volMounts,
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(csConfig.Port),
				HostPort:      int32(csConfig.Port),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
		SecurityContext: &v1.SecurityContext{
			Privileged:             &privileged,
			RunAsUser:              &runAsUser,
			RunAsNonRoot:           &runAsNonRoot,
			ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		},
	}

	return container
}
