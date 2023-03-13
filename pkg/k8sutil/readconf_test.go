package k8sutil

import (
	"testing"
)

func TestReadConfFromTemplate(t *testing.T) {
	_, err := ReadConfFromTemplate("../template/mds.conf")
	if err != nil {
		t.Error("failed to read config file")
	}
}
