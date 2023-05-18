package topology

import (
	"github.com/opencurve/curve-operator/pkg/clusterd"
	v1 "k8s.io/api/core/v1"
)

type DeployConfig struct {
	Ctx              clusterd.Context
	Namespace        string
	Image            string
	ImagePullPolicy  v1.PullPolicy
	Kind             string
	Role             string
	Copysets         int
	NodeName         string
	NodeIP           string
	Port             int
	DeviceName       string
	HostSequence     int
	ReplicasSequence int
	Replicas         int
	StandAlone       bool
}
