package snapshotclone

import (
	"path"

	v1 "k8s.io/api/core/v1"

	"github.com/opencurve/curve-operator/pkg/config"
)

// DaemonVolumes returns the pod volumes used only by snapshotclone
func SnapDaemonVolumes(snapConfig *snapConfig) []v1.Volume {
	vols := []v1.Volume{}
	// create configmap volume
	configMapVolumes, _ := SnapConfigMapVolumeAndMount(snapConfig)
	vols = append(vols, configMapVolumes...)

	hostPathType := v1.HostPathDirectoryOrCreate
	// create data volume
	src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: snapConfig.DataPathMap.HostDataDir, Type: &hostPathType}}
	vols = append(vols, v1.Volume{Name: "data-volume", VolumeSource: src})

	// create log volume
	src = v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: snapConfig.DataPathMap.HostLogDir, Type: &hostPathType}}
	vols = append(vols, v1.Volume{Name: "log-volume", VolumeSource: src})

	return vols
}

// DaemonVolumeMounts returns the pod container volume mounts used only by chunkserver
func SnapDaemonVolumeMounts(snapConfig *snapConfig) []v1.VolumeMount {
	mounts := []v1.VolumeMount{}

	// create configmap mount path
	_, configMapMounts := SnapConfigMapVolumeAndMount(snapConfig)
	mounts = append(mounts, configMapMounts...)

	// create data mount path and log mount path on container
	// create data mount path and log mount path on container
	mounts = append(mounts, v1.VolumeMount{Name: "data-volume", MountPath: snapConfig.DataPathMap.ContainerDataDir})
	mounts = append(mounts, v1.VolumeMount{Name: "log-volume", MountPath: snapConfig.DataPathMap.ContainerLogDir})

	return mounts
}

// configConfigMapVolumeAndMount Create configmap volume and volume mount for daemon chunkserver pod
func SnapConfigMapVolumeAndMount(snapConfig *snapConfig) ([]v1.Volume, []v1.VolumeMount) {
	vols := []v1.Volume{}
	mounts := []v1.VolumeMount{}

	// nginx.conf
	mode := int32(0644)
	nginxConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.NginxConfigMapName}, Items: []v1.KeyToPath{{Key: config.NginxConfigMapDataKey, Path: config.NginxConfigMapDataKey, Mode: &mode}}}
	nginxConfigVol := v1.Volume{
		Name: config.NginxConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: nginxConfigMapVolSource,
		},
	}
	vols = append(vols, nginxConfigVol)

	// start_snap.sh
	startSnapConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.StartSnapConfigMap}, Items: []v1.KeyToPath{{Key: config.StartSnapConfigMapDataKey, Path: config.StartSnapConfigMapDataKey, Mode: &mode}}}
	startSnapConfigVol := v1.Volume{
		Name: config.StartSnapConfigMap,
		VolumeSource: v1.VolumeSource{
			ConfigMap: startSnapConfigMapVolSource,
		},
	}
	vols = append(vols, startSnapConfigVol)

	// snap_client.conf
	snapClientConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.SnapClientConfigMapName}, Items: []v1.KeyToPath{{Key: config.SnapClientConfigMapDataKey, Path: config.SnapClientConfigMapDataKey, Mode: &mode}}}
	snapClientConfigVol := v1.Volume{
		Name: config.SnapClientConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: snapClientConfigMapVolSource,
		},
	}
	vols = append(vols, snapClientConfigVol)

	// s3.conf
	S3ConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: config.S3ConfigMapName}, Items: []v1.KeyToPath{{Key: config.S3ConfigMapDataKey, Path: config.S3ConfigMapDataKey, Mode: &mode}}}
	S3ConfigVol := v1.Volume{
		Name: config.S3ConfigMapName + "-snapshotclone",
		VolumeSource: v1.VolumeSource{
			ConfigMap: S3ConfigMapVolSource,
		},
	}
	vols = append(vols, S3ConfigVol)

	// snapshotclone.conf
	snapShotConfigMapVolSource := &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: snapConfig.CurrentConfigMapName}, Items: []v1.KeyToPath{{Key: config.SnapShotCloneConfigMapDataKey, Path: config.SnapShotCloneConfigMapDataKey, Mode: &mode}}}
	configVol := v1.Volume{
		Name: snapConfig.CurrentConfigMapName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: snapShotConfigMapVolSource,
		},
	}
	vols = append(vols, configVol)

	// nginx.conf volume mount
	nginxMountPath := v1.VolumeMount{
		Name:      config.NginxConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: path.Join(config.NginxConfigMapMountPath, config.NginxConfigMapDataKey),
		SubPath:   config.NginxConfigMapDataKey,
	}
	mounts = append(mounts, nginxMountPath)

	startSnapMountPath := v1.VolumeMount{
		Name:      config.StartSnapConfigMap,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: config.StartSnapConfigMapMountPath,
		SubPath:   config.StartSnapConfigMapDataKey,
	}
	mounts = append(mounts, startSnapMountPath)

	// snap_client.conf volume mount
	snapClientMountPath := v1.VolumeMount{
		Name:      config.SnapClientConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: path.Join(config.SnapClientConfigMapMountPath, config.SnapClientConfigMapDataKey),
		SubPath:   config.SnapClientConfigMapDataKey,
	}
	mounts = append(mounts, snapClientMountPath)

	// s3.conf volume mount
	S3MountPath := v1.VolumeMount{
		Name:      config.S3ConfigMapName + "-snapshotclone",
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: path.Join(config.S3ConfigMapMountSnapPathDir, config.S3ConfigMapDataKey),
		SubPath:   config.S3ConfigMapDataKey,
	}
	mounts = append(mounts, S3MountPath)

	// snapshotclone volume mount path
	m := v1.VolumeMount{
		Name:      snapConfig.CurrentConfigMapName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: path.Join(config.SnapShotCloneConfigMapMountPath, config.SnapShotCloneConfigMapDataKey),
		SubPath:   config.SnapShotCloneConfigMapDataKey,
	}

	mounts = append(mounts, m)

	return vols, mounts
}
