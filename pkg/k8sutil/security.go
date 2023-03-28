package k8sutil

import v1 "k8s.io/api/core/v1"

// PrivilegedContext returns a privileged Pod security context
func PrivilegedContext(runAsRoot bool) *v1.SecurityContext {
	privileged := true
	rootUser := int64(0)

	sec := &v1.SecurityContext{
		Privileged: &privileged,
	}

	if runAsRoot {
		sec.RunAsUser = &rootUser
	}

	return sec
}
