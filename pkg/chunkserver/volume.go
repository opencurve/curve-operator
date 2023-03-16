package chunkserver

import (
	"strings"

	"github.com/opencurve/curve-operator/pkg/config"
	v1 "k8s.io/api/core/v1"
)

const (
	chunkserverVolumeName = "chunkserver-data"
	devVolumeName         = "devices"

	chunkserverVolumeMountName = "/curvebs/chunkserver/data"
	devVolumeMountName         = "/dev"
)

// createMountPathVolume returns the pod volumes and volumemounts
func (c *Cluster) createVolumeAndMount() ([]v1.Volume, []v1.VolumeMount) {
	vols := []v1.Volume{}
	mounts := []v1.VolumeMount{}

	hostPathType := v1.HostPathDirectoryOrCreate

	for _, device := range c.spec.Storage.Devices {
		volumeName := strings.TrimSpace(device.MountPath)
		volumeName = strings.TrimRight(volumeName, "/")
		volumeNameArr := strings.Split(volumeName, "/")
		volumeName = volumeNameArr[len(volumeNameArr)-1]

		src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: device.MountPath, Type: &hostPathType}}

		// chunkserver-data-chunkserver0
		tmpVolumeName := chunkserverVolumeName + "-" + volumeName
		// /curvebs/chunkserver/data/chunkserver0
		tmpVolumeMountName := chunkserverVolumeMountName + "/" + volumeName

		vols = append(vols, v1.Volume{Name: tmpVolumeName, VolumeSource: src})
		mounts = append(mounts, v1.VolumeMount{Name: tmpVolumeName, MountPath: tmpVolumeMountName})

	}

	// create '/dev' hostpath volume
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev"}}
	vols = append(vols, v1.Volume{Name: devVolumeName, VolumeSource: src})

	mounts = append(mounts, v1.VolumeMount{Name: devVolumeName, MountPath: devVolumeMountName})

	return vols, mounts
}

// createDevVolumeAndMount
func (c *Cluster) createDevVolumeAndMount() ([]v1.Volume, []v1.VolumeMount) {
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

	for _, device := range c.spec.Storage.Devices {
		volumeName := strings.TrimSpace(device.MountPath)
		volumeName = strings.TrimRight(volumeName, "/")
		volumeNameArr := strings.Split(volumeName, "/")
		volumeName = volumeNameArr[len(volumeNameArr)-1]

		// volume name : chunkserver-data-chunkserver0
		tmpVolumeName := chunkserverVolumeName + "-" + volumeName

		src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: device.MountPath, Type: &hostPathType}}

		vols = append(vols, v1.Volume{Name: tmpVolumeName, VolumeSource: src})
		mounts = append(mounts, v1.VolumeMount{Name: tmpVolumeName, MountPath: device.MountPath})
	}

	// 3. Create hostpath volume and volume mount for '/dev'
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev"}}
	vols = append(vols, v1.Volume{Name: devVolumeName, VolumeSource: src})

	mounts = append(mounts, v1.VolumeMount{Name: devVolumeName, MountPath: devVolumeMountName})

	return vols, mounts
}

// DaemonVolumes returns the pod volumes used only by chunkserver

// configMapDataKey = config.ChunkserverConfigMapDataKey ("chunkserver.conf")
// configMapMountPathDir = config.ChunkserverConfigMapMountPathDir ("/curvebs/chunkserver/conf")
// curConfigMapName = config.ChunkserverConfigMapName ("curve-chunkserver-conf")

func CSDaemonVolumes(configMapDataKey string, configMapMountPathDir string, curConfigMapName string, dataPaths *chunkserverDataPathMap) []v1.Volume {
	vols := []v1.Volume{}

	// create configmap volume
	configMapVolumes, _ := CSConfigConfigMapVolumeAndMount(configMapDataKey, configMapMountPathDir, curConfigMapName)
	vols = append(vols, configMapVolumes...)

	// create hostpath volume for '/dev'
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev"}}
	vols = append(vols, v1.Volume{Name: "data-volume", VolumeSource: src})

	return vols
}

// DaemonVolumeMounts returns the pod container volume mounth used only by chunkserver
func CSDaemonVolumeMounts(configMapDataKey string, configMapMountPathDir string, curConfigMapName string, dataPaths *chunkserverDataPathMap) []v1.VolumeMount {
	mounts := []v1.VolumeMount{}

	// create configmap mount path
	_, configMapMounts := CSConfigConfigMapVolumeAndMount(configMapDataKey, configMapMountPathDir, curConfigMapName)
	mounts = append(mounts, configMapMounts...)

	// create data mount path and log mount path on container
	mounts = append(mounts, v1.VolumeMount{Name: "data-volume", MountPath: "/dev"})

	return mounts
}

// configConfigMapVolumeAndMount Create configmap volume and volume mount for daemon chunkserver pod
func CSConfigConfigMapVolumeAndMount(configMapDataKey string, configMapMountPathDir string, curConfigMapName string) ([]v1.Volume, []v1.VolumeMount) {
	vols := []v1.Volume{}
	mounts := []v1.VolumeMount{}

	mode := int32(0644)
	// cs_client.conf
	CSClientConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.CSClientConfigMapName}, Items: []v1.KeyToPath{{Key: config.CSClientConfigMapDataKey, Path: config.CSClientConfigMapDataKey, Mode: &mode}}}
	CSClientConfigVol := v1.Volume{
		Name: config.CSClientConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: CSClientConfigMapVolSource,
		},
	}
	vols = append(vols, CSClientConfigVol)

	S3ConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.S3ConfigMapName}, Items: []v1.KeyToPath{{Key: config.S3ConfigMapDataKey, Path: config.S3ConfigMapDataKey, Mode: &mode}}}
	S3ConfigVol := v1.Volume{
		Name: config.S3ConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: S3ConfigMapVolSource,
		},
	}
	vols = append(vols, S3ConfigVol)

	configMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: curConfigMapName}, Items: []v1.KeyToPath{{Key: configMapDataKey, Path: configMapDataKey, Mode: &mode}}}
	configVol := v1.Volume{
		Name: curConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: configMapVolSource,
		},
	}
	vols = append(vols, configVol)

	// 3 volume mount
	CSClientMountPath := v1.VolumeMount{
		Name:      config.CSClientConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: config.CSClientConfigMapMountPathDir + "/cs_client.conf",
		SubPath:   "cs_client.conf",
	}
	mounts = append(mounts, CSClientMountPath)

	S3MountPath := v1.VolumeMount{
		Name:      config.S3ConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: config.S3ConfigMapMountPathDir + "/s3.conf",
		SubPath:   "s3.conf",
	}
	mounts = append(mounts, S3MountPath)

	// configmap volume mount path
	m := v1.VolumeMount{
		Name:      curConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: configMapMountPathDir + "/chunkserver.conf",
		SubPath:   "chunkserver.conf",
	}
	mounts = append(mounts, m)

	return vols, mounts
}
