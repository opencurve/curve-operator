package chunkserver

import (
	"strings"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/config"
	v1 "k8s.io/api/core/v1"
)

const (
	chunkserverVolumeName = "chunkserver-data"
)

// createFormatVolumeAndMount
func (c *Cluster) createFormatVolumeAndMount(device curvev1.DevicesSpec) ([]v1.Volume, []v1.VolumeMount) {
	vols := []v1.Volume{}
	mounts := []v1.VolumeMount{}

	// 1. Create format configmap volume and volume path
	mode := int32(0644)
	configMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: formatConfigMapName}, Items: []v1.KeyToPath{{Key: formatScriptFileDataKey, Path: formatScriptFileDataKey, Mode: &mode}}}
	configVol := v1.Volume{
		Name: formatConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: configMapVolSource,
		},
	}

	// configmap volume mount path
	m := v1.VolumeMount{
		Name:      formatConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: formatScriptMountPath,
		SubPath:   formatScriptFileDataKey,
	}
	vols = append(vols, configVol)
	mounts = append(mounts, m)

	// 2. Create hostpath volume and volume mount for device.MountPath
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

	// 3. Create hostpath volume and volume mount for '/dev'
	src = v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev"}}
	vols = append(vols, v1.Volume{Name: "devices", VolumeSource: src})
	mounts = append(mounts, v1.VolumeMount{Name: "devices", MountPath: "/dev"})

	return vols, mounts
}

// DaemonVolumes returns the pod volumes used only by chunkserver

// configMapDataKey = config.ChunkserverConfigMapDataKey ("chunkserver.conf")
// configMapMountPathDir = config.ChunkserverConfigMapMountPathDir ("/curvebs/chunkserver/conf")
// curConfigMapName = config.ChunkserverConfigMapName ("curve-chunkserver-conf")

func CSDaemonVolumes(dataPaths *chunkserverDataPathMap) []v1.Volume {
	vols := []v1.Volume{}

	// create configmap volume
	configMapVolumes, _ := CSConfigConfigMapVolumeAndMount()
	vols = append(vols, configMapVolumes...)

	// create hostpath volume for '/dev'
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev"}}
	vols = append(vols, v1.Volume{Name: "dev-volume", VolumeSource: src})

	// create logs volume for
	hostPathType := v1.HostPathDirectoryOrCreate
	src = v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: dataPaths.HostLogDir, Type: &hostPathType}}
	vols = append(vols, v1.Volume{Name: "log-volume", VolumeSource: src})

	return vols
}

// DaemonVolumeMounts returns the pod container volume mounth used only by chunkserver
func CSDaemonVolumeMounts(dataPaths *chunkserverDataPathMap) []v1.VolumeMount {
	mounts := []v1.VolumeMount{}

	// create configmap mount path
	_, configMapMounts := CSConfigConfigMapVolumeAndMount()
	mounts = append(mounts, configMapMounts...)

	// create data mount path and log mount path on container
	mounts = append(mounts, v1.VolumeMount{Name: "dev-volume", MountPath: "/dev"})
	mounts = append(mounts, v1.VolumeMount{Name: "log-volume", MountPath: dataPaths.ContainerLogDir})

	return mounts
}

// configConfigMapVolumeAndMount Create configmap volume and volume mount for daemon chunkserver pod
func CSConfigConfigMapVolumeAndMount() ([]v1.Volume, []v1.VolumeMount) {
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
	configMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.ChunkserverConfigMapName}, Items: []v1.KeyToPath{{Key: config.ChunkserverConfigMapDataKey, Path: config.ChunkserverConfigMapDataKey, Mode: &mode}}}
	configVol := v1.Volume{
		Name: config.ChunkserverConfigMapName,
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
		Name:      config.ChunkserverConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: config.ChunkserverConfigMapMountPathDir + "/" + config.ChunkserverConfigMapDataKey,
		SubPath:   config.ChunkserverConfigMapDataKey,
	}
	mounts = append(mounts, m)

	return vols, mounts
}

// createTopoAndToolVolumeAndMount
func (c *Cluster) createTopoAndToolVolumeAndMount() ([]v1.Volume, []v1.VolumeMount) {
	vols := []v1.Volume{}
	mounts := []v1.VolumeMount{}

	// 1. Create topology configmap volume and volume mount path("/curvebs/tools/conf/topology.json")
	mode := int32(0644)
	topoConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.TopoJsonConfigMapName}, Items: []v1.KeyToPath{{Key: config.TopoJsonConfigmapDataKey, Path: config.TopoJsonConfigmapDataKey, Mode: &mode}}}
	topoConfigVol := v1.Volume{
		Name: config.TopoJsonConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: topoConfigMapVolSource,
		},
	}
	vols = append(vols, topoConfigVol)

	topoMount := v1.VolumeMount{
		Name:      config.TopoJsonConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: config.TopoJsonConfigmapMountPathDir,
	}
	mounts = append(mounts, topoMount)

	// 1. Create tools configmap volume and volume mount path("/etc/curve/tools.conf")
	toolConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.ToolsConfigMapName}, Items: []v1.KeyToPath{{Key: config.ToolsConfigMapDataKey, Path: config.ToolsConfigMapDataKey, Mode: &mode}}}
	toolConfigVol := v1.Volume{
		Name: config.ToolsConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: toolConfigMapVolSource,
		},
	}
	vols = append(vols, toolConfigVol)

	toolMount := v1.VolumeMount{
		Name:      config.ToolsConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: config.ToolsConfigMapMountPathDir,
	}
	mounts = append(mounts, toolMount)

	return vols, mounts
}
