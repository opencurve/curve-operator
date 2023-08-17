package k8sutil

import v1 "k8s.io/api/core/v1"

const (
	NodeNameEnvVar     = "NODE_NAME"
	PodNameEnvVar      = "POD_NAME"
	PodNamespaceEnvVar = "POD_NAMESPACE"
)

const (
	DiscoverUdevBlacklist = "DISCOVER_UDEV_BLACKLIST"
	DiscoverDisk          = "DISCOVER_DISK"
	DiscoverInterval      = "DISCOVER_INTERVAL"
	OperatorImage         = "OPERATOR_IMAGE"
)

// NamespaceEnvVar namespace env var
func NamespaceEnvVar() v1.EnvVar {
	return v1.EnvVar{Name: PodNamespaceEnvVar, ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}}
}

// NameEnvVar pod name env var
func NameEnvVar() v1.EnvVar {
	return v1.EnvVar{Name: PodNameEnvVar, ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.name"}}}
}

// NodeEnvVar node env var
func NodeEnvVar() v1.EnvVar {
	return v1.EnvVar{Name: NodeNameEnvVar, ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}}
}
