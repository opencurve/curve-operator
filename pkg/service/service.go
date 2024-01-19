package service

import (
	"fmt"
	"strings"

	"github.com/coreos/pkg/capnslog"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "service")

// createService create specified service according to specified dc object
// for example etcd, mds
func StartService(cluster clusterd.Clusterer, dc *topology.DeployConfig) error {
	return makeServiceDeployment(cluster, dc)
}

// createServiceDeployment create service Deployment and wait it to start according to specified dc object
func makeServiceDeployment(cluster clusterd.Clusterer, dc *topology.DeployConfig) error {
	layout := dc.GetProjectLayout()
	vols, volMounts := getServiceHostPathVolumeAndMount(dc)

	// resolve configmap volume and volumeMount
	for _, conf := range layout.ServiceConfFiles {
		vm, vms := getServiceConfigMapVolumeAndMount(fmt.Sprintf("%s_%s", dc.GetName(), conf.Name),
			layout.ServiceConfDir)
		vols = append(vols, vm)
		volMounts = append(volMounts, vms)
	}

	container := v1.Container{
		Name: getResourceName(dc),
		Command: []string{
			fmt.Sprintf("--role %s --args='%s'", dc.GetRole(), getArguments(dc)),
		},
		Image:           cluster.GetContainerImage(),
		ImagePullPolicy: v1.PullIfNotPresent,
		VolumeMounts:    volMounts,
		Ports:           getContainerPorts(dc),
		Env: []v1.EnvVar{
			{Name: "TZ", Value: "Asia/Hangzhou"},
			{Name: "'LD_PRELOAD=%s'", Value: "/usr/local/lib/libjemalloc.so"},
		},
	}

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   getResourceName(dc),
			Labels: getServiceLabel(dc),
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				// c.makeChmodDirInitContainer(etcdConfig),
			},
			Containers: []v1.Container{
				// c.makeEtcdDaemonContainer(nodeName, ip, etcdConfig, etcdConfig.ClusterEtcdHttpAddr),
				// logrotate.MakeLogrotateContainer(),
				container,
			},
			NodeName:      dc.GetHost(),
			RestartPolicy: getRestartPolicy(dc),
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       vols,
		},
	}

	replicas := int32(1)
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getResourceName(dc),
			Namespace: cluster.GetNameSpace(),
			Labels:    getServiceLabel(dc),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getServiceLabel(dc),
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}

	// set ownerReference
	err := cluster.GetOwnerInfo().SetControllerReference(d)
	if err != nil {
		return err
	}

	_, err = k8sutil.CreateOrUpdateDeploymentAndWaitStart(cluster.GetContext().Clientset, d)
	if err != nil {
		return err
	}

	logger.Infof("Create %s service Deployment in namespace %s successed", dc.GetName(), cluster.GetNameSpace())

	return nil
}

// getResourceName get the name of k8s curve resource
func getResourceName(dc *topology.DeployConfig) string {
	return fmt.Sprintf("%s-%s", "curve", dc.GetName())
}

// getServiceLabel get labels of specified service
func getServiceLabel(dc *topology.DeployConfig) map[string]string {
	labels := map[string]string{}
	labels["role"] = dc.GetRole()
	labels["name"] = dc.GetName()
	return labels
}

// getArguments get service command arguments
func getArguments(dc *topology.DeployConfig) string {
	role := dc.GetRole()
	if role != topology.ROLE_CHUNKSERVER {
		return ""
	}

	// only chunkserver need so many arguments, but who cares
	layout := dc.GetProjectLayout()
	dataDir := layout.ServiceDataDir
	chunkserverArguments := map[string]interface{}{
		// chunkserver
		"conf":                  layout.ServiceConfPath,
		"chunkServerIp":         dc.GetHostIp(),
		"enableExternalServer":  dc.GetEnableExternalServer(),
		"chunkServerExternalIp": dc.GetListenExternalIp(),
		"chunkServerPort":       dc.GetListenPort(),
		"chunkFilePoolDir":      dataDir,
		"chunkFilePoolMetaPath": fmt.Sprintf("%s/chunkfilepool.meta", dataDir),
		"walFilePoolDir":        dataDir,
		"walFilePoolMetaPath":   fmt.Sprintf("%s/walfilepool.meta", dataDir),
		"copySetUri":            fmt.Sprintf("local://%s/copysets", dataDir),
		"recycleUri":            fmt.Sprintf("local://%s/recycler", dataDir),
		"raftLogUri":            fmt.Sprintf("curve://%s/copysets", dataDir),
		"raftSnapshotUri":       fmt.Sprintf("curve://%s/copysets", dataDir),
		"chunkServerStoreUri":   fmt.Sprintf("local://%s", dataDir),
		"chunkServerMetaUri":    fmt.Sprintf("local://%s/chunkserver.dat", dataDir),
		// brpc
		"bthread_concurrency":      18,
		"graceful_quit_on_sigterm": true,
		// raft
		"raft_sync":                            true,
		"raft_sync_meta":                       true,
		"raft_sync_segments":                   true,
		"raft_max_segment_size":                8388608,
		"raft_max_install_snapshot_tasks_num":  1,
		"raft_use_fsync_rather_than_fdatasync": false,
	}

	arguments := []string{}
	for k, v := range chunkserverArguments {
		arguments = append(arguments, fmt.Sprintf("-%s=%v", k, v))
	}
	return strings.Join(arguments, " ")
}

// getRestartPolicy chunkserver and metaserver never restart and others always start
func getRestartPolicy(dc *topology.DeployConfig) v1.RestartPolicy {
	switch dc.GetRole() {
	case topology.ROLE_ETCD,
		topology.ROLE_MDS,
		topology.ROLE_SNAPSHOTCLONE:
		return v1.RestartPolicyAlways
	}
	return v1.RestartPolicyNever
}

// newContainerPort create a container port obj
func newContainerPort(name string, containerPort, hostPort int32) v1.ContainerPort {
	return v1.ContainerPort{
		Name:          name,
		ContainerPort: containerPort,
		HostPort:      hostPort,
	}
}

// getContainerPorts get the service need to network port
func getContainerPorts(dc *topology.DeployConfig) []v1.ContainerPort {
	ports := []v1.ContainerPort{}
	ports = append(ports, newContainerPort(
		topology.CONFIG_LISTEN_PORT.Key(),
		int32(dc.GetListenPort()),
		int32(dc.GetListenPort()),
	))

	role := dc.GetRole()
	switch role {
	case topology.ROLE_ETCD:
		ports = append(ports, newContainerPort(
			topology.CONFIG_LISTEN_CLIENT_PORT.Key(),
			int32(dc.GetListenClientPort()),
			int32(dc.GetListenClientPort()),
		))
	case topology.ROLE_MDS:
		ports = append(ports, newContainerPort(
			topology.CONFIG_LISTEN_DUMMY_PORT.Key(),
			int32(dc.GetListenDummyPort()),
			int32(dc.GetListenDummyPort()),
		))
	case topology.ROLE_CHUNKSERVER:
		if dc.GetEnableExternalServer() {
			ports = append(ports, newContainerPort(
				topology.CONFIG_LISTEN_EXTERNAL_PORT.Key(),
				int32(dc.GetListenExternalPort()),
				int32(dc.GetListenExternalPort()),
			))
		}
	case topology.ROLE_SNAPSHOTCLONE:
		ports = append(ports, newContainerPort(
			topology.CONFIG_LISTEN_DUMMY_PORT.Key(),
			int32(dc.GetListenDummyPort()),
			int32(dc.GetListenDummyPort()),
		))
	case topology.ROLE_METASERVER:
		if dc.GetEnableExternalServer() {
			ports = append(ports, newContainerPort(
				topology.CONFIG_LISTEN_EXTERNAL_PORT.Key(),
				int32(dc.GetListenExternalPort()),
				int32(dc.GetListenExternalPort()),
			))
		}
	}

	return ports
}
