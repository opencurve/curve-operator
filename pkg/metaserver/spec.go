package metaserver

import (
	"fmt"
	"path"
	"strconv"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/logrotate"
	"github.com/opencurve/curve-operator/pkg/topology"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// makeDeployment make metaserver deployment to run mds daemon
func (c *Cluster) makeDeployment(metaserverConfig *metaserverConfig, nodeName string, nodeIP string) (*apps.Deployment, error) {
	volumes := daemon.DaemonVolumes(config.MetaServerConfigMapDataKey, config.MetaServerConfigMapMountPath, metaserverConfig.DataPathMap, metaserverConfig.CurrentConfigMapName)
	vols, _ := topology.CreateTopoAndToolVolumeAndMount(c.Cluster)
	volumes = append(volumes, vols...)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "metaserver", metaserverConfig.DaemonID, c.Kind)

	// add log config volume
	logConfCMVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: "log-conf"}}
	volumes = append(volumes, v1.Volume{Name: "log-conf", VolumeSource: v1.VolumeSource{ConfigMap: logConfCMVolSource}})

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   metaserverConfig.ResourceName,
			Labels: labels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeMSDaemonContainer(nodeIP, metaserverConfig),
				logrotate.MakeLogrotateContainer(),
			},
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metaserverConfig.ResourceName,
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

	err := c.OwnerInfo.SetControllerReference(d)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// makeMdsDaemonContainer create mds container
func (c *Cluster) makeMSDaemonContainer(nodeIP string, metaserverConfig *metaserverConfig) v1.Container {
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
