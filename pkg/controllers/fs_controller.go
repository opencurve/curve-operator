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
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil"
	"github.com/opencurve/curve-operator/pkg/service"
	"github.com/opencurve/curve-operator/pkg/topology"
)

// CurvefsReconciler reconciles a Curvefs object
type CurvefsReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	context    clusterd.Context
	clusterMap map[string]*clusterd.FsClusterManager
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

		context:    context,
		clusterMap: make(map[string]*clusterd.FsClusterManager),
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
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

func (r *CurvefsReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("curve FS cluster", req.NamespacedName)
	logger.Info("reconcileing CurvefsCluster")

	r.context.Client = r.Client
	ctx := context.Background()

	curvefsCluster := &curvev1.Curvefs{}
	if err := r.Client.Get(ctx, req.NamespacedName, curvefsCluster); err != nil {
		logger.Error(err, "curvefs resource not found. Ignoring since object must be deleted.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Set a finalizer so we can do cleanup before the object goes away
	if err := k8sutil.AddFinalizerIfNotPresent(ctx, r.Client, curvefsCluster); err != nil {
		return reconcile.Result{}, err
	}

	// k8sutil.UpdateCondition(context.TODO(),
	// 	r.Client,
	// 	clusterd.KIND_CURVEFS,
	// 	req.NamespacedName,
	// 	curvev1.ClusterPending,
	// 	curvev1.ClusterCondition{
	// 		Type:    curvev1.ConditionProgressing,
	// 		Status:  curvev1.ConditionStatusTrue,
	// 		Reason:  curvev1.ConditionReconcileStarted,
	// 		Message: "start to reconcile curve fs cluster",
	// 	},
	// )

	// The CR was deleted
	if !curvefsCluster.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, r.reconcileCurvefsDelete(curvefsCluster)
	}

	ownerInfo := clusterd.NewOwnerInfo(curvefsCluster, r.Scheme)
	return r.reconcileCurvefsCluster(curvefsCluster, ownerInfo)
}

// reconcileCurvefsDelete
func (r *CurvefsReconciler) reconcileCurvefsDelete(clusterObj *curvev1.Curvefs) error {
	// get currnet cluster and delete it
	cluster, ok := r.clusterMap[clusterObj.GetNamespace()]
	if !ok {
		logger.Errorf("failed to find the cluster %q", clusterObj.GetName())
		return errors.New("internal error")
	}

	dcs, err := topology.ParseTopology(cluster)
	if err != nil {
		return err
	}

	err = service.StartClusterCleanUpJob(cluster, dcs)
	if err != nil {
		return err
	}

	// delete it from clusterMap
	if _, ok := r.clusterMap[cluster.GetNameSpace()]; ok {
		delete(r.clusterMap, cluster.GetNameSpace())
	}

	// remove finalizers
	k8sutil.RemoveFinalizer(context.Background(),
		r.Client,
		types.NamespacedName{Namespace: clusterObj.GetNamespace(), Name: clusterObj.GetName()},
		clusterObj)

	logger.Infof("curve cluster %v has been deleted successed", clusterObj.GetName())

	return nil
}

// reconcileCurvefsCluster start reconcile a CurveFS cluster
func (r *CurvefsReconciler) reconcileCurvefsCluster(clusterObj *curvev1.Curvefs, ownerInfo *clusterd.OwnerInfo) (reconcile.Result, error) {
	m, ok := r.clusterMap[clusterObj.Namespace]
	if !ok {
		newUUID := uuid.New().String()
		m = newFsClusterManager(newUUID, clusterd.KIND_CURVEFS)
	}

	// construct cluster object
	m.Context = r.context
	m.Cluster = clusterObj
	m.Logger = r.Log
	m.OwnerInfo = ownerInfo

	r.clusterMap[m.GetNameSpace()] = m
	m.Logger.Info("reconcileing Curve FS Cluster in namespace %q", m.GetNameSpace())

	dcs, err := topology.ParseTopology(m)
	if err != nil {
		return reconcile.Result{}, err
	}

	switch m.Cluster.Status.Phase {
	case "":
		// Update the cluster status to 'Creating'
		m.Logger.Info("Curvefs accepted by operator", "curvefs", client.ObjectKey{
			Name:      m.GetName(),
			Namespace: m.GetNameSpace(),
		})

		// create a configmap to record previous config of yaml file
		if err := createorUpdateRecordConfigMap(m); err != nil {
			m.Logger.Error(err, "failed to create or update previous ConfigMap")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		m.Cluster.Status.Phase = curvev1.ClusterCreating
		m.Cluster.Status.CurveVersion = m.Cluster.Spec.CurveVersion
		m.Cluster.Status.StorageDir.DataDir = m.Cluster.Spec.DataDir
		m.Cluster.Status.StorageDir.LogDir = m.Cluster.Spec.LogDir
		if err := r.Status().Update(context.TODO(), m.Cluster); err != nil {
			m.Logger.Error(err, "unable to update Curvefs")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	case curvev1.ClusterCreating:
		// Create a new cluster and update cluster status to 'Running'
		initCluster(m, dcs)
		m.Logger.Info("Curvefs accepted by operator", "curvefs", client.ObjectKey{
			Name:      m.GetName(),
			Namespace: m.GetNameSpace(),
		})

		m.Cluster.Status.Phase = curvev1.ClusterRunning
		if err := r.Status().Update(context.TODO(), m.Cluster); err != nil {
			m.Logger.Error(err, "unable to update Curvefs")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	case curvev1.ClusterRunning:
		// Watch the update event and update cluster stauts to specfied 'status'
		// Upgrading、Updating、Scaling

		// 1. check for upgrade
		if m.Cluster.Spec.CurveVersion.Image != m.Cluster.Status.CurveVersion.Image {
			m.Logger.Info("Check curvefs cluster image not match, need upgrade")
			m.Cluster.Status.Phase = curvev1.ClusterUpgrading
			m.Cluster.Status.CurveVersion = m.Cluster.Spec.CurveVersion
		}

		// TODO: 2. compare DataDir and LogDir - not implement
		// if m.Cluster.Spec.DataDir != m.Cluster.Status.StorageDir.DataDir ||
		// 	m.Cluster.Spec.LogDir != m.Cluster.Status.StorageDir.LogDir {
		// 	m.Cluster.Status.Phase = curvev1.ClusterUpdating
		// 	m.Cluster.Status.StorageDir.DataDir = m.Cluster.Spec.DataDir
		// 	m.Cluster.Status.StorageDir.LogDir = m.Cluster.Spec.LogDir
		// }

		// 3. compare etcd and mds and metaserver config
		specParameters, _ := parseSpecParameters(m)
		statusParameters, err := getDataFromRecordConfigMap(m)
		if err != nil {
			m.Logger.Error(err, "failed to read record config from record-configmap")
			return ctrl.Result{}, nil
		}
		statusModified := false
		for role, specRolePara := range specParameters {
			roleParaVar := map[string]string{}
			for specPK, specPV := range specRolePara {
				paraStatusVal, paraExists := statusParameters[role][specPK]
				if !paraExists || paraStatusVal != specPV {
					roleParaVar[specPK] = specPV
					statusModified = true
				}
				delete(statusParameters[role], specPK)
			}
			// delete some parameters
			if len(statusParameters[role]) > 0 {
				statusModified = true
			}
			m.Cluster.Status.LastModContextSet.ModContextSet = append(m.Cluster.Status.LastModContextSet.ModContextSet, curvev1.ModContext{
				Role:       role,
				Parameters: roleParaVar,
			})
		}
		if statusModified {
			m.Cluster.Status.Phase = curvev1.ClusterUpdating
		}

		if err := r.Status().Update(context.TODO(), m.Cluster); err != nil {
			m.Logger.Error(err, "unable to update Curvefs")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	case curvev1.ClusterUpdating:
		// Update cluster and the target status is Running to watch other update events.
		m.Logger.Info("Curvefs running to update", "curvefs", client.ObjectKey{
			Name:      m.GetName(),
			Namespace: m.GetNameSpace(),
		})
		mcs := m.Cluster.Status.LastModContextSet.ModContextSet
		if len(mcs) <= 0 {
			m.Logger.Info("No Config need to update, ignore the event")
			return ctrl.Result{}, nil
		}

		roles2Modfing := map[string]bool{}
		for _, ctx := range mcs {
			roles2Modfing[ctx.Role] = true
		}
		// render fs-record-config ConfigMap again
		if err := createorUpdateRecordConfigMap(m); err != nil {
			m.Logger.Error(err, "failed to create or update previous ConfigMap")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		// 1. render After-Mutate-Config ConfigMap again
		for role := range roles2Modfing {
			for _, dc := range topology.FilterDeployConfigByRole(dcs, role) {
				serviceConfigs := dc.GetProjectLayout().ServiceConfFiles
				for _, conf := range serviceConfigs {
					err := mutateConfig(m, dc, conf.Name)
					if err != nil {
						m.Logger.Error(err, "failed to render configmap again")
						return ctrl.Result{}, err
					}
				}
			}

		}
		// 2. rebuild the Pods under the Deployment corresponding to the role, upgrade one by one.
		//  And wait for all Pods under the Deployment (only one) to be in the Ready state.
		for role := range roles2Modfing {
			for _, dc := range topology.FilterDeployConfigByRole(dcs, role) {
				if err := service.StartService(m, dc); err != nil {
					m.Logger.Error(err, "failed to update Deployment Service")
					return ctrl.Result{}, err
				}
			}
		}

		m.Cluster.Status.Phase = curvev1.ClusterRunning
		m.Cluster.Status.LastModContextSet.ModContextSet = nil
		if err := r.Status().Update(context.TODO(), m.Cluster); err != nil {
			m.Logger.Error(err, "failed to update Curvefs")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	case curvev1.ClusterUpgrading:
		// Upgrade cluster and the target status is Running to watch other update events.
		m.Logger.Info("Curvefs running to update", "curvefs", client.ObjectKey{
			Name:      m.GetName(),
			Namespace: m.GetNameSpace(),
		})

		for _, dc := range dcs {
			if err := service.StartService(m, dc); err != nil {
				m.Logger.Error(err, "failed to upgrade service ", dc.GetName())
				return ctrl.Result{}, err
			}
		}

		m.Cluster.Status.Phase = curvev1.ClusterRunning
		if err := r.Status().Update(context.TODO(), m.Cluster); err != nil {
			m.Logger.Error(err, "failed to update Curvefs")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	case curvev1.ClusterScaling:
		// Perform the scale operation.
		// The target status is Running, and continue to listen to other events.
		m.Cluster.Status.Phase = curvev1.ClusterRunning
		if err := r.Status().Update(context.TODO(), m.Cluster); err != nil {
			m.Logger.Error(err, "failed to update Curvefs")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *CurvefsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&curvev1.Curvefs{}).
		Complete(r)
}
