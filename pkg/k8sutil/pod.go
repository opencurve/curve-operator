package k8sutil

import (
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetPodsByLabelSelector(clientset kubernetes.Interface, namespace string, selector string) (*v1.PodList, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return &v1.PodList{}, errors.Wrapf(err, "failed to list pods by LabelSelector %s", selector)
	}
	return pods, nil
}
