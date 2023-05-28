package chunkserver

import (
	"path"
	"strconv"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/topology"
)

func (c *Cluster) makeDeployment(csConfig *chunkserverConfig) (*apps.Deployment, error) {
	volumes := CSDaemonVolumes(csConfig)
	vols, _ := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volumes = append(volumes, vols...)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "chunkserver", csConfig.DaemonId, c.Kind)

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   csConfig.ResourceName,
			Labels: labels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeCSDaemonContainer(csConfig),
			},
			NodeName:      csConfig.NodeName,
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      csConfig.ResourceName,
			Namespace: c.NamespacedName.Namespace,
			Labels:    labels,
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
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
