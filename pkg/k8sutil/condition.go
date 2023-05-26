package k8sutil

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	curvev1 "github.com/opencurve/curve-operator/api/v1"
	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/config"
)

func UpdateStatusCondition(kind string, ctx context.Context, c *clusterd.Context, namespaceName types.NamespacedName, conditionType curvev1.ConditionType, status curvev1.ConditionStatus, reason curvev1.ConditionReason, message string) error {
	if kind == config.KIND_CURVEBS {
		err := UpdateCondition(ctx, c, namespaceName, conditionType, status, reason, message)
		if err != nil {
			return err
		}
	} else {
		err := UpdateFSCondition(ctx, c, namespaceName, conditionType, status, reason, message)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateCondition function will export each condition into the BS cluster custom resource
func UpdateCondition(ctx context.Context, c *clusterd.Context, namespaceName types.NamespacedName, conditionType curvev1.ConditionType, status curvev1.ConditionStatus, reason curvev1.ConditionReason, message string) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		cluster := &curvev1.CurveCluster{}
		if err := c.Client.Get(ctx, namespaceName, cluster); err != nil {
			logger.Errorf("failed to get cluster %v to update the conditions. %v", namespaceName, err)
			return err
		}

		return UpdateClusterCondition(c, cluster, namespaceName, conditionType, status, reason, message, false)
	})
}

// UpdateFSCondition function will export each condition into the FS cluster custom resource
func UpdateFSCondition(ctx context.Context, c *clusterd.Context, namespaceName types.NamespacedName, conditionType curvev1.ConditionType, status curvev1.ConditionStatus, reason curvev1.ConditionReason, message string) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		cluster := &curvev1.Curvefs{}
		if err := c.Client.Get(ctx, namespaceName, cluster); err != nil {
			logger.Errorf("failed to get cluster %v to update the conditions. %v", namespaceName, err)
			return err
		}

		return UpdateFSClusterCondition(c, cluster, namespaceName, conditionType, status, reason, message, false)
	})
}

// UpdateClusterCondition function will export each condition into the cluster custom resource
func UpdateFSClusterCondition(c *clusterd.Context, cluster *curvev1.Curvefs, namespaceName types.NamespacedName, conditionType curvev1.ConditionType, status curvev1.ConditionStatus,
	reason curvev1.ConditionReason, message string, preserveAllConditions bool) error {

	// Keep the conditions that already existed if they are in the list of long-term conditions,
	// otherwise discard the temporary conditions
	var currentCondition *curvev1.ClusterCondition
	var conditions []curvev1.ClusterCondition
	for _, condition := range cluster.Status.Conditions {
		// Only keep conditions in the list if it's a persisted condition such as the cluster creation being completed.
		// The transient conditions are not persisted. However, if the currently requested condition is not expected to
		// reset the transient conditions, they are retained. For example, if the operator is checking for curve health
		// in the middle of the reconcile, the progress condition should not be reset by the status check update.
		if preserveAllConditions ||
			condition.Type == curvev1.ConditionTypeEtcdReady ||
			condition.Type == curvev1.ConditionTypeMdsReady ||
			condition.Type == curvev1.ConditionTypeMetaServerReady ||
			condition.Type == curvev1.ConditionTypeSnapShotCloneReady {
			if conditionType != condition.Type {
				conditions = append(conditions, condition)
				continue
			}

			// Update the existing condition with the new status
			currentCondition = condition.DeepCopy()
			if currentCondition.Status != status || currentCondition.Message != message {
				// Update the last transition time since the status changed
				currentCondition.LastTransitionTime = metav1.NewTime(time.Now())
			}
			currentCondition.Status = status
			currentCondition.Reason = reason
			currentCondition.Message = message
		}
	}

	// Create a new condition since not found in the existing conditions
	if currentCondition == nil {
		currentCondition = &curvev1.ClusterCondition{
			Type:               conditionType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
	}

	conditions = append(conditions, *currentCondition)
	cluster.Status.Conditions = conditions

	// Once the cluster begins deleting, the phase should not revert back to any other phase
	if cluster.Status.Phase != curvev1.ClusterPhaseDeleting {
		cluster.Status.Phase = translateConditionType2Phase(conditionType)
		cluster.Status.Message = currentCondition.Message
		cluster.Status.CurveVersion.Image = cluster.Spec.CurveVersion.Image
		logger.Debugf("CurveCluster %q status: %q. %q", namespaceName.Namespace, cluster.Status.Phase, cluster.Status.Message)
	}

	if err := UpdateStatus(c.Client, namespaceName, cluster); err != nil {
		logger.Errorf("failed to update cluster condition to %+v. %v", *currentCondition, err)
		return err
	}

	return nil
}

// UpdateClusterCondition function will export each condition into the cluster custom resource
func UpdateClusterCondition(c *clusterd.Context, cluster *curvev1.CurveCluster, namespaceName types.NamespacedName, conditionType curvev1.ConditionType, status curvev1.ConditionStatus,
	reason curvev1.ConditionReason, message string, preserveAllConditions bool) error {

	// Keep the conditions that already existed if they are in the list of long-term conditions,
	// otherwise discard the temporary conditions
	var currentCondition *curvev1.ClusterCondition
	var conditions []curvev1.ClusterCondition
	for _, condition := range cluster.Status.Conditions {
		// Only keep conditions in the list if it's a persisted condition such as the cluster creation being completed.
		// The transient conditions are not persisted. However, if the currently requested condition is not expected to
		// reset the transient conditions, they are retained. For example, if the operator is checking for curve health
		// in the middle of the reconcile, the progress condition should not be reset by the status check update.
		if preserveAllConditions ||
			condition.Type == curvev1.ConditionTypeEtcdReady ||
			condition.Type == curvev1.ConditionTypeMdsReady ||
			condition.Type == curvev1.ConditionTypeFormatedReady ||
			condition.Type == curvev1.ConditionTypeChunkServerReady ||
			condition.Type == curvev1.ConditionTypeSnapShotCloneReady {
			if conditionType != condition.Type {
				conditions = append(conditions, condition)
				continue
			}

			// Update the existing condition with the new status
			currentCondition = condition.DeepCopy()
			if currentCondition.Status != status || currentCondition.Message != message {
				// Update the last transition time since the status changed
				currentCondition.LastTransitionTime = metav1.NewTime(time.Now())
			}
			currentCondition.Status = status
			currentCondition.Reason = reason
			currentCondition.Message = message
		}
	}

	// Create a new condition since not found in the existing conditions
	if currentCondition == nil {
		currentCondition = &curvev1.ClusterCondition{
			Type:               conditionType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
	}

	conditions = append(conditions, *currentCondition)
	cluster.Status.Conditions = conditions

	// Once the cluster begins deleting, the phase should not revert back to any other phase
	if cluster.Status.Phase != curvev1.ClusterPhaseDeleting {
		cluster.Status.Phase = translateConditionType2Phase(conditionType)
		cluster.Status.Message = currentCondition.Message
		cluster.Status.CurveVersion.Image = cluster.Spec.CurveVersion.Image
		logger.Debugf("CurveCluster %q status: %q. %q", namespaceName.Namespace, cluster.Status.Phase, cluster.Status.Message)
	}

	if err := UpdateStatus(c.Client, namespaceName, cluster); err != nil {
		logger.Errorf("failed to update cluster condition to %+v. %v", *currentCondition, err)
		return err
	}
	return nil
}

func translateConditionType2Phase(conditionType curvev1.ConditionType) curvev1.ConditionType {
	if conditionType == curvev1.ConditionTypeEtcdReady ||
		conditionType == curvev1.ConditionTypeMdsReady ||
		conditionType == curvev1.ConditionTypeFormatedReady ||
		conditionType == curvev1.ConditionTypeChunkServerReady ||
		conditionType == curvev1.ConditionTypeSnapShotCloneReady {
		return curvev1.ClusterPhasePending
	}
	return conditionType
}

// UpdateStatus updates an object with a given status. The object is updated with the latest version
// from the server on a successful update.
func UpdateStatus(client client.Client, namespaceName types.NamespacedName, obj client.Object) error {
	nsName := types.NamespacedName{
		Namespace: namespaceName.Namespace,
		Name:      namespaceName.Name,
	}

	// Try to update the status
	err := client.Status().Update(context.Background(), obj)
	// If the object doesn't exist yet, we need to initialize it
	if kerrors.IsNotFound(err) {
		err = client.Update(context.Background(), obj)
	}

	if err != nil {
		return errors.Wrapf(err, "failed to update object %q status", nsName.String())
	}

	return nil
}
