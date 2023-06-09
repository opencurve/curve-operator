package metaserver

import (
	"fmt"
	"path"
	"strconv"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

// makeDeployment make metaserver deployment to run mds daemon
func (c *Cluster) makeDeployment(metaServerConfig *metaserverConfig, nodeName string, nodeIP string) (*apps.Deployment, error) {
	volumes := daemon.DaemonVolumes(config.MetaServerConfigMapDataKey, config.MetaServerConfigMapMountPath, metaServerConfig.DataPathMap, metaServerConfig.CurrentConfigMapName)
	vols, _ := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volumes = append(volumes, vols...)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "metaserver", metaServerConfig.DaemonID, c.Kind)

	containers := []v1.Container{c.makeMSDaemonContainer(metaServerConfig)}
	deploymentConfig := k8sutil.DeploymentConfig{Name: metaServerConfig.ResourceName, NodeName: nodeName, Namespace: c.NamespacedName.Namespace,
		Labels: labels, Volumes: volumes, Containers: containers, OwnerInfo: c.OwnerInfo}
	d, err := k8sutil.MakeDeployment(deploymentConfig)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// makeMdsDaemonContainer create mds container
func (c *Cluster) makeMSDaemonContainer(metaserverConfig *metaserverConfig) v1.Container {
	configFileMountPath := path.Join(config.MetaServerConfigMapMountPath, config.MetaServerConfigMapDataKey)
	argsConfigFileDir := fmt.Sprintf("--confPath=%s", configFileMountPath)

	volMounts := daemon.DaemonVolumeMounts(config.MetaServerConfigMapDataKey, config.MetaServerConfigMapMountPath, metaserverConfig.DataPathMap, metaserverConfig.CurrentConfigMapName)
	_, mounts := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volMounts = append(volMounts, mounts...)

	port, _ := strconv.Atoi(metaserverConfig.ServicePort)
	// externalPort, _ := strconv.Atoi(metaserverConfig.ServiceExternalPort)

	container := v1.Container{
		Name: "metaserver",
		Command: []string{
			"/curvefs/metaserver/sbin/curvefs-metaserver",
		},
		Args: []string{
			argsConfigFileDir,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    volMounts,
		Ports: []v1.ContainerPort{
			{
				Name:          "listen-port",
				ContainerPort: int32(port),
				HostPort:      int32(port),
				Protocol:      v1.ProtocolTCP,
			},
			// {
			// 	Name:          "external-port",
			// 	ContainerPort: int32(externalPort),
			// 	HostPort:      int32(externalPort),
			// 	Protocol:      v1.ProtocolTCP,
			// },
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	return container
}
