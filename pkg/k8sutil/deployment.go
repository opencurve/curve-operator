package k8sutil

import (
	"fmt"
	"time"

	"emperror.dev/errors"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// IsDeploymentExist check the Deployment if exist in specified namespace
func IsDeploymentExist(clientset kubernetes.Interface, d *appsv1.Deployment) (bool, error) {
	_, err := clientset.AppsV1().Deployments(d.Namespace).Get(d.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to check whether Deployment %s is exist", d.Name)
	}
	return true, nil
}

// GetDeployment get a Deployment in specified namespace
func GetDeployment(clientset kubernetes.Interface, d *appsv1.Deployment) error {
	err := clientset.AppsV1().Deployments(d.Namespace).Delete(d.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to delete Deployment %s in namespace %s", d.Name, d.Namespace)
	}

	return nil
}

// CreateNewDeploymentAndWaitStart create a new Deployment in specified namespace and wait it to start up
func CreateNewDeploymentAndWaitStart(clientset kubernetes.Interface, d *appsv1.Deployment) error {
	newDeploy, err := clientset.AppsV1().Deployments(d.Namespace).Create(d)
	if err != nil {
		return err
	}

	if err := WaitForDeploymentToStart(clientset, newDeploy); err != nil {
		return err
	}

	return nil
}

// UpdateDeploymentAndWaitStart update a Deployment and wait it to start
func UpdateDeploymentAndWaitStart(clientset kubernetes.Interface, d *appsv1.Deployment) (*appsv1.Deployment, error) {
	updatedDeploy, err := clientset.AppsV1().Deployments(d.Namespace).Update(d)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update Deployment %s in namespace %s", d.Name, d.Namespace)
	}

	if err := WaitForDeploymentToStart(clientset, updatedDeploy); err != nil {
		return nil, err
	}

	return updatedDeploy, nil
}

// CreateOrUpdateDeploymentAndWaitStart create Deployment if not exist or update deployment and wait it to start.
func CreateOrUpdateDeploymentAndWaitStart(clientset kubernetes.Interface, d *appsv1.Deployment) (*appsv1.Deployment, error) {
	isExist, err := IsDeploymentExist(clientset, d)
	if err != nil {
		return nil, err
	}

	if !isExist {
		err = CreateNewDeploymentAndWaitStart(clientset, d)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	newDeploy, err := UpdateDeploymentAndWaitStart(clientset, d)
	if err != nil {
		return nil, err
	}
	return newDeploy, nil
}

func WaitForDeploymentToStart(clientset kubernetes.Interface, d *appsv1.Deployment) error {
	sleepTime := 3
	attempts := 100
	for i := 0; i < attempts; i++ {
		deploy, err := clientset.AppsV1().Deployments(d.Namespace).Get(d.Name, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to get Deployment %s in namespace %s", d.Name, d.Namespace)
		}
		if deploy.Status.ObservedGeneration >= d.Status.ObservedGeneration &&
			deploy.Status.UpdatedReplicas > 0 &&
			deploy.Status.ReadyReplicas > 0 {
			return nil
		}

		// If ProgressDeadlineExceeded is reached let's fail earlier
		// This can happen if one of the deployment cannot be scheduled on a node and stays in "pending" state
		for _, condition := range d.Status.Conditions {
			if condition.Type == appsv1.DeploymentProgressing && condition.Reason == "ProgressDeadlineExceeded" {
				return fmt.Errorf("gave up waiting for deployment %s to update because %s", d.Name, condition.Reason)
			}
		}

		time.Sleep(time.Duration(sleepTime) * time.Second)
	}

	return fmt.Errorf("give up waiting for deployment %q to update", d.Name)
}

// DeleteDeployment delete a Deployment in specified namespace
func DeleteDeployment(clientset kubernetes.Interface, d *appsv1.Deployment) error {
	err := clientset.AppsV1().Deployments(d.Namespace).Delete(d.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to delete Deployment %s in namespace %s", d.Name, d.Namespace)
	}

	return nil
}
