package service

import (
	"strings"
	"time"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterCleanUpPolicyRetryInterval = 5 * time.Second

	CURVE_DATA_DIR_HOST_PATH = "CURVE_DATA_DIR_HOST_PATH"
	CURVE_LOG_DIR_HOST_PATH  = "CURVE_LOG_DIR_HOST_PATH"
)

var (
	CURVE_CLEAN_UP_APP_NAME = "curve-cleanup-%s"
	CURVE_CLEAN_UP_POD_NAME = "curve-cleanup"
)

func StartClusterCleanUpJob(cluster clusterd.Clusterer, dcs []*topology.DeployConfig) error {
	labels := map[string]string{"app": CURVE_CLEAN_UP_APP_NAME}
	securityContext := k8sutil.PrivilegedContext(true)

	commandLine := `rm -rf ${CURVE_DATA_DIR_HOST_PATH} && rm -rf ${CURVE_LOG_DIR_HOST_PATH} `

	for _, dc := range dcs {
		vols, volMounts := getServiceHostPathVolumeAndMount(dc)
		jobName := k8sutil.TruncateNodeNameForJob(CURVE_CLEAN_UP_APP_NAME, dc.GetHost())
		container := v1.Container{
			Name:            CURVE_CLEAN_UP_POD_NAME,
			Image:           cluster.GetContainerImage(),
			ImagePullPolicy: v1.PullIfNotPresent,
			Command: []string{
				"/bin/bash",
				"-c",
			},
			Args: []string{
				commandLine,
			},
			Env: []v1.EnvVar{
				{Name: CURVE_DATA_DIR_HOST_PATH, Value: strings.TrimRight(dc.GetDataDir(), "/")},
				{Name: CURVE_LOG_DIR_HOST_PATH, Value: strings.TrimRight(dc.GetLogDir(), "/")},
			},
			VolumeMounts:    volMounts,
			SecurityContext: securityContext,
		}
		podTempalteSpec := v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:   jobName,
				Labels: labels,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					container,
				},
				Volumes:       vols,
				RestartPolicy: v1.RestartPolicyOnFailure,
				NodeName:      dc.GetHost(),
			},
		}

		ttlTimeout := int32(0)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      CURVE_CLEAN_UP_POD_NAME,
				Namespace: cluster.GetNameSpace(),
				Labels:    labels,
			},
			Spec: batchv1.JobSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
				Template:                podTempalteSpec,
				TTLSecondsAfterFinished: &ttlTimeout, // delete itself immediately after finished.
			},
		}
		err := k8sutil.RunReplaceableJob(cluster.GetContext().Clientset, job, true)
		if err != nil {
			return err
		}
	}

	return nil
}
