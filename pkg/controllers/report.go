package controllers

import (
	"fmt"
	"path"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createReportConfigMap(c *daemon.Cluster) error {
	configMapData := map[string]string{
		config.ReportConfigMapDataKey: REPORT,
	}
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ReportConfigMapName,
			Namespace: c.Namespace,
		},
		Data: configMapData,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return err
	}

	// Create topology-json-conf configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create %s configmap in namespace %s", cm.Name, c.Namespace)
	}

	logger.Info("create report configmap successfully")
	return nil
}

func runReportCronJob(c *daemon.Cluster, snapshotEnable bool) error {
	reportPath := path.Join("/", c.GetKind(), config.ReportConfigMapMountPathCommon)
	reportFilePath := path.Join(reportPath, config.ReportConfigMapDataKey)
	vols := daemon.DaemonVolumes(config.ToolsConfigMapDataKey, config.ToolsConfigMapMountPathDir, nil, config.ToolsConfigMapName)
	vols = append(vols, daemon.DaemonVolumes(config.ReportConfigMapDataKey, reportPath, nil, config.ReportConfigMapName)...)

	volMounts := daemon.DaemonVolumeMounts(config.ToolsConfigMapDataKey, config.ToolsConfigMapMountPathDir, nil, config.ToolsConfigMapName)
	volMounts = append(volMounts, daemon.DaemonVolumeMounts(config.ReportConfigMapDataKey, reportPath, nil, config.ReportConfigMapName)...)

	// construct command line
	commandLine := fmt.Sprintf("%s %s %s %s %s %s", "bash", reportFilePath, c.GetKind(), c.GetUUID(), config.ROLE_ETCD, "&&") +
		fmt.Sprintf("%s %s %s %s %s %s", "bash", reportFilePath, c.GetKind(), c.GetUUID(), config.ROLE_MDS, "&&")
	if c.GetKind() == config.KIND_CURVEBS {
		commandLine += fmt.Sprintf("%s %s %s %s %s", "bash", reportFilePath, c.GetKind(), c.GetUUID(), config.ROLE_CHUNKSERVER)
	} else {
		commandLine += fmt.Sprintf("%s %s %s %s %s", "bash", reportFilePath, c.GetKind(), c.GetUUID(), config.ROLE_METASERVER)
	}

	if c.GetKind() == config.KIND_CURVEBS && snapshotEnable {
		commandLine += "&& " + fmt.Sprintf("%s %s %s %s %s", "bash", reportFilePath, c.GetKind(), c.GetUUID(), config.ROLE_SNAPSHOTCLONE)
	}

	container := v1.Container{
		Name:            "crontab",
		Image:           c.CurveVersion.Image,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		Command: []string{
			"/bin/bash",
			"-c",
			commandLine,
		},
		VolumeMounts: volMounts,
	}

	podSpec := v1.PodSpec{
		Containers:    []v1.Container{container},
		Volumes:       vols,
		RestartPolicy: "OnFailure",
	}

	jobSpec := batchv1.JobSpec{
		Template: v1.PodTemplateSpec{
			Spec: podSpec,
		},
	}

	reserverJobs := int32(1)

	cronjob := &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.Namespace,
			Name:      "report-crontab",
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: "0 * * * *",
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: jobSpec,
			},
			SuccessfulJobsHistoryLimit: &reserverJobs,
		},
	}

	// set ownerReference
	err := c.OwnerInfo.SetControllerReference(cronjob)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to cronJob to %q", cronjob.Name)
	}

	// create CronJob in cluster
	_, err = c.Context.Clientset.BatchV1beta1().CronJobs(c.Namespace).Create(cronjob)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create CronJob to report")
	}

	return nil
}

var REPORT = `
function rematch() {
    local s=$1 regex=$2
    if [[ $s =~ $regex ]]; then
        echo "${BASH_REMATCH[1]}"
    fi
}

function fs_usage() {
    curvefs_tool usage-metadata 2>/dev/null | awk 'BEGIN {
        BYTES["KB"] = 1024
        BYTES["MB"] = BYTES["KB"] * 1024
        BYTES["GB"] = BYTES["MB"] * 1024
        BYTES["TB"] = BYTES["GB"] * 1024
    }
    { 
        if ($0 ~ /all cluster/) {
            printf ("%0.f", $8 * BYTES[$9])
        }
    }'
}

function bs_usage() {
    local message=$(curve_ops_tool space | grep physical)
    local used=$(rematch "$message" "used = ([0-9]+)GB")
    echo $(($used*1024*1024*1024))
}

[[ -z $(which curl) ]] && apt-get install -y curl
g_kind=$1
g_uuid=$2
g_role=$3
g_usage=$(([[ $g_kind = "curvebs" ]] && bs_usage) || fs_usage)
curl -XPOST http://curveadm.aspirer.wang:19302/ \
    -d "kind=$g_kind" \
    -d "uuid=$g_uuid" \
    -d "role=$g_role" \
    -d "usage=$g_usage"
`
