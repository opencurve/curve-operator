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
	"github.com/opencurve/curve-operator/pkg/k8sutil"
)

// ClusterController controls an instance of a Curve Cluster
type ClusterController struct {
	context        clusterd.Context
	namespacedName types.NamespacedName
	clusterMap     map[string]*cluster
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
			clusterMap: make(map[string]*cluster),
		},
	}
}

// +kubebuilder:rbac:groups=operator.curve.io,resources=curveclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.curve.io,resources=curveclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *CurveClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("curvecluster", req.NamespacedName)

	// your logic here
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
		return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
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

	if _, ok := r.ClusterController.clusterMap[curveCluster.Namespace]; ok {
		delete(r.ClusterController.clusterMap, curveCluster.Namespace)
	}

	// Remove finalizers
	err := r.removeFinalizer(r.Client, r.ClusterController.namespacedName, curveCluster, "")
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to remove curvecluster cr finalizers")
	}
	return reconcile.Result{}, nil
}

// reconcileCurveCluster
func (c *ClusterController) reconcileCurveCluster(clusterObj *curvev1.CurveCluster, ownerInfo *k8sutil.OwnerInfo) error {
	// one cr cluster in one namespace is allowed
	cluster, ok := c.clusterMap[clusterObj.Namespace]
	if !ok {
		log.Log.Info("A new Cluster will be created!!!")
		cluster = newCluster(c.context, clusterObj, ownerInfo)
		// TODO: update cluster spec if the cluster has already exist!
	} else {
		log.Log.Info("Cluster has been exist but need configured but we don't apply it now, you need delete it and recreate it!!!", "namespace", cluster.NameSpace)
		return nil
	}

	// Set the context and NameSpacedName
	cluster.context = c.context
	cluster.NamespacedName = c.namespacedName
	cluster.NameSpace = c.namespacedName.Namespace
	// Set the spec
	cluster.Spec = &clusterObj.Spec

	// updating observedGeneration in cluster if it's not the first reconcile
	cluster.observedGeneration = clusterObj.ObjectMeta.Generation

	c.clusterMap[cluster.NameSpace] = cluster

	log.Log.Info("reconcileing CurveCluster in namespace", "namespace", cluster.NameSpace)

	// Start the main Curve cluster orchestration
	return c.initCluster(cluster)
}

// initCluster initialize cluster info
func (c *ClusterController) initCluster(cluster *cluster) error {
	err := preClusterStartValidation(cluster)
	if err != nil {
		return errors.Wrap(err, "failed to preforem validation before cluster creation")
	}
	err = cluster.reconcileCurveDaemons()
	if err != nil {
		return errors.Wrap(err, "failed to create cluster")
	}
	return nil
}

// preClusterStartValidation Cluster Spec validation
func preClusterStartValidation(cluster *cluster) error {
	// Assert the node num is 3
	nodesNum := len(cluster.Spec.Nodes)
	if nodesNum < 3 {
		return errors.Errorf("nodes count shoule at least 3, cannot start cluster %d", len(cluster.Spec.Nodes))
	} else if nodesNum > 3 {
		return errors.Errorf("nodes count more than 3, cannot start cluster temporary %d", len(cluster.Spec.Nodes))
	}

	return nil
}

func (r *CurveClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&curvev1.CurveCluster{}).
		Complete(r)
}
