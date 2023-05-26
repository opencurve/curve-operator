package daemon

import (
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/opencurve/curve-operator/pkg/config"
)

// DaemonVolumes returns the pod volumes used by all Curve daemons.
func DaemonVolumes(configMapDataKey string, configMapMountPathDir string, dataPaths *config.DataPathMap, curConfigMapName string) []v1.Volume {
	// create configmap volume
	vols := []v1.Volume{}
	if curConfigMapName != "" {
		configVol, _ := configConfigMapVolumeAndMount(configMapDataKey, configMapMountPathDir, curConfigMapName)
		vols = append(vols, configVol)
	}

	// create Data hostpath volume and log hostpath volume
	hostPathType := v1.HostPathDirectoryOrCreate
	if dataPaths != nil && dataPaths.HostDataDir != "" {
		src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: dataPaths.HostDataDir, Type: &hostPathType}}
		vols = append(vols, v1.Volume{Name: "data-volume", VolumeSource: src})
	}

	if dataPaths != nil && dataPaths.HostLogDir != "" {
		src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: dataPaths.HostLogDir, Type: &hostPathType}}
		vols = append(vols, v1.Volume{Name: "log-volume", VolumeSource: src})
	}

	return vols
}

// DaemonVolumeMounts returns the pod container volumeMounts used by Curve daemon
func DaemonVolumeMounts(configMapDataKey string, configMapMountPathDir string, dataPaths *config.DataPathMap, curConfigMapName string) []v1.VolumeMount {
	// create configmap mount path
	mounts := []v1.VolumeMount{}
	if curConfigMapName != "" {
		_, configMapMount := configConfigMapVolumeAndMount(configMapDataKey, configMapMountPathDir, curConfigMapName)
		mounts = append(mounts, configMapMount)
	}

	// create data mount path and log mount path on container
	if dataPaths != nil && dataPaths.ContainerDataDir != "" {
		mounts = append(mounts, v1.VolumeMount{Name: "data-volume", MountPath: dataPaths.ContainerDataDir})
	}

	if dataPaths != nil && dataPaths.ContainerLogDir != "" {
		mounts = append(mounts, v1.VolumeMount{Name: "log-volume", MountPath: dataPaths.ContainerLogDir})
	}

	return mounts
}

// configConfigMapVolumeAndMount Create configmap volume and volume mount for daemon pod
func configConfigMapVolumeAndMount(configMapDataKey string, configMapMountPathDir string, curConfigMapName string) (v1.Volume, v1.VolumeMount) {
	configMapVolSource := &v1.ConfigMapVolumeSource{}
	mode := int32(0644)
	if configMapDataKey == "" {
		configMapVolSource = &v1.ConfigMapVolumeSource{
			LocalObjectReference: v1.LocalObjectReference{Name: curConfigMapName},
		}
	} else {
		configMapVolSource = &v1.ConfigMapVolumeSource{
			LocalObjectReference: v1.LocalObjectReference{Name: curConfigMapName},
			Items:                []v1.KeyToPath{{Key: configMapDataKey, Path: configMapDataKey, Mode: &mode}},
		}
	}

	volumeName := curConfigMapName + strings.Split(configMapDataKey, ".")[0]
	configVol := v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: configMapVolSource,
		},
	}

	// configmap volume mount path
	m := v1.VolumeMount{
		Name:      volumeName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: configMapMountPathDir,
	}

	return configVol, m
}
