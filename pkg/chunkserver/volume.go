package chunkserver

import (
	"path"
	"strings"

	v1 "k8s.io/api/core/v1"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/config"
)

const (
	chunkserverVolumeName = "chunkserver-data"
)

// createFormatVolumeAndMount
func (c *Cluster) createFormatVolumeAndMount(device curvev1.DevicesSpec) ([]v1.Volume, []v1.VolumeMount) {
	vols := []v1.Volume{}
	mounts := []v1.VolumeMount{}

	// Create format configmap volume and volume path
	mode := int32(0644)
	formatCMVolSource := &v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{
			Name: formatConfigMapName,
		},
		Items: []v1.KeyToPath{
			{
				Key:  formatScriptFileDataKey,
				Path: formatScriptFileDataKey,
				Mode: &mode,
			},
		},
	}
	configVol := v1.Volume{
		Name: formatConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: formatCMVolSource,
		},
	}

	// configmap volume mount path
	formatCMVolumeMount := v1.VolumeMount{
		Name:      formatConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: formatScriptMountPath,
		SubPath:   formatScriptFileDataKey,
	}
	vols = append(vols, configVol)
	mounts = append(mounts, formatCMVolumeMount)

	// create hostpath volume and volume mount for device.MountPath
	hostPathType := v1.HostPathDirectoryOrCreate
	volumeName := strings.TrimSpace(device.MountPath)
	volumeName = strings.TrimRight(volumeName, "/")
	volumeNameArr := strings.Split(volumeName, "/")
	volumeName = volumeNameArr[len(volumeNameArr)-1]
	// volume name : chunkserver-data-chunkserver0
	tmpVolumeName := chunkserverVolumeName + "-" + volumeName

	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: device.MountPath, Type: &hostPathType}}
	vols = append(vols, v1.Volume{Name: tmpVolumeName, VolumeSource: src})
	mounts = append(mounts, v1.VolumeMount{Name: tmpVolumeName, MountPath: ChunkserverContainerDataDir})

	// Create hostpath volume and volume mount for '/dev'
	src = v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev"}}
	vols = append(vols, v1.Volume{Name: "devices", VolumeSource: src})
	mounts = append(mounts, v1.VolumeMount{Name: "devices", MountPath: "/dev"})

	return vols, mounts
}

// DaemonVolumes returns the pod volumes used only by chunkserver
func CSDaemonVolumes(csConfig *chunkserverConfig) []v1.Volume {
	vols := []v1.Volume{}

	// create configmap volume
	configMapVolumes, _ := CSConfigConfigMapVolumeAndMount(csConfig)
	vols = append(vols, configMapVolumes...)

	// create hostpath volume for '/dev'
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev"}}
	vols = append(vols, v1.Volume{Name: "dev-volume", VolumeSource: src})

	// create logs volume for
	hostPathType := v1.HostPathDirectoryOrCreate
	src = v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: csConfig.DataPathMap.HostLogDir, Type: &hostPathType}}
	vols = append(vols, v1.Volume{Name: "log-volume", VolumeSource: src})

	return vols
}

// CSDaemonVolumeMounts returns the pod container volume mounth used only by chunkserver
func CSDaemonVolumeMounts(csConfig *chunkserverConfig) []v1.VolumeMount {
	mounts := []v1.VolumeMount{}

	// create configmap mount path
	_, configMapMounts := CSConfigConfigMapVolumeAndMount(csConfig)
	mounts = append(mounts, configMapMounts...)

	// create data mount path and log mount path on container
	mounts = append(mounts, v1.VolumeMount{Name: "dev-volume", MountPath: "/dev"})
	mounts = append(mounts, v1.VolumeMount{Name: "log-volume", MountPath: csConfig.DataPathMap.ContainerLogDir})

	return mounts
}

// CSConfigConfigMapVolumeAndMount creates configmap volume and volume mount for daemon chunkserver pod
func CSConfigConfigMapVolumeAndMount(csConfig *chunkserverConfig) ([]v1.Volume, []v1.VolumeMount) {
	vols := []v1.Volume{}
	mounts := []v1.VolumeMount{}

	// start_chunkserver.sh
	mode := int32(0644)
	startChunkserverConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: startChunkserverConfigMapName}, Items: []v1.KeyToPath{{Key: startChunkserverScriptFileDataKey, Path: startChunkserverScriptFileDataKey, Mode: &mode}}}
	startChunkserverConfigVol := v1.Volume{
		Name: startChunkserverConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: startChunkserverConfigMapVolSource,
		},
	}
	vols = append(vols, startChunkserverConfigVol)

	// cs_client.conf
	CSClientConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.CSClientConfigMapName}, Items: []v1.KeyToPath{{Key: config.CSClientConfigMapDataKey, Path: config.CSClientConfigMapDataKey, Mode: &mode}}}
	CSClientConfigVol := v1.Volume{
		Name: config.CSClientConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: CSClientConfigMapVolSource,
		},
	}
	vols = append(vols, CSClientConfigVol)

	// s3.conf
	S3ConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.S3ConfigMapName}, Items: []v1.KeyToPath{{Key: config.S3ConfigMapDataKey, Path: config.S3ConfigMapDataKey, Mode: &mode}}}
	S3ConfigVol := v1.Volume{
		Name: config.S3ConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: S3ConfigMapVolSource,
		},
	}
	vols = append(vols, S3ConfigVol)

	// chunkserver.conf
	configMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: csConfig.CurrentConfigMapName}, Items: []v1.KeyToPath{{Key: config.ChunkserverConfigMapDataKey, Path: config.ChunkserverConfigMapDataKey, Mode: &mode}}}
	configVol := v1.Volume{
		Name: csConfig.CurrentConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: configMapVolSource,
		},
	}
	vols = append(vols, configVol)

	// start_chunkserver.sh volume mount
	startChunkserverMountPath := v1.VolumeMount{
		Name:      startChunkserverConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: startChunkserverMountPath,
		SubPath:   startChunkserverScriptFileDataKey,
	}
	mounts = append(mounts, startChunkserverMountPath)

	// cs_client.conf volume mount
	CSClientMountPath := v1.VolumeMount{
		Name:      config.CSClientConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: config.CSClientConfigMapMountPathDir + "/" + config.CSClientConfigMapDataKey,
		SubPath:   config.CSClientConfigMapDataKey,
	}
	mounts = append(mounts, CSClientMountPath)

	S3MountPath := v1.VolumeMount{
		Name:      config.S3ConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: config.S3ConfigMapMountPathDir + "/" + config.S3ConfigMapDataKey,
		SubPath:   config.S3ConfigMapDataKey,
	}
	mounts = append(mounts, S3MountPath)

	// configmap volume mount path
	m := v1.VolumeMount{
		Name:      csConfig.CurrentConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: path.Join(config.ChunkserverConfigMapMountPathDir, config.ChunkserverConfigMapDataKey),
		SubPath:   config.ChunkserverConfigMapDataKey,
	}
	mounts = append(mounts, m)

	return vols, mounts
}
