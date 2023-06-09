package k8sutil

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opencurve/curve-operator/pkg/clusterd"
	"github.com/opencurve/curve-operator/pkg/k8sutil/patch"
)

type DeploymentConfig struct {
	Name           string
	Namespace      string
	Labels         map[string]string
	InitContainers []v1.Container
	Containers     []v1.Container
	NodeName       string
	Volumes        []v1.Volume
	OwnerInfo      *OwnerInfo
}

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

	// Let's verify the deployment can be stopped
	// if err := verifyCallback("stop"); err != nil {
	// 	return fmt.Errorf("failed to check if deployment %q can be updated. %v", modifiedDeployment.Name, err)
	// }

	// Set hash annotation to the newly generated deployment
	if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(modifiedDeployment); err != nil {
		return fmt.Errorf("failed to set hash annotation on deployment %q. %v", modifiedDeployment.Name, err)
	}

	if _, err := clusterContext.Clientset.AppsV1().Deployments(namespace).Update(modifiedDeployment); err != nil {
		return fmt.Errorf("failed to update deployment %q. %v", modifiedDeployment.Name, err)
	}

	if err := WaitForDeploymentToStart(ctx, clusterContext, currentDeployment); err != nil {
		return err
	}

	// Now we check if we can go to the next daemon
	// if err := verifyCallback("continue"); err != nil {
	// 	return fmt.Errorf("failed to check if deployment %q can continue: %v", modifiedDeployment.Name, err)
	// }
	return nil
}

func WaitForDeploymentToStart(ctx context.Context, clusterdContext *clusterd.Context, deployment *appsv1.Deployment) error {
	// wait for the deployment to be restarted up to 300s（5min）
	sleepTime := 3
	attempts := 100
	for i := 0; i < attempts; i++ {
		// check for the status of the deployment
		d, err := clusterdContext.Clientset.AppsV1().Deployments(deployment.Namespace).Get(deployment.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment %q. %v", deployment.Name, err)
		}
		if d.Status.ObservedGeneration >= deployment.Status.ObservedGeneration && d.Status.UpdatedReplicas > 0 && d.Status.ReadyReplicas > 0 {
			logger.Infof("finished waiting for updated deployment %q", d.Name)
			return nil
		}

		// If ProgressDeadlineExceeded is reached let's fail earlier
		// This can happen if one of the deployment cannot be scheduled on a node and stays in "pending" state
		for _, condition := range d.Status.Conditions {
			if condition.Type == appsv1.DeploymentProgressing && condition.Reason == "ProgressDeadlineExceeded" {
				return fmt.Errorf("gave up waiting for deployment %q to update because %q", deployment.Name, condition.Reason)
			}
		}

		logger.Debugf("deployment %q status=%+v", d.Name, d.Status)

		time.Sleep(time.Duration(sleepTime) * time.Second)
	}
	return fmt.Errorf("gave up waiting for deployment %q to update", deployment.Name)
}

func MakeDeployment(c DeploymentConfig) (*appsv1.Deployment, error) {
	runAsUser := int64(0)
	runAsNonRoot := false

	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   c.Name,
			Labels: c.Labels,
		},
		Spec: v1.PodSpec{
			InitContainers: c.InitContainers,
			Containers:     c.Containers,
			NodeName:       c.NodeName,
			RestartPolicy:  v1.RestartPolicyAlways,
			HostNetwork:    true,
			DNSPolicy:      v1.DNSClusterFirstWithHostNet,
			Volumes:        c.Volumes,
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:    &runAsUser,
				RunAsNonRoot: &runAsNonRoot,
			},
		},
	}

	replicas := int32(1)

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
			Labels:    c.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: c.Labels,
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}

	// set ownerReference
	err := c.OwnerInfo.SetControllerReference(d)
	if err != nil {
		return nil, err
	}

	return d, nil
}
