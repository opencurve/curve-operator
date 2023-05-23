package k8sutil

import (
	"context"
	"fmt"
	"time"

	batch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// RunReplaceableJob runs a Kubernetes job with the intention that the job can be replaced by
// another call to this function with the same job name. For example, if a storage operator is
// restarted/updated before the job can complete, the operator's next run of the job should replace
// the previous job if deleteIfFound is set to true.
func RunReplaceableJob(ctx context.Context, clientset kubernetes.Interface, job *batch.Job, deleteIfFound bool) error {
	// check if the job was already created and what its status is
	existingJob, err := clientset.BatchV1().Jobs(job.Namespace).Get(context.Background(), job.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		// error but found, will recreate it
		logger.Warningf("failed to detect job %s. %+v", job.Name, err)
	} else if err == nil {
		// if the job is still running, and the caller has not asked for deletion,
		// allow it to continue to completion
		if existingJob.Status.Active > 0 && !deleteIfFound {
			logger.Infof("Found previous job %s. Status=%+v", job.Name, existingJob.Status)
			return nil
		}

		// delete the job that already exists from a previous run
		logger.Infof("Removing previous job %s to start a new one", job.Name)
		err := DeleteBatchJob(ctx, clientset, job.Namespace, existingJob.Name, true)
		if err != nil {
			return fmt.Errorf("failed to remove job %s. %+v", job.Name, err)
		}
	}

	// always create new job
	_, err = clientset.BatchV1().Jobs(job.Namespace).Create(context.Background(), job, metav1.CreateOptions{})
	return err
}

// DeleteBatchJob deletes a Kubernetes job.
func DeleteBatchJob(ctx context.Context, clientset kubernetes.Interface, namespace, name string, wait bool) error {
	propagation := metav1.DeletePropagationForeground
	gracePeriod := int64(0)
	options := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod, PropagationPolicy: &propagation}
	if err := clientset.BatchV1().Jobs(namespace).Delete(context.Background(), name, options); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to remove previous provisioning job for node %s. %+v", name, err)
	}

	if !wait {
		return nil
	}

	// Retry for the job to be deleted for 90s. A pod can easily take 60s to timeout before
	// deletion so we add some buffer to that time.
	retries := 30
	sleepInterval := 3 * time.Second
	for i := 0; i < retries; i++ {
		_, err := clientset.BatchV1().Jobs(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			logger.Infof("batch job %s deleted", name)
			return nil
		}

		logger.Infof("batch job %s still exists", name)
		time.Sleep(sleepInterval)
	}

	logger.Warningf("gave up waiting for batch job %s to be deleted", name)
	return nil
}

// WaitForJobCompletion waits for a job to reach the completed state.
// Assumes that only one pod needs to complete.
func WaitForJobCompletion(ctx context.Context, clientset kubernetes.Interface, job *batch.Job, timeout time.Duration) error {
	logger.Infof("waiting for job %s to complete...", job.Name)
	return wait.Poll(5*time.Second, timeout, func() (bool, error) {
		job, err := clientset.BatchV1().Jobs(job.Namespace).Get(context.Background(), job.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to detect job %s. %+v", job.Name, err)
		}

		// if the job is still running, allow it to continue to completion
		if job.Status.Active > 0 {
			logger.Debugf("job is still running. Status=%+v", job.Status)
			return false, nil
		}
		if job.Status.Failed > 0 {
			return false, fmt.Errorf("job %s failed", job.Name)
		}
		if job.Status.Succeeded > 0 {
			return true, nil
		}
		logger.Debugf("job is still initializing")
		return false, nil
	})
}
