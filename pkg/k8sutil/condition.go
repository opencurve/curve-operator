package k8sutil

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
)

// UpdateCondition function will export each condition into the cluster custom resource
func UpdateCondition(ctx context.Context, client client.Client, kind string, namespacedName types.NamespacedName, phase curvev1.ClusterPhase, condition curvev1.ClusterCondition) {
	// use client.Client unit test this more easily with updating statuses which must use the client
	switch kind {
	case clusterd.KIND_CURVEBS:
		cluster := &curvev1.CurveCluster{}
		if err := client.Get(ctx, namespacedName, cluster); err != nil {
			logger.Errorf("failed to get cluster %v to update the conditions. %v", namespacedName, err)
			return
		}
		UpdateBsClusterCondition(client, cluster, phase, condition)
	case clusterd.KIND_CURVEFS:
		cluster := &curvev1.Curvefs{}
		if err := client.Get(ctx, namespacedName, cluster); err != nil {
			logger.Errorf("failed to get cluster %v to update the conditions. %v", namespacedName, err)
			return
		}
		UpdateFsClusterCondition(client, cluster, phase, condition)
	default:
		logger.Errorf("Unknown cluster kind %q", kind)
	}
}

// UpdateFsClusterCondition function will export each condition into the cluster custom resource
func UpdateFsClusterCondition(client client.Client, cluster *curvev1.Curvefs, phase curvev1.ClusterPhase, newCondition curvev1.ClusterCondition) {
	// Keep the conditions that already existed if they are in the list of long-term conditions,
	// otherwise discard the temporary conditions
	var currentCondition *curvev1.ClusterCondition
	var conditions []curvev1.ClusterCondition
	for _, condition := range cluster.Status.Conditions {
		// Only keep conditions in the list if it's a persisted condition such as the cluster creation being completed.
		// The transient conditions are not persisted. However, if the currently requested condition is not expected to
		// reset the transient conditions, they are retained. For example, if the operator is checking for ceph health
		// in the middle of the reconcile, the progress condition should not be reset by the status check update.
		if condition.Type == curvev1.ConditionClusterReady {
			if newCondition.Type != condition.Type {
				conditions = append(conditions, newCondition)
				continue
			}
			// Update the existing condition with the new status
			currentCondition = condition.DeepCopy()
			if currentCondition.Status != newCondition.Status || currentCondition.Message != newCondition.Message {
				// Update the last transition time since the status changed
				currentCondition.LastTransitionTime = metav1.NewTime(time.Now())
			}
			currentCondition.Status = newCondition.Status
			currentCondition.Reason = newCondition.Reason
			currentCondition.Message = newCondition.Message
		}
	}
	if currentCondition == nil {
		// Create a new condition since not found in the existing conditions
		currentCondition = &curvev1.ClusterCondition{
			Type:               newCondition.Type,
			Status:             newCondition.Status,
			Reason:             newCondition.Reason,
			Message:            newCondition.Message,
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
	}
	conditions = append(conditions, *currentCondition)
	cluster.Status.Conditions = conditions

	// Once the cluster begins deleting, the phase should not revert back to any other phase
	if string(cluster.Status.Phase) != string(curvev1.ConditionDeleting) {
		cluster.Status.Phase = phase
		cluster.Status.Message = currentCondition.Message
		logger.Debugf("CurveFsCluster %q status: %q. %q", cluster.GetName(), cluster.Status.Phase, cluster.Status.Message)
	}

	if err := client.Status().Update(context.TODO(), cluster); err != nil {
		logger.Errorf("failed to update cluster condition to %+v. %v", *currentCondition, err)

	}
}

// UpdateBsClusterCondition function will export each condition into the cluster custom resource
func UpdateBsClusterCondition(client client.Client, cluster *curvev1.CurveCluster, phase curvev1.ClusterPhase, newCondition curvev1.ClusterCondition) {
	// Keep the conditions that already existed if they are in the list of long-term conditions,
	// otherwise discard the temporary conditions
	var currentCondition *curvev1.ClusterCondition
	var conditions []curvev1.ClusterCondition
	for _, condition := range cluster.Status.Conditions {
		// Only keep conditions in the list if it's a persisted condition such as the cluster creation being completed.
		// The transient conditions are not persisted. However, if the currently requested condition is not expected to
		// reset the transient conditions, they are retained. For example, if the operator is checking for ceph health
		// in the middle of the reconcile, the progress condition should not be reset by the status check update.
		if condition.Type == curvev1.ConditionClusterReady {
			if newCondition.Type != condition.Type {
				conditions = append(conditions, newCondition)
				continue
			}
			// Update the existing condition with the new status
			currentCondition = condition.DeepCopy()
			if currentCondition.Status != newCondition.Status || currentCondition.Message != newCondition.Message {
				// Update the last transition time since the status changed
				currentCondition.LastTransitionTime = metav1.NewTime(time.Now())
			}
			currentCondition.Status = newCondition.Status
			currentCondition.Reason = newCondition.Reason
			currentCondition.Message = newCondition.Message
		}
	}
	if currentCondition == nil {
		// Create a new condition since not found in the existing conditions
		currentCondition = &curvev1.ClusterCondition{
			Type:               newCondition.Type,
			Status:             newCondition.Status,
			Reason:             newCondition.Reason,
			Message:            newCondition.Message,
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
	}
	conditions = append(conditions, *currentCondition)
	cluster.Status.Conditions = conditions

	// Once the cluster begins deleting, the phase should not revert back to any other phase
	if string(cluster.Status.Phase) != string(curvev1.ConditionDeleting) {
		cluster.Status.Phase = phase
		cluster.Status.Message = currentCondition.Message
		logger.Debugf("CurveBsCluster %q status: %q. %q", cluster.GetName(), cluster.Status.Phase, cluster.Status.Message)
	}

	if err := client.Status().Update(context.TODO(), cluster); err != nil {
		logger.Errorf("failed to update cluster condition to %+v. %v", *currentCondition, err)

	}
}
