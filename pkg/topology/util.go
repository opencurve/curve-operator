package topology

import (
	"fmt"
	"strings"
)

func Choose(ok bool, first, second string) string {
	if ok {
		return first
	}
	return second
}

// helper function
// etcd_hostname_0_0
func formatId(role, host string, hostSequence, instancesSequence int) string {
	return fmt.Sprintf("%s_%s_%d_%d", role, host, hostSequence, instancesSequence)
}

// etcd00
func formatName(role string, hostSequence, instancesSequence int) string {
	return fmt.Sprintf("%s%d%d", role, hostSequence, instancesSequence)
}

// isEmptyString trim the left and right space of a string, if "" return true, else return false
func isEmptyString(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

// trimString trim the left and right space and '/' right of string
func trimString(s string) string {
	ret := strings.TrimSpace(s)
	ret = strings.TrimSuffix(ret, "/")
	return ret
}
