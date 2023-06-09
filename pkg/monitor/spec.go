package monitor

import (
	"fmt"
	"path"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/pkg/errors"
)

// createPrometheusConfigMap create prometheus.yml configmap and mount to prometheus container
func (c *Cluster) createPrometheusConfigMap(targetJson string, nodeIPs []string) error {
	configMapData := make(map[string]string)
	nodeExporterEndpoints := c.getExporterEndpoints(nodeIPs)
	prometheusYamlContent := fmt.Sprintf(PROMETHEUS_YML, c.Monitor.Prometheus.ListenPort, nodeExporterEndpoints)
	configMapData[config.PrometheusConfigMapDataKey] = prometheusYamlContent
	configMapData[TargetJSONDataKey] = targetJson

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.PrometheusConfigMapName,
			Namespace: c.Namespace,
		},
		Data: configMapData,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to configmap %q", config.PrometheusConfigMapName)
	}

	// create configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", config.PrometheusConfigMapName)
	}
	return nil
}

// makePrometheusDeployment make prometheus deployment
func (c *Cluster) makePrometheusDeployment() (*apps.Deployment, error) {
	dataPath := &config.DataPathMap{
		HostDataDir:      c.Monitor.Prometheus.DataDir,
		ContainerDataDir: PrometheusTSDBPath,
	}
	volumes := daemon.DaemonVolumes("", PrometheusConfPath, dataPath, config.PrometheusConfigMapName)

	runAsUser := int64(0)
	runAsNonRoot := false

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   PromAppName,
			Labels: prometheusLabels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.createPrometheusContainer(dataPath),
			},
			NodeName:      c.Monitor.MonitorHost,
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			Volumes:       volumes,
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:    &runAsUser,
				RunAsNonRoot: &runAsNonRoot,
			},
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PromAppName,
			Namespace: c.NamespacedName.Namespace,
			Labels:    prometheusLabels,
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: prometheusLabels,
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
		},
	}

	// set ownerReference
	err := c.OwnerInfo.SetControllerReference(d)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set owner reference to mon deployment %q", d.Name)
	}

	return d, nil
}

// createPrometheusContainer create prometheus container
func (c *Cluster) createPrometheusContainer(dataPath *config.DataPathMap) v1.Container {
	// construct start parameters
	argsMap := map[string]string{
		"config.file":                 path.Join(PrometheusConfPath, config.PrometheusConfigMapDataKey),
		"storage.tsdb.path":           PrometheusTSDBPath,
		"storage.tsdb.retention.time": c.Monitor.Prometheus.RetentionTime,
		"storage.tsdb.retention.size": c.Monitor.Prometheus.RetentionSize,
		"web.listen-address":          fmt.Sprint(":", c.Monitor.Prometheus.ListenPort),
	}
	args := []string{}
	for k, v := range argsMap {
		var item string
		if v != "" {
			item = fmt.Sprintf("--%s=%v", k, v)
		} else {
			item = fmt.Sprintf("--%s", k)
		}
		args = append(args, item)
	}

	container := v1.Container{
		Name:            PromAppName,
		Image:           c.Monitor.Prometheus.ContainerImage,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		Args:            args,
		VolumeMounts:    daemon.DaemonVolumeMounts("", PrometheusConfPath, dataPath, config.PrometheusConfigMapName),
		Ports: []v1.ContainerPort{
			{
				Name:          "prometheus-port",
				ContainerPort: int32(c.Monitor.Prometheus.ListenPort),
				HostPort:      int32(c.Monitor.Prometheus.ListenPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}
	return container
}

// createGrafanaConfigMap create grafana datasource configmap all.yml
func (c *Cluster) createGrafanaConfigMap() error {
	configMapData := make(map[string]string)
	content := fmt.Sprintf(GRAFANA_DATA_SOURCE, "127.0.0.1", c.Monitor.Prometheus.ListenPort)
	configMapData[config.GrafanaDataSourcesConfigMapDataKey] = content

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.GrafanaDataSourcesConfigMapName,
			Namespace: c.Namespace,
		},
		Data: configMapData,
	}

	err := c.OwnerInfo.SetControllerReference(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to configmap %q", config.PrometheusConfigMapName)
	}

	// create configmap in cluster
	_, err = c.Context.Clientset.CoreV1().ConfigMaps(c.NamespacedName.Namespace).Create(cm)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create configmap %s", config.PrometheusConfigMapName)
	}

	return nil
}

// makeGrafanaDeployment make grafana deployment
func (c *Cluster) makeGrafanaDeployment() (*apps.Deployment, error) {
	dataPath := &config.DataPathMap{
		HostDataDir:      c.Monitor.Grafana.DataDir,
		ContainerDataDir: GrafanaContainerDataPath,
	}
	volumes := daemon.DaemonVolumes(config.GrafanaDataSourcesConfigMapDataKey, config.GrafanaDataSourcesConfigMapMountPath, dataPath, config.GrafanaDataSourcesConfigMapName)
	vols := daemon.DaemonVolumes("", config.GrafanaDashboardsMountPath, nil, config.GrafanaDashboardsTemp)
	volumes = append(volumes, vols...)
	vols = daemon.DaemonVolumes(config.GrafanaINIConfigMapDataKey, config.GrafanaINIConfigMountPath, nil, config.GrafanaDashboardsTemp)
	volumes = append(volumes, vols...)

	containers := []v1.Container{c.createGrafaContainer(dataPath)}
	deploymentConfig := k8sutil.DeploymentConfig{Name: GrafanaAppName, NodeName: c.Monitor.MonitorHost, Namespace: c.NamespacedName.Namespace,
		Labels: grafanaLables, Volumes: volumes, Containers: containers, OwnerInfo: c.OwnerInfo}
	d, err := k8sutil.MakeDeployment(deploymentConfig)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// createGrafaContainer create grafana container
func (c *Cluster) createGrafaContainer(dataPath *config.DataPathMap) v1.Container {
	volMounts := daemon.DaemonVolumeMounts(config.GrafanaDataSourcesConfigMapDataKey, config.GrafanaDataSourcesConfigMapMountPath, dataPath, config.GrafanaDataSourcesConfigMapName)
	volM := daemon.DaemonVolumeMounts("", config.GrafanaDashboardsMountPath, nil, config.GrafanaDashboardsTemp)
	volMounts = append(volMounts, volM...)
	volM = daemon.DaemonVolumeMounts(config.GrafanaINIConfigMapDataKey, config.GrafanaINIConfigMountPath, nil, config.GrafanaDashboardsTemp)
	volMounts = append(volMounts, volM...)

	container := v1.Container{
		Name:            GrafanaAppName,
		Image:           c.Monitor.Grafana.ContainerImage,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		VolumeMounts:    volMounts,
		Ports: []v1.ContainerPort{
			{
				Name:          "grafana-port",
				ContainerPort: int32(c.Monitor.Grafana.ListenPort),
				HostPort:      int32(c.Monitor.Grafana.ListenPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{
			{Name: "TZ", Value: "Asia/Hangzhou"},
			{Name: "GF_SECURITY_ADMIN_USER", Value: c.Monitor.Grafana.UserName},
			{Name: "GF_SECURITY_ADMIN_PASSWORD", Value: c.Monitor.Grafana.PassWord},
		},
	}
	return container
}

func (c *Cluster) makeNodeExporterDeployment(nodeName string) (*apps.Deployment, error) {
	runAsUser := int64(0)
	runAsNonRoot := false

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   NodeExporterAppName,
			Labels: nodeExporterLabels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				c.createNodeExporterContainer(),
			},
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyAlways,
			HostNetwork:   true,
			DNSPolicy:     v1.DNSClusterFirstWithHostNet,
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:    &runAsUser,
				RunAsNonRoot: &runAsNonRoot,
			},
		},
	}

	replicas := int32(1)

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprint(NodeExporterAppName, "-", nodeName),
			Namespace: c.NamespacedName.Namespace,
			Labels:    nodeExporterLabels,
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: nodeExporterLabels,
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
		},
	}

	// set ownerReference
	err := c.OwnerInfo.SetControllerReference(d)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set owner reference to mon deployment %q", d.Name)
	}

	return d, nil
}

func (c *Cluster) createNodeExporterContainer() v1.Container {
	// construct start parameters
	argsMap := map[string]interface{}{
		"path.rootfs":        "/host",
		"collector.cpu.info": nil,
		"web.listen-address": fmt.Sprintf(":%d", c.Monitor.NodeExporter.ListenPort),
	}
	args := []string{}
	for k, v := range argsMap {
		var item string
		if v != nil {
			item = fmt.Sprintf("--%s=%v", k, v)
		} else {
			item = fmt.Sprintf("--%s", k)
		}
		args = append(args, item)
	}

	container := v1.Container{
		Name:            NodeExporterAppName,
		Image:           c.Monitor.NodeExporter.ContainerImage,
		ImagePullPolicy: c.CurveVersion.ImagePullPolicy,
		Args:            args,
		Ports: []v1.ContainerPort{
			{
				Name:          "exporter-port",
				ContainerPort: int32(c.Monitor.NodeExporter.ListenPort),
				HostPort:      int32(c.Monitor.NodeExporter.ListenPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{{Name: "TZ", Value: "Asia/Hangzhou"}},
	}
	return container
}
