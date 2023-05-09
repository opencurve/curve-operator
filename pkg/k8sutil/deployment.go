package k8sutil

import (
	"context"
	"fmt"
	"time"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil/patch"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	WaitForRunningInterval = 2 * time.Second
	WaitForRunningTimeout  = 5 * time.Minute
)

// UpdateDeploymentAndWait updates a deployment and waits until it is running to return. It will
// error if the deployment does not exist to be updated or if it takes too long.
// This method has a generic callback function that each backend can rely on
// It serves two purposes:
//  1. verify that a resource can be stopped
//  2. verify that we can continue the update procedure
//
// Basically, we go one resource by one and check if we can stop and then if the resource has been successfully updated
// we check if we can go ahead and move to the next one.
func UpdateDeploymentAndWait(ctx context.Context, clusterContext *clusterd.Context, modifiedDeployment *appsv1.Deployment, namespace string, verifyCallback func(action string) error) error {
	currentDeployment, err := clusterContext.Clientset.AppsV1().Deployments(namespace).Get(modifiedDeployment.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s. %+v", modifiedDeployment.Name, err)
	}

	// Check whether the current deployment and newly generated one are identical
	patchChanged := false
	patchResult, err := patch.DefaultPatchMaker.Calculate(currentDeployment, modifiedDeployment)
	if err != nil {
		logger.Warningf("failed to calculate diff between current deployment %q and newly generated one. Assuming it changed. %v", currentDeployment.Name, err)
		patchChanged = true
	} else if !patchResult.IsEmpty() {
		patchChanged = true
	}

	if !patchChanged {
		logger.Infof("deployment %q did not change, nothing to update", currentDeployment.Name)
		return nil
	}

	// If deployments are different, let's update!
	logger.Infof("updating deployment %q after verifying it is safe to stop", modifiedDeployment.Name)

	// Set hash annotation to the newly generated deployment
	if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(modifiedDeployment); err != nil {
		return fmt.Errorf("failed to set hash annotation on deployment %q. %v", modifiedDeployment.Name, err)
	}

	if _, err := clusterContext.Clientset.AppsV1().Deployments(namespace).Update(modifiedDeployment); err != nil {
		return fmt.Errorf("failed to update deployment %q. %v", modifiedDeployment.Name, err)
	}

	waitChan := make(chan error)
	defer close(waitChan)

	go func() {
		waitChan <- WaitForDeploymentToStart(clusterContext, currentDeployment, WaitForRunningInterval, WaitForRunningTimeout)
	}()

	select {
	case <-ctx.Done():
		return errors.Wrapf(ctx.Err(), "failed to wait for deployment %q to start due to context is done", currentDeployment.Name)
	case err := <-waitChan:
		if err != nil {
			return fmt.Errorf("failed to wait for deployment %q to start due to timeout. %v", currentDeployment.Name, err)
		}
	}
	return nil
}

// WaitForDeploymentsToStart waits for the deployments to start, and returns an error if any of the deployments
// not started
//
// interval is the interval to check the deployment status
// timeout is the timeout to wait for the deployment to start, if timeout, it returns an error
func WaitForDeploymentsToStart(clusterContext *clusterd.Context, objectMetas []*appsv1.Deployment,
	interval time.Duration, timeout time.Duration) error {
	length := len(objectMetas)
	hub := make(chan error, length)
	defer close(hub)
	for i := range objectMetas {
		objectMata := objectMetas[i]
		go func() {
			hub <- WaitForDeploymentToStart(clusterContext, objectMata, interval, timeout)
		}()
	}

	var errorSlice []error
	for i := 0; i < length; i++ {
		if err := <-hub; err != nil {
			errorSlice = append(errorSlice, err)
		}
	}
	return utilerrors.NewAggregate(errorSlice)
}

// WaitForDeploymentToStart waits for the deployment to start, and returns an error if the deployment not started
//
// interval is the interval to check the deployment status
// timeout is the timeout to wait for the deployment to start, if timeout, it returns an error
func WaitForDeploymentToStart(clusterContext *clusterd.Context, d *appsv1.Deployment, interval time.Duration,
	timeout time.Duration) error {

	// err is the error of once poll, it may provide the reason why the deployment is not started, and tell
	// wait.PollImmediate to continue to poll
	// and the lastErr is the error of the last poll
	var lastErr error
	_ = wait.PollImmediate(interval, timeout, func() (bool, error) {
		deployment, err := clusterContext.Clientset.AppsV1().Deployments(d.GetNamespace()).
			Get(d.GetName(), metav1.GetOptions{})
		if err != nil {
			logger.Errorf("failed to get deployment %s in cluster: %s", d.GetName(), err.Error())
			return false, err
		}
		newStatus := deployment.Status
		// this code is copied from pkg/controller/deployment/deployment_controller.go
		if newStatus.UpdatedReplicas == *(deployment.Spec.Replicas) &&
			newStatus.Replicas == *(deployment.Spec.Replicas) &&
			newStatus.AvailableReplicas == *(deployment.Spec.Replicas) &&
			newStatus.ObservedGeneration >= deployment.Generation {
			logger.Infof("deployment %s has been started", deployment.Name)
			// Set lastErr to nil because we have a successful poll
			lastErr = nil
			return true, nil
		}

		var unready []appsv1.DeploymentCondition
		// filter out the conditions that are not ready to help debug
		for i := range deployment.Status.Conditions {
			condition := deployment.Status.Conditions[i]
			if condition.Status != v1.ConditionTrue {
				unready = append(unready, condition)
			}
		}
		logger.Infof("deployment %s is starting, Generation: %d, ObservedGeneration: %d, UpdatedReplicas: %d,"+
			" ReadyReplicas: %d, UnReadyConditions: %v", deployment.Name, deployment.GetGeneration(),
			deployment.Status.ObservedGeneration, deployment.Status.UpdatedReplicas,
			deployment.Status.ReadyReplicas, unready)
		if err != nil {
			lastErr = err
		}
		return false, nil
	})
	if lastErr != nil {
		return errors.Wrapf(lastErr, "failed to waiting deplyoment %s to start after %vs waiting",
			d.GetName(), timeout.Seconds())
	}
	return errors.Errorf("failed to waiting deplyoment %s to start after %vs waiting",
		d.GetName(), timeout.Seconds())
}
