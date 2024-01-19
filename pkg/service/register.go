package service

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
)

const (
	POOL_TYPE_PHYSICAL = "physicalpool"
	POOL_TYPE_LOGICAL  = "logicalpool"

	WAIT_MDS_ELECTION_CONTAINER      = "wait-mds-election-container"
	WAIT_CHUNKSERVER_START_CONTAINER = "wait-chunkserver-start-container"
)

var (
	CURVE_CREATE_POOL_JOB = "curve-create-%s"
)

// StartJobCreatePool create job to create physicalpool or logicalpool
func StartJobCreatePool(cluster clusterd.Clusterer, dc *topology.DeployConfig, dcs []*topology.DeployConfig, poolType string) error {
	// create or update CURVE_TOPOLOGY_CONFIGMAP configmap that store cluster pool json
	err := createOrUpdatePoolConfigMap(cluster, dcs)
	if err != nil {
		return err
	}

	// security context
	privileged := true
	runAsUser := int64(0)
	runAsNonRoot := false
	readOnlyRootFilesystem := false

	vols, volMounts := getToolsAndTopoVolumeAndMount(dc)
	poolJsonPath := path.Join(dc.GetProjectLayout().ToolsConfDir, TOPO_JSON_FILE_NAME)
	container := v1.Container{
		Name: fmt.Sprintf(CURVE_CREATE_POOL_JOB, poolType),
		Command: []string{
			genCreatePoolCommand(dc, poolType, poolJsonPath),
		},
		Image:           cluster.GetContainerImage(),
		ImagePullPolicy: v1.PullIfNotPresent,
		VolumeMounts:    volMounts,
		SecurityContext: &v1.SecurityContext{
			Privileged:             &privileged,
			RunAsUser:              &runAsUser,
			RunAsNonRoot:           &runAsNonRoot,
			ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		},
	}

	initContianers, err := makeCreatePoolJobInitContainers(cluster, dcs, poolType)
	if err != nil {
		return err
	}

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf(CURVE_CREATE_POOL_JOB, poolType),
			Labels: getCreatePoolJobLabel(poolType),
		},
		Spec: v1.PodSpec{
			InitContainers: initContianers,
			Containers: []v1.Container{
				container,
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       vols,
		},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(CURVE_CREATE_POOL_JOB, poolType),
			Namespace: cluster.GetNameSpace(),
			Labels:    getCreatePoolJobLabel(poolType),
		},
		Spec: batchv1.JobSpec{
			Template: podSpec,
		},
	}

	err = cluster.GetOwnerInfo().SetControllerReference(job)
	if err != nil {
		return err
	}

	err = k8sutil.RunReplaceableJob(cluster.GetContext().Clientset, job, true)
	return err
}

// getCreatePoolJobLabel return curve-create-pool Pod and Deployment label
func getCreatePoolJobLabel(poolType string) map[string]string {
	labels := map[string]string{}
	labels["app"] = "create-pool"
	labels["type"] = poolType
	return labels
}

// genCreatePoolCommand generate create pool command by cluster kind and poolType parameter
func genCreatePoolCommand(dc *topology.DeployConfig, pooltype, poolJSONPath string) string {
	layout := dc.GetProjectLayout()
	toolsBinaryPath := layout.ToolsBinaryPath
	if dc.GetKind() == topology.KIND_CURVEFS {
		// for curvefs, the default topology json path is current directory's topology.json
		return fmt.Sprintf("%s create-topology", toolsBinaryPath)
	}

	return fmt.Sprintf("%s -op=create_%s -cluster_map=%s",
		toolsBinaryPath, pooltype, poolJSONPath)
}

// makeCreatePoolJobInitContainers create two init container to precheck work
// 1. wait mds leader election success(bs and fs)
// 2. wait chunkservers online before create logical pool(bs)
func makeCreatePoolJobInitContainers(cluster clusterd.Clusterer, dcs []*topology.DeployConfig, poolType string) ([]v1.Container, error) {
	containers := []v1.Container{}
	clusterMdsAddr, err := dcs[0].GetVariables().Get("cluster_mds_addr")
	if err != nil {
		return nil, err
	}
	clusterMdsAddr = strings.Replace(clusterMdsAddr, ",", " ", -1)

	wait_mds_election_container := v1.Container{
		Name: WAIT_MDS_ELECTION_CONTAINER,
		Command: []string{
			"bash",
			"-c",
			wait_mds_election,
		},
		Image:           cluster.GetContainerImage(),
		ImagePullPolicy: v1.PullIfNotPresent,
		Env: []v1.EnvVar{
			{
				Name:  "CLUSTER_MDS_ADDR",
				Value: clusterMdsAddr,
			},
		},
	}

	containers = append(containers, wait_mds_election_container)

	if dcs[0].GetKind() == topology.KIND_CURVEBS && poolType == POOL_TYPE_LOGICAL {
		nChunkserver := len(topology.FilterDeployConfigByRole(dcs, topology.ROLE_CHUNKSERVER))
		wait_chunkserver_start_container := v1.Container{
			Name: WAIT_CHUNKSERVER_START_CONTAINER,
			Command: []string{
				"bash",
				"-c",
				wait_chunkserver_start,
			},
			Image:           cluster.GetContainerImage(),
			ImagePullPolicy: v1.PullIfNotPresent,
			Env: []v1.EnvVar{
				{
					Name:  "CHUNKSERVER_NUMS",
					Value: strconv.Itoa(nChunkserver),
				},
			},
		}
		containers = append(containers, wait_chunkserver_start_container)
	}

	return containers, nil
}
