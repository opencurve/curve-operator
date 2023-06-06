package monitor

import (
	"context"

	"github.com/coreos/pkg/capnslog"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/topology"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	PromAppName         = "curve-prometheus"
	GrafanaAppName      = "curve-grafana"
	NodeExporterAppName = "node-exporter"
)

const (
	// container config path
	PrometheusConfPath = "/etc/prometheus"
	// container data path
	PrometheusTSDBPath = "/prometheus"
	// target json
	TargetJSONDataKey = "target.json"
	// container grafana data path
	GrafanaContainerDataPath = "/var/lib/grafana"
)

var (
	prometheusLabels   = map[string]string{"app": "curve-prometheus"}
	grafanaLables      = map[string]string{"app": "curve-grafana"}
	nodeExporterLabels = map[string]string{"app": "node-exporter"}
)

type Cluster struct {
	*daemon.Cluster
}

func New(c *daemon.Cluster) *Cluster {
	return &Cluster{Cluster: c}
}

var logger = capnslog.NewPackageLogger("github.com/opencurve/curve-operator", "monitor")

type serviceTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// Start configure monitor for curve cluster including Prometheus, Grafana and Node-Exporter
func (c *Cluster) Start(nodesInfo []daemon.NodeInfo, dcs []*topology.DeployConfig) error {
	err := c.startPrometheus(nodesInfo, dcs)
	if err != nil {
		return err
	}

	err = c.startGrafana()
	if err != nil {
		return err
	}

	err = c.startNodeExporter(nodesInfo)
	if err != nil {
		return err
	}

	return nil
}

// startPrometheus create prometheus config and deployment then create it in cluster.
func (c *Cluster) startPrometheus(nodesInfo []daemon.NodeInfo, dcs []*topology.DeployConfig) error {
	targetJson, err := parsePrometheusTarget(dcs)
	if err != nil {
		return err
	}

	nodeIPs := filterNodeForExporter(nodesInfo)

	err = c.createPrometheusConfigMap(targetJson, nodeIPs)
	if err != nil {
		return err
	}

	d, err := c.makePrometheusDeployment()
	if err != nil {
		return err
	}

	newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(d)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create prometheus deployment %s", d.GetName())
		}
		logger.Infof("deployment for monitor %s already exists. updating if needed", d.GetName())

		// TODO:Update the daemon Deployment
		// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
		// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
		// }
	} else {
		logger.Infof("Deployment %s has been created , waiting for startup", newDeployment.GetName())
	}

	// wait deployment to start
	if err := k8sutil.WaitForDeploymentToStart(context.TODO(), &c.Context, d); err != nil {
		return err
	}

	logger.Info("Promethes deploy successed")

	return nil
}

// startGrafana create Grafana deployment and create it in cluster.
func (c *Cluster) startGrafana() error {
	err := c.createGrafanaConfigMap()
	if err != nil {
		return err
	}

	d, err := c.makeGrafanaDeployment()
	if err != nil {
		return err
	}

	newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(d)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create grafana deployment %s", d.GetName())
		}
		logger.Infof("deployment for monitor %s already exists. updating if needed", d.GetName())

		// TODO:Update the daemon Deployment
		// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
		// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
		// }
	} else {
		logger.Infof("Deployment %s has been created , waiting for startup", newDeployment.GetName())
	}

	// wait deployment to start
	if err := k8sutil.WaitForDeploymentToStart(context.TODO(), &c.Context, d); err != nil {
		return err
	}

	return nil
}

// startNodeExporter create node-exporter deployment and create it in cluster.
func (c *Cluster) startNodeExporter(nodesInfo []daemon.NodeInfo) error {
	nodeNames := filterNodeNameForExporter(nodesInfo)
	var deploymentsToWaitFor []*apps.Deployment
	for _, nodeName := range nodeNames {
		d, err := c.makeNodeExporterDeployment(nodeName)
		if err != nil {
			return err
		}
		newDeployment, err := c.Context.Clientset.AppsV1().Deployments(c.NamespacedName.Namespace).Create(d)
		if err != nil {
			if !kerrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create node exporter deployment %s", d.GetName())
			}
			logger.Infof("deployment for monitor %s already exists. updating if needed", d.GetName())

			// TODO:Update the daemon Deployment
			// if err := updateDeploymentAndWait(c.context, c.clusterInfo, d, config.MgrType, mgrConfig.DaemonID, c.spec.SkipUpgradeChecks, false); err != nil {
			// 	logger.Errorf("failed to update mgr deployment %q. %v", resourceName, err)
			// }
		} else {
			logger.Infof("Deployment %s has been created , waiting for startup", newDeployment.GetName())
			deploymentsToWaitFor = append(deploymentsToWaitFor, newDeployment)
		}
	}

	// wait all Deployments to start
	for _, d := range deploymentsToWaitFor {
		if err := k8sutil.WaitForDeploymentToStart(context.TODO(), &c.Context, d); err != nil {
			return err
		}
	}
	return nil
}
