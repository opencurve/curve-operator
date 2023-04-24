/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"path"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/config"
	"github.com/opencurve/curve-operator/pkg/daemon"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

// ClusterController controls an instance of a Curve Cluster
type ClusterController struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	clusterMap     map[string]*daemon.Cluster
}

// CurveClusterReconciler reconciles a CurveCluster object
type CurveClusterReconciler struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	ClusterController *ClusterController
}

func NewCurveClusterReconciler(
	client client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	context clusterd.Context,
) *CurveClusterReconciler {
	return &CurveClusterReconciler{
		Client: client,
		Log:    log,
		Scheme: scheme,
		ClusterController: &ClusterController{
			context:    context,
			clusterMap: make(map[string]*daemon.Cluster),
		},
	}
}

// +kubebuilder:rbac:groups=operator.curve.io,resources=curveclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.curve.io,resources=curveclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create;update;get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *CurveClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("curvecluster", req.NamespacedName)

	log.Info("reconcileing CurveCluster")

	r.ClusterController.context.Client = r.Client
	r.ClusterController.namespacedName = req.NamespacedName

	// Fetch the curveCluster instance
	var curveCluster curvev1.CurveCluster
	err := r.Client.Get(ctx, req.NamespacedName, &curveCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Arrive it represent the cluster has been delete
			log.Error(err, "curveCluster resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, errors.Wrap(err, "failed to get curveCluster")
	}

	// Set a finalizer so we can do cleanup before the object goes away
	err = AddFinalizerIfNotPresent(context.Background(), r.Client, &curveCluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Delete: the CR was deleted
	if !curveCluster.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(&curveCluster)
	}

	ownerInfo := k8sutil.NewOwnerInfo(&curveCluster, r.Scheme)
	// reconcileCurveCluster func to run reconcile curve cluster
	if err := r.ClusterController.reconcileCurveCluster(&curveCluster, ownerInfo); err != nil {
		k8sutil.UpdateCondition(context.TODO(), &r.ClusterController.context, r.ClusterController.namespacedName, curvev1.ConditionTypeFailure, curvev1.ConditionTrue, curvev1.ConditionReconcileFailed, "Reconcile curvecluster failed")
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile cluster %q", curveCluster.Name)
	}

	k8sutil.UpdateCondition(context.TODO(), &r.ClusterController.context, r.ClusterController.namespacedName, curvev1.ConditionTypeClusterReady, curvev1.ConditionTrue, curvev1.ConditionReconcileSucceeded, "Reconcile curvecluster successed")

	return ctrl.Result{}, nil
}

// reconcileDelete
func (r *CurveClusterReconciler) reconcileDelete(curveCluster *curvev1.CurveCluster) (reconcile.Result, error) {
	log.Log.Info("Delete the cluster CR now", "namespace", curveCluster.ObjectMeta.Name)
	k8sutil.UpdateCondition(context.TODO(), &r.ClusterController.context, r.ClusterController.namespacedName, curvev1.ConditionTypeDeleting, curvev1.ConditionTrue, curvev1.ConditionDeletingClusterReason, "Reconcile curvecluster deleting")

	if curveCluster.Spec.CleanupConfirm == "Confirm" || curveCluster.Spec.CleanupConfirm == "confirm" {
		daemonHosts, _ := k8sutil.GetValidDaemonHosts(r.ClusterController.context, curveCluster)
		chunkserverHosts, _ := k8sutil.GetValidChunkserverHosts(r.ClusterController.context, curveCluster)
		nodesForJob := k8sutil.MergeNodesOfDaemonAndChunk(daemonHosts, chunkserverHosts)

		go r.ClusterController.startClusterCleanUp(r.ClusterController.context, curveCluster.Namespace, nodesForJob)
	}

	// Delete it from clusterMap
	if _, ok := r.ClusterController.clusterMap[curveCluster.Namespace]; ok {
		delete(r.ClusterController.clusterMap, curveCluster.Namespace)
	}
	// Remove finalizers
	err := removeFinalizer(r.Client, r.ClusterController.namespacedName, curveCluster, "")
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to remove curvecluster cr finalizers")
	}

	logger.Infof("curve cluster %v deleted", curveCluster.Name)

	return reconcile.Result{}, nil
}

// reconcileCurveCluster
func (c *ClusterController) reconcileCurveCluster(clusterObj *curvev1.CurveCluster, ownerInfo *k8sutil.OwnerInfo) error {
	// one cr cluster in one namespace is allowed
	cluster, ok := c.clusterMap[clusterObj.Namespace]
	if !ok {
		logger.Info("A new curve BS Cluster will be created!!!")
		cluster = newCluster(config.KIND_CURVEBS, false)
		// TODO: update cluster spec if the cluster has already exist!
	} else {
		logger.Info("Cluster has been exist but need configured but we don't apply it now, you need delete it and recreate it!!!namespace=%q", cluster.Namespace)
		return nil
	}

	// Set the context and NameSpacedName
	cluster.Context = c.context
	cluster.Namespace = c.namespacedName.Namespace
	cluster.NamespacedName = c.namespacedName
	cluster.ObservedGeneration = clusterObj.ObjectMeta.Generation
	cluster.OwnerInfo = ownerInfo
	// Set the spec
	cluster.Nodes = clusterObj.Spec.Nodes
	cluster.CurveVersion = clusterObj.Spec.CurveVersion
	cluster.Etcd = clusterObj.Spec.Etcd
	cluster.Mds = clusterObj.Spec.Mds
	cluster.SnapShotClone = clusterObj.Spec.SnapShotClone
	cluster.Chunkserver = clusterObj.Spec.Storage

	cluster.HostDataDir = clusterObj.Spec.HostDataDir
	cluster.DataDirHostPath = path.Join(clusterObj.Spec.HostDataDir, "data")
	cluster.LogDirHostPath = path.Join(clusterObj.Spec.HostDataDir, "logs")
	cluster.ConfDirHostPath = path.Join(clusterObj.Spec.HostDataDir, "conf")
	c.clusterMap[cluster.Namespace] = cluster

	log.Log.Info("reconcileing CurveCluster in namespace", "namespace", cluster.Namespace)

	return c.initCluster(cluster)
}

// initCluster initialize cluster info
func (c *ClusterController) initCluster(cluster *daemon.Cluster) error {
	err := preClusterStartValidation(cluster)
	if err != nil {
		return errors.Wrap(err, "failed to preforem validation before cluster creation")
	}
	if cluster.Kind == config.KIND_CURVEBS {
		err = reconcileCurveDaemons(cluster)
	} else {
		err = reconcileCurveFSDaemons(cluster)
	}
	if err != nil {
		return nil
	}

	return nil
}

// preClusterStartValidation Cluster Spec validation
func preClusterStartValidation(cluster *daemon.Cluster) error {
	// Assert the node num is 3
	nodesNum := len(cluster.Nodes)
	if nodesNum < 3 {
		return errors.Errorf("nodes count shoule at least 3, cannot start cluster %d", len(cluster.Nodes))
	} else if nodesNum > 3 {
		return errors.Errorf("nodes count more than 3, cannot start cluster temporary %d", len(cluster.Nodes))
	}

	return nil
}

func (r *CurveClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&curvev1.CurveCluster{}).
		Complete(r)
}
