package chunkserver

import (
	"strings"

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

func (c *Cluster) getPodLabels(nodeName string) map[string]string {
	labels := make(map[string]string)
	labels["app"] = PrepareJobName
	labels["chunkserver_name"] = nodeName
	labels["curve_cluster"] = c.namespacedName.Namespace
	return labels
}
