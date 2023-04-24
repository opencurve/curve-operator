package k8sutil

import "strings"

// GetLabelSelector get labelSelector by labels of pod
func GetLabelSelector(labels map[string]string) string {
	var labelSelector []string
	for k, v := range labels {
		labelSelector = append(labelSelector, k+"="+v)
	}
	selector := strings.Join(labelSelector, ",")
	return selector
}
