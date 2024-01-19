package controllers

import (
	"bytes"
	"fmt"
	"path"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
	"github.com/pkg/errors"
)

const (
	CURVE_DUMMY_SERVICE   = "curve-dummy-service"
	CURVE_CONFIG_TEMPLATE = "curve-config-template"
)

func getDummyServiceLabels() map[string]string {
	labels := make(map[string]string)
	labels["app"] = CURVE_DUMMY_SERVICE
	return labels
}

// createSyncDeployment create a deployment for read config file
func makeDummyDeployment(c clusterd.Clusterer, dcs []*topology.DeployConfig) error {
	container := v1.Container{
		Name: CURVE_DUMMY_SERVICE,
		Command: []string{
			"/bin/bash",
		},
		Args: []string{
			"-c",
			"while true; do echo sync pod to read various config file from it; sleep 10;done",
		},
		Image:           dcs[0].GetContainerImage(),
		ImagePullPolicy: v1.PullIfNotPresent,
		Env:             []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   CURVE_DUMMY_SERVICE,
			Labels: getDummyServiceLabels(),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				container,
			},
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			NodeName:      dcs[0].GetHost(),
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
		},
	}

	replicas := int32(1)
	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CURVE_DUMMY_SERVICE,
			Namespace: c.GetNameSpace(),
			Labels:    getDummyServiceLabels(),
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getDummyServiceLabels(),
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: apps.DeploymentStrategy{
				Type: apps.RollingUpdateDeploymentStrategyType,
			},
		},
	}

	if err := c.GetOwnerInfo().SetControllerReference(d); err != nil {
		return err
	}

	if _, err := k8sutil.CreateOrUpdateDeploymentAndWaitStart(c.GetContext().Clientset, d); err != nil {
		return err
	}

	// update condition type and phase etc.
	return nil
}

// makeTemplateConfigMap make a configmap store all config file with template value
func makeTemplateConfigMap(c clusterd.Clusterer, dcs []*topology.DeployConfig) error {
	configMapData, err := getDefaultConfigMapData(c, dcs)
	if err != nil {
		return err
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CURVE_CONFIG_TEMPLATE,
			Namespace: c.GetNameSpace(),
		},
		Data: configMapData,
	}

	err = c.GetOwnerInfo().SetControllerReference(cm)
	if err != nil {
		return err
	}

	_, err = k8sutil.CreateOrUpdateConfigMap(c.GetContext().Clientset, cm)
	if err != nil {
		return err
	}

	logger.Infof("create configmap %s successed", CURVE_CONFIG_TEMPLATE)
	return nil
}

// getDefaultConfigMapData read all config files with template value
func getDefaultConfigMapData(c clusterd.Clusterer, dcs []*topology.DeployConfig) (map[string]string, error) {
	labels := getDummyServiceLabels()
	selector := k8sutil.GetLabelSelector(labels)
	pods, err := k8sutil.GetPodsByLabelSelector(c.GetContext().Clientset, c.GetNameSpace(), selector)
	if err != nil {
		return nil, err
	}

	if len(pods.Items) != 1 {
		return nil, errors.New("app=sync-config label matches no pods")
	}
	pod := pods.Items[0]

	role2Configs := map[string][]string{}
	// distinct the same config
	for _, dc := range dcs {
		role2Configs[dc.GetRole()] = topology.ServiceConfigs[dc.GetRole()]
	}
	// for tool.conf
	role2Configs["tools.conf"] = []string{
		topology.LAYOUT_TOOLS_NAME,
	}

	confSrcDir := dcs[0].GetProjectLayout().ServiceConfSrcDir // /curvefs/conf

	configMapData := make(map[string]string)
	for _, confNames := range role2Configs {
		for _, confName := range confNames {
			confSrcPath := path.Join(confSrcDir, confName) // /curvefs/conf/mds.conf
			configMapData[confName], err = readConfigFromDummyPod(c, &pod, confSrcPath)
			if err != nil {
				return nil, err
			}
		}
	}

	return configMapData, nil
}

// readConfigFromDummyPod read content (config file) from dummy pod
func readConfigFromDummyPod(c clusterd.Clusterer, pod *v1.Pod, configPath string) (string, error) {
	logger.Infof("syncing %v", configPath)
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)

	req := c.GetContext().Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Container: pod.Spec.Containers[0].Name,
		Command:   []string{"cat", configPath},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.GetContext().KubeConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to init executor: %v", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})

	if err != nil {
		return "", fmt.Errorf("could not execute: %v", err)
	}

	if execErr.Len() > 0 {
		return "", fmt.Errorf("stderr: %v", execErr.String())
	}

	cmdOutput := execOut.String()
	return cmdOutput, nil
}

// // createGrafanaConfigMapTemplate copy grafana dashborads source to grafana container
// func createGrafanaConfigMapTemplate(c *daemon.Cluster) error {
// 	labels := getReadConfigJobLabel(c)
// 	selector := k8sutil.GetLabelSelector(labels)
// 	pods, err := k8sutil.GetPodsByLabelSelector(c.Context.Clientset, c.Namespace, selector)
// 	if err != nil {
// 		return err
// 	}

// 	if len(pods.Items) != 1 {
// 		return errors.New("app=sync-config label matches no pods")
// 	}
// 	pod := pods.Items[0]

// 	configMapData := make(map[string]string)

// 	var pathPrefix string
// 	var dashboards []string
// 	if c.Kind == config.KIND_CURVEBS {
// 		pathPrefix = "/curvebs/monitor/grafana"
// 		dashboards = GrafanaDashboardsConfigs
// 	} else {
// 		pathPrefix = "/curvefs/monitor/grafana"
// 		dashboards = FSGrafanaDashboardsConfigs
// 	}

// 	for _, name := range dashboards {
// 		configPath := pathPrefix
// 		if name != "grafana.ini" {
// 			configPath = path.Join(pathPrefix, "/provisioning/dashboards")
// 		}
// 		configPath = path.Join(configPath, name)
// 		content, err := readConfigFromContainer(c, pod, configPath)
// 		if err != nil {
// 			return err
// 		}

// 		configMapData[name] = content
// 	}

// 	cm := &v1.ConfigMap{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      config.GrafanaDashboardsTemp,
// 			Namespace: c.Namespace,
// 		},
// 		Data: configMapData,
// 	}

// 	err = c.OwnerInfo.SetControllerReference(cm)
// 	if err != nil {
// 		return errors.Wrapf(err, "failed to set owner reference to configmap %q", config.GrafanaDashboardsTemp)
// 	}

// 	// create configmap in cluster
// 	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
// 	if err != nil && !kerrors.IsAlreadyExists(err) {
// 		return errors.Wrapf(err, "failed to create configmap %s", config.GrafanaDashboardsTemp)
// 	}

// 	return nil
// }
