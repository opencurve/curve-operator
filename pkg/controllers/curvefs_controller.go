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

// CurvefsReconciler reconciles a Curvefs object
type CurvefsReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	ClusterController *ClusterController
}

func NewCurvefsReconciler(
	client client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	context clusterd.Context,
) *CurvefsReconciler {
	return &CurvefsReconciler{
		Client: client,
		Log:    log,
		Scheme: scheme,
		ClusterController: &ClusterController{
			context:    context,
			clusterMap: make(map[string]*daemon.Cluster),
		},
	}
}

// +kubebuilder:rbac:groups=operator.curve.io,resources=curvefs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.curve.io,resources=curvefs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create;update;get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *CurvefsReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log.WithValues("curve FS cluster", req.NamespacedName)

	logger.Info("reconcileing CurvefsCluster")

	r.ClusterController.context.Client = r.Client
	r.ClusterController.namespacedName = req.NamespacedName

	var curvefsCluster curvev1.Curvefs
	err := r.Client.Get(ctx, req.NamespacedName, &curvefsCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Error(err, "curvefs resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, errors.Wrap(err, "failed to get curvefs Cluster")
	}

	// Set a finalizer so we can do cleanup before the object goes away
	err = AddFinalizerIfNotPresent(context.Background(), r.Client, &curvefsCluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Delete: the CR was deleted
	if !curvefsCluster.GetDeletionTimestamp().IsZero() {
		return r.reconcileCurvefsDelete(&curvefsCluster)
	}

	ownerInfo := k8sutil.NewOwnerInfo(&curvefsCluster, r.Scheme)
	if err := r.ClusterController.reconcileCurvefsCluster(&curvefsCluster, ownerInfo); err != nil {
		k8sutil.UpdateFSCondition(context.TODO(), &r.ClusterController.context, r.ClusterController.namespacedName, curvev1.ConditionTypeFailure, curvev1.ConditionTrue, curvev1.ConditionReconcileFailed, "Reconcile curvecluster failed")
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile cluster %q", curvefsCluster.Name)
	}

	k8sutil.UpdateFSCondition(context.TODO(), &r.ClusterController.context, r.ClusterController.namespacedName, curvev1.ConditionTypeClusterReady, curvev1.ConditionTrue, curvev1.ConditionReconcileSucceeded, "Reconcile curvecluster successed")

	return ctrl.Result{}, nil
}

// reconcileDelete
func (r *CurvefsReconciler) reconcileCurvefsDelete(curvefsCluster *curvev1.Curvefs) (reconcile.Result, error) {
	log.Log.Info("Delete the cluster CR now", "namespace", curvefsCluster.ObjectMeta.Name)
	k8sutil.UpdateFSCondition(context.TODO(), &r.ClusterController.context, r.ClusterController.namespacedName, curvev1.ConditionTypeDeleting, curvev1.ConditionTrue, curvev1.ConditionDeletingClusterReason, "Reconcile curvecluster deleting")

	daemonHosts, _ := k8sutil.GetValidFSDaemonHosts(r.ClusterController.context, curvefsCluster)
	if curvefsCluster.Spec.CleanupConfirm == "Confirm" || curvefsCluster.Spec.CleanupConfirm == "confirm" {
		go r.ClusterController.startClusterCleanUp(r.ClusterController.context, curvefsCluster.Namespace, daemonHosts)
	}

	// Delete it from clusterMap
	if _, ok := r.ClusterController.clusterMap[curvefsCluster.Namespace]; ok {
		delete(r.ClusterController.clusterMap, curvefsCluster.Namespace)
	}
	// Remove finalizers
	err := removeFinalizer(r.Client, r.ClusterController.namespacedName, curvefsCluster, "")
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to remove curve fs cluster cr finalizers")
	}

	logger.Infof("curve cluster %v deleted", curvefsCluster.Name)

	return reconcile.Result{}, nil
}

// reconcileCurveCluster
func (c *ClusterController) reconcileCurvefsCluster(clusterObj *curvev1.Curvefs, ownerInfo *k8sutil.OwnerInfo) error {
	// one cr cluster in one namespace is allowed
	cluster, ok := c.clusterMap[clusterObj.Namespace]
	if !ok {
		logger.Info("A new curve FS Cluster will be created!!!")
		cluster = newCluster(config.KIND_CURVEFS, false)
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
	cluster.Metaserver = clusterObj.Spec.MetaServer

	cluster.HostDataDir = clusterObj.Spec.HostDataDir
	cluster.DataDirHostPath = path.Join(clusterObj.Spec.HostDataDir, "data")
	cluster.LogDirHostPath = path.Join(clusterObj.Spec.HostDataDir, "logs")
	cluster.ConfDirHostPath = path.Join(clusterObj.Spec.HostDataDir, "conf")

	c.clusterMap[cluster.Namespace] = cluster

	log.Log.Info("reconcileing Curve FS Cluster in namespace", "namespace", cluster.Namespace)

	// Start the main Curve cluster orchestration
	return c.initCluster(cluster)
}

func (r *CurvefsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&curvev1.Curvefs{}).
		Complete(r)
}
