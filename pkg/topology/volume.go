package topology

import (
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	v1 "k8s.io/api/core/v1"
)

// createTopoAndToolVolumeAndMount creates volumes and volumeMounts for topo and tool
func CreateTopoAndToolVolumeAndMount(c *daemon.Cluster) ([]v1.Volume, []v1.VolumeMount) {
	vols := []v1.Volume{}
	mounts := []v1.VolumeMount{}

	var topoMountPath, toolMountPath string
	if c.Kind == config.KIND_CURVEBS {
		topoMountPath = config.TopoJsonConfigmapMountPathDir
		toolMountPath = config.ToolsConfigMapMountPathDir
	} else {
		topoMountPath = config.FSTopoJsonConfigmapMountPathDir
		toolMountPath = config.FSToolsConfigMapMountPathDir
	}

	// Create topology configmap volume and volume mount("/curvebs/tools/conf/topology.json")
	mode := int32(0644)
	topoConfigMapVolSource := &v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{
			Name: config.TopoJsonConfigMapName,
		},
		Items: []v1.KeyToPath{
			{
				Key:  config.TopoJsonConfigmapDataKey,
				Path: config.TopoJsonConfigmapDataKey,
				Mode: &mode,
			},
		},
	}
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
		MountPath: topoMountPath,
	}
	mounts = append(mounts, topoMount)

	// Create tools configmap volume and volume mount("/etc/curve/tools.conf")
	toolConfigMapVolSource := &v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{
			Name: config.ToolsConfigMapName,
		},
		Items: []v1.KeyToPath{
			{
				Key:  config.ToolsConfigMapDataKey,
				Path: config.ToolsConfigMapDataKey,
				Mode: &mode,
			},
		},
	}
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
		MountPath: toolMountPath,
	}
	mounts = append(mounts, toolMount)

	return vols, mounts
}
