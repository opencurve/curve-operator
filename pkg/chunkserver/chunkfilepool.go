package chunkserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/chunkserver/script"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

const (
	PrepareJobName         = "prepare-chunkfile"
	DEFAULT_CHUNKFILE_SIZE = 16 * 1024 * 1024 // 16MB

	formatConfigMapName     = "format-chunkfile-conf"
	formatScriptFileDataKey = "format.sh"
	formatScriptMountPath   = "/curvebs/tools/sbin/format.sh"
)

type Job2DeviceInfo struct {
	job      *batch.Job
	device   *curvev1.DevicesSpec
	nodeName string
}

// global variables
var job2DeviceInfos []*Job2DeviceInfo
var chunkserverConfigs []chunkserverConfig

// startProvisioningOverNodes format device and provision chunk files
func (c *Cluster) startProvisioningOverNodes(nodeNameIP map[string]string) ([]*topology.DeployConfig, error) {
	dcs := []*topology.DeployConfig{}
	if !c.Chunkserver.UseSelectedNodes {
		// clear slice
		job2DeviceInfos = []*Job2DeviceInfo{}
		chunkserverConfigs = []chunkserverConfig{}
		hostnameMap, err := k8sutil.GetNodeHostNames(c.Context.Clientset)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get node hostnames")
		}

		var storageNodes []string
		for _, nodeName := range c.Chunkserver.Nodes {
			storageNodes = append(storageNodes, hostnameMap[nodeName])
		}

		// get valid nodes that ready status and is schedulable
		validNodes, _ := k8sutil.GetValidNodes(c.Context, storageNodes)
		if len(validNodes) == 0 {
			logger.Warningf("no valid nodes available to run chunkservers on nodes in namespace %q", c.NamespacedName.Namespace)
			return nil, nil
		}

		logger.Infof("%d of the %d storage nodes are valid", len(validNodes), len(c.Chunkserver.Nodes))

		// create FORMAT configmap
		err = c.createFormatConfigMap()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create format ConfigMap")
		}

		// get ClusterEtcdAddr
		etcdOverrideCM, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.EtcdOverrideConfigMapName, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get etcd override endoints configmap")
		}
		clusterEtcdAddr := etcdOverrideCM.Data[config.ClusterEtcdAddr]

		// get ClusterMdsAddr
		mdsOverrideCM, err := c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Get(config.MdsOverrideConfigMapName, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get mds override endoints configmap")
		}
		clusterMdsAddr := mdsOverrideCM.Data[config.MdsOvverideConfigMapDataKey]

		// get clusterMdsDummyPort
		dummyPort := strconv.Itoa(c.Mds.DummyPort)
		clusterMdsDummyPort := dummyPort + "," + dummyPort + "," + dummyPort

		// get clusterSnapCloneAddr and clusterSnapShotCloneDummyPort
		var clusterSnapCloneAddr string
		var clusterSnapShotCloneDummyPort string
		if c.SnapShotClone.Enable {
			for _, ipAddr := range nodeNameIP {
				clusterSnapCloneAddr = fmt.Sprint(clusterSnapCloneAddr, ipAddr, ":", c.SnapShotClone.Port, ",")
			}
			clusterSnapCloneAddr = strings.TrimRight(clusterSnapCloneAddr, ",")

			dummyPort := strconv.Itoa(c.SnapShotClone.DummyPort)
			clusterSnapShotCloneDummyPort = fmt.Sprintf("%s,%s,%s", dummyPort, dummyPort, dummyPort)
		}

		hostSequence := 0
		daemonID := 0
		var daemonIDString string
		// travel all valid nodes to start job to prepare chunkfiles
		for _, node := range validNodes {
			nodeIP := nodeNameIP[node.Name]
			portBase := c.Chunkserver.Port
			replicasSequence := 0
			// travel all device to run format job and construct chunkserverConfig
			for _, device := range c.Chunkserver.Devices {
				daemonIDString = k8sutil.IndexToName(daemonID)
				name := strings.TrimSpace(device.Name)
				name = strings.TrimRight(name, "/")
				nameArr := strings.Split(name, "/")
				name = nameArr[len(nameArr)-1]
				resourceName := fmt.Sprintf("%s-%s-%s", AppName, node.Name, name)
				currentConfigMapName := fmt.Sprintf("%s-%s-%s", ConfigMapNamePrefix, node.Name, name)

				logger.Infof("creating job for device %s on %s", device.Name, node.Name)

				job, err := c.runPrepareJob(node.Name, device)
				if err != nil {
					return nil, err
				}

				jobInfo := &Job2DeviceInfo{
					job,
					&device,
					node.Name,
				}
				// jobsArr record all the job that have started, to determine whether the format is completed
				job2DeviceInfos = append(job2DeviceInfos, jobInfo)

				// create chunkserver config for each device of every node
				chunkserverConfig := chunkserverConfig{
					Prefix:                        Prefix,
					Port:                          portBase,
					ClusterMdsAddr:                clusterMdsAddr,
					ClusterMdsDummyPort:           clusterMdsDummyPort,
					ClusterEtcdAddr:               clusterEtcdAddr,
					ClusterSnapshotcloneAddr:      clusterSnapCloneAddr,
					ClusterSnapshotcloneDummyPort: clusterSnapShotCloneDummyPort,

					ResourceName:         resourceName,
					DaemonId:             daemonIDString,
					CurrentConfigMapName: currentConfigMapName,
					DataPathMap: &chunkserverDataPathMap{
						HostDevice:       device.Name,
						HostLogDir:       c.LogDirHostPath + "/chunkserver-" + node.Name + "-" + name,
						ContainerDataDir: ChunkserverContainerDataDir,
						ContainerLogDir:  ChunkserverContainerLogDir,
					},
					NodeName:         node.Name,
					NodeIP:           nodeIP,
					DeviceName:       device.Name,
					HostSequence:     hostSequence,
					ReplicasSequence: replicasSequence,
					Replicas:         len(c.Chunkserver.Devices),
				}

				dc := &topology.DeployConfig{
					Kind:             c.Kind,
					Role:             "chunkserver",
					Copysets:         c.Chunkserver.CopySets,
					NodeName:         node.Name,
					NodeIP:           nodeIP,
					Port:             portBase,
					DeviceName:       device.Name,
					ReplicasSequence: replicasSequence,
					Replicas:         len(c.Chunkserver.Devices),
				}
				chunkserverConfigs = append(chunkserverConfigs, chunkserverConfig)
				dcs = append(dcs, dc)
				portBase++
				replicasSequence++
				daemonID++
			}
			hostSequence++
		}
	}

	return dcs, nil
}

// createConfigMap create configmap to store format.sh script
func (c *Cluster) createFormatConfigMap() error {
	formatConfigMapData := map[string]string{
		formatScriptFileDataKey: script.FORMAT,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      formatConfigMapName,
			Namespace: c.NamespacedName.Namespace,
		},
		Data: formatConfigMapData,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to format configmap %q", formatConfigMapName)
	}

	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create override configmap %s", c.NamespacedName.Namespace)
	}

	return nil
}

// runPrepareJob create job and run job
func (c *Cluster) runPrepareJob(nodeName string, device curvev1.DevicesSpec) (*batch.Job, error) {
	job, _ := c.makeJob(nodeName, device)
	err := k8sutil.RunReplaceableJob(context.TODO(), c.Context.Clientset, job, false)
	if err != nil {
		return &batch.Job{}, err
	}

	return job, nil
}

func (c *Cluster) makeJob(nodeName string, device curvev1.DevicesSpec) (*batch.Job, error) {
	volumes, volumeMounts := c.createFormatVolumeAndMount(device)

	name := strings.TrimSpace(device.Name)
	name = strings.TrimRight(name, "/")
	nameArr := strings.Split(name, "/")
	name = nameArr[len(nameArr)-1]

	jobName := PrepareJobName + "-" + nodeName + "-" + name
	podName := PrepareJobName + "-" + nodeName

	runAsUser := int64(0)
	runAsNonRoot := false

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: c.getPodLabels(nodeName, device.Name),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.makeFormatContainer(device, volumeMounts),
			},
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyOnFailure,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:    &runAsUser,
				RunAsNonRoot: &runAsNonRoot,
			},
		},
	}

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: c.NamespacedName.Namespace,
			Labels:    c.getPodLabels(nodeName, device.Name),
		},
		Spec: batch.JobSpec{
			Template: podSpec,
		},
	}

	// set ownerReference
	err := c.OwnerInfo.SetControllerReference(job)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set owner reference to job %q", job.Name)
	}

	return job, nil
}

func (c *Cluster) makeFormatContainer(device curvev1.DevicesSpec, volumeMounts []v1.VolumeMount) v1.Container {
	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	argsPercent := strconv.Itoa(device.Percentage)
	argsFileSize := strconv.Itoa(DEFAULT_CHUNKFILE_SIZE)
	argsFilePoolDir := ChunkserverContainerDataDir + "/chunkfilepool"
	argsFilePoolMetaPath := ChunkserverContainerDataDir + "/chunkfilepool.meta"

	container := v1.Container{
		Name: "format",
		Args: []string{
			device.Name,
			ChunkserverContainerDataDir,
			argsPercent,
			argsFileSize,
			argsFilePoolDir,
			argsFilePoolMetaPath,
		},
		Command: []string{
			"/bin/bash",
			formatScriptMountPath,
		},
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    volumeMounts,
		SecurityContext: &v1.SecurityContext{
			Privileged:             &privileged,
			RunAsUser:              &runAsUser,
			RunAsNonRoot:           &runAsNonRoot,
			ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		},
	}

	return container
}

func (c *Cluster) getPodLabels(nodeName, deviceName string) map[string]string {
	labels := make(map[string]string)
	labels["app"] = PrepareJobName
	labels["node"] = nodeName
	s := strings.Split(deviceName, "/")
	if len(s) > 1 {
		deviceName = s[1]
	} else {
		// not occur
		deviceName = nodeName
	}
	labels["device"] = deviceName
	labels["curve_cluster"] = c.NamespacedName.Namespace
	return labels
}
