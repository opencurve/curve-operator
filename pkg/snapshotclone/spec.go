package snapshotclone

import (
	"fmt"
	"path"
	"strconv"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
)

// makeDeployment make snapshotclone deployment to run snapshotclone daemon
func (c *Cluster) makeDeployment(nodeName string, nodeIP string, snapConfig *snapConfig) (*apps.Deployment, error) {
	volumes := SnapDaemonVolumes(snapConfig)
	labels := daemon.CephDaemonAppLabels(AppName, c.Namespace, "snapshotclone", snapConfig.DaemonID, c.Kind)

	// for debug
	// log.Infof("snapConfig %+v", snapConfig)

	runAsUser := int64(0)
	runAsNonRoot := false

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   snapConfig.ResourceName,
			Labels: labels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeSnapshotDaemonContainer(nodeIP, snapConfig),
			},
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:    &runAsUser,
				RunAsNonRoot: &runAsNonRoot,
			},
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapConfig.ResourceName,
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
		return nil, errors.Wrapf(err, "failed to set owner reference to chunkserver deployment %q", d.Name)
	}

	return d, nil
}

// makeSnapshotDaemonContainer create snapshotclone container
func (c *Cluster) makeSnapshotDaemonContainer(nodeIP string, snapConfig *snapConfig) v1.Container {
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
