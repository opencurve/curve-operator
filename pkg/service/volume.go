package service

import (
	"path"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/opencurve/curve-operator/pkg/topology"
	"github.com/opencurve/curve-operator/pkg/utils"
)

const (
	DATA_VOLUME = "data-volume"
	LOG_VOLUME  = "log-volume"
)

// A DataPathMap is a struct which contains information about where Curve service data is stored in
// containers and whether the data should be persisted to the host. If it is persisted to the host,
// directory on the host where the specific service's data is stored is given.
type DataPathMap struct {
	// HostDataDir should be set to the path on the host
	// where the specific service's data is stored.
	HostDataDir string

	// HostLogDir should be set to the path on the host
	// where the specific service's log is stored.
	HostLogDir string

	// ContainerDataDir should be set to the path in the container
	// where the specific service's data is stored.
	ContainerDataDir string

	// ContainerDataDir should be set to the path in the container
	// where the specific service's log is stored.
	ContainerLogDir string
}

// NewServiceDataPathMap returns a new DataPathMap for a service which does not utilize a data
// dir in the container as the mon, mgr, osd, mds, and rgw service do.
func NewServiceDataPathMap(hostDataDir string, hostLogDir string, containerDataDir string, containerLogDir string) *DataPathMap {
	return &DataPathMap{
		HostDataDir:      hostDataDir,
		HostLogDir:       hostLogDir,
		ContainerDataDir: containerDataDir,
		ContainerLogDir:  containerLogDir,
	}
}

// getServiceHostPathVolumeAndMount
func getServiceHostPathVolumeAndMount(dc *topology.DeployConfig) ([]v1.Volume, []v1.VolumeMount) {
	layout := dc.GetProjectLayout()
	dataPaths := &DataPathMap{
		HostDataDir:      dc.GetDataDir(),
		HostLogDir:       dc.GetLogDir(),
		ContainerDataDir: layout.ServiceDataDir,
		ContainerLogDir:  layout.ServiceLogDir,
	}

	// create Data hostpath volume and log hostpath volume
	vols, mounts := []v1.Volume{}, []v1.VolumeMount{}
	hostPathType := v1.HostPathDirectoryOrCreate
	if dataPaths != nil && dataPaths.HostDataDir != "" {
		src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: dataPaths.HostDataDir, Type: &hostPathType}}
		vols = append(vols, v1.Volume{Name: DATA_VOLUME, VolumeSource: src})
	}

	if dataPaths != nil && dataPaths.HostLogDir != "" {
		src := v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: dataPaths.HostLogDir, Type: &hostPathType}}
		vols = append(vols, v1.Volume{Name: LOG_VOLUME, VolumeSource: src})
	}

	// create data mount path and log mount path on container
	if dataPaths != nil && dataPaths.ContainerDataDir != "" {
		mounts = append(mounts, v1.VolumeMount{Name: DATA_VOLUME, MountPath: dataPaths.ContainerDataDir})
	}

	if dataPaths != nil && dataPaths.ContainerLogDir != "" {
		mounts = append(mounts, v1.VolumeMount{Name: LOG_VOLUME, MountPath: dataPaths.ContainerLogDir})
	}

	return vols, mounts
}

// getVolumeAndMount Create configmap volume and volumeMount for specified key
func getServiceConfigMapVolumeAndMount(dataKey, mountDir string) (v1.Volume, v1.VolumeMount) {
	configMapVolSource := &v1.ConfigMapVolumeSource{}
	mode := int32(0644)
	subPath := strings.Split(dataKey, "_")[1]
	configMapVolSource = &v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{Name: utils.AFTER_MUTATE_CONF},
		Items:                []v1.KeyToPath{{Key: dataKey, Path: subPath, Mode: &mode}},
	}

	volumeName := dataKey
	vol := v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: configMapVolSource,
		},
	}

	vm := v1.VolumeMount{
		Name:      volumeName,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: path.Join(mountDir, subPath),
		SubPath:   subPath,
	}

	return vol, vm
}

// getToolsAndTopoVolumeAndMount for create-pool job using
func getToolsAndTopoVolumeAndMount(dc *topology.DeployConfig) ([]v1.Volume, []v1.VolumeMount) {
	vols, volMounts := []v1.Volume{}, []v1.VolumeMount{}
	mode := int32(0644)
	subPath := topology.LAYOUT_TOOLS_NAME

	toolVolSource := &v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{
			Name: utils.AFTER_MUTATE_CONF,
		},
		Items: []v1.KeyToPath{
			{
				Key:  topology.LAYOUT_TOOLS_NAME,
				Path: subPath,
				Mode: &mode,
			},
		},
	}
	toolConfigVol := v1.Volume{
		Name: utils.AFTER_MUTATE_CONF,
		VolumeSource: v1.VolumeSource{
			ConfigMap: toolVolSource,
		},
	}

	toolVolMount := v1.VolumeMount{
		Name:      utils.AFTER_MUTATE_CONF,
		ReadOnly:  true, // should be no reason to write to the config in pods, so enforce this
		MountPath: dc.GetProjectLayout().ToolsConfSystemPath,
		SubPath:   subPath,
	}
	vols = append(vols, toolConfigVol)
	volMounts = append(volMounts, toolVolMount)

	topoVolSource := &v1.ConfigMapVolumeSource{
		LocalObjectReference: v1.LocalObjectReference{
			Name: CURVE_TOPOLOGY_CONFIGMAP,
		},
	}
	topoVol := v1.Volume{
		Name: CURVE_TOPOLOGY_CONFIGMAP,
		VolumeSource: v1.VolumeSource{
			ConfigMap: topoVolSource,
		},
	}
	topoVolMount := v1.VolumeMount{
		Name:      CURVE_TOPOLOGY_CONFIGMAP,
		ReadOnly:  true,
		MountPath: dc.GetProjectLayout().ToolsConfDir,
	}

	vols = append(vols, topoVol)
	volMounts = append(volMounts, topoVolMount)

	return vols, volMounts
}
