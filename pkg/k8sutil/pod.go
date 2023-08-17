package k8sutil

import (
	"fmt"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
)

func GetPodsByLabelSelector(clientset kubernetes.Interface, namespace string, selector string) (*v1.PodList, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return &v1.PodList{}, errors.Wrap(err, "failed to list pods by LabelSelector")
	}
	return pods, nil
}

// GetRunningPod reads the name and namespace of a pod from the
// environment, and returns the pod (if it exists).
func GetRunningPod(clientset kubernetes.Interface) (*v1.Pod, error) {
	podName := os.Getenv(PodNameEnvVar)
	if podName == "" {
		return nil, fmt.Errorf("cannot detect the pod name. Please provide it using the downward API in the manifest file")
	}
	podNamespace := os.Getenv(PodNamespaceEnvVar)
	if podName == "" {
		return nil, fmt.Errorf("cannot detect the pod namespace. Please provide it using the downward API in the manifest file")
	}

	pod, err := clientset.CoreV1().Pods(podNamespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}
