package k8sutil

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// IsConfigMapExist check the ConfigMap if exist in specified namespace
func IsConfigMapExist(clientset kubernetes.Interface, c *corev1.ConfigMap) (bool, error) {
	_, err := clientset.CoreV1().ConfigMaps(c.Namespace).Get(c.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to check whether ConfigMap %s is exist", c.Name)
	}
	return true, nil
}

// GetConfigMap get configmap in specified namespace
func GetConfigMapByName(clientset kubernetes.Interface, namespace, name string) (*corev1.ConfigMap, error) {
	existConfigMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	return existConfigMap, err
}

// GetConfigMap get configmap in specified namespace
func GetConfigMap(clientset kubernetes.Interface, c *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	existConfigMap, err := clientset.CoreV1().ConfigMaps(c.Namespace).Get(c.Name, metav1.GetOptions{})
	return existConfigMap, err
}

// CreateNewConfigMap create a new ConfigMap in specified namespace
func CreateNewConfigMap(clientset kubernetes.Interface, c *corev1.ConfigMap) error {
	_, err := clientset.CoreV1().ConfigMaps(c.Namespace).Create(c)
	if err != nil {
		return errors.Wrapf(err, "failed to create ConfigMap %s in namespace %s", c.Name, c.Namespace)
	}
	return nil
}

// DeleteConfigMap delete a ConfigMap in specified namespace
func DeleteConfigMap(clientset kubernetes.Interface, c *corev1.ConfigMap) error {
	err := clientset.CoreV1().ConfigMaps(c.Namespace).Delete(c.Name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to delete ConfigMap %s in namespace %s", c.Name, c.Namespace)
	}

	return nil
}

// UpdateDeploymentAndWaitStart update a ConfigMap in specified namespace
func UpdateConfigMap(clientset kubernetes.Interface, c *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	updatedConfigMap, err := clientset.CoreV1().ConfigMaps(c.Namespace).Update(c)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update ConfigMap %s in namespace %s", c.Name, c.Namespace)
	}

	return updatedConfigMap, nil
}

// CreateOrUpdate create ConfigMap if not exist or update the ConfigMap.
func CreateOrUpdateConfigMap(clientset kubernetes.Interface, c *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	isExist, err := IsConfigMapExist(clientset, c)
	if err != nil {
		return nil, err
	}

	if !isExist {
		err = CreateNewConfigMap(clientset, c)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	updateConfigMap, err := UpdateConfigMap(clientset, c)
	if err != nil {
		return nil, err
	}
	return updateConfigMap, nil
}
