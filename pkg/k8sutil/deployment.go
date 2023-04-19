package k8sutil

import (
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// WaitForDeploymentsToStart waits for the deployments to start, and returns an error if any of the deployments
// not started
//
// interval is the interval to check the deployment status
// timeout is the timeout to wait for the deployment to start, if timeout, it returns an error
func WaitForDeploymentsToStart(clientSet kubernetes.Interface, interval time.Duration,
	timeout time.Duration, objectMetas []*appsv1.Deployment) error {
	length := len(objectMetas)
	hub := make(chan error, length)
	defer close(hub)
	for i := range objectMetas {
		objectMata := objectMetas[i]
		go func() {
			hub <- WaitForDeploymentToStart(clientSet, interval, timeout, objectMata)
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
func WaitForDeploymentToStart(clientSet kubernetes.Interface, interval time.Duration,
	timeout time.Duration, d *appsv1.Deployment) error {

	// err is the error of once poll, it may provide the reason why the deployment is not started, and tell
	// wait.PollImmediate to continue to poll
	// and the lastErr is the error of the last poll
	var lastErr error
	_ = wait.PollImmediate(interval, timeout, func() (bool, error) {
		deployment, err := clientSet.AppsV1().Deployments(d.GetNamespace()).
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
