package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	PORT          = "port"
	PEER_PORT     = "peerPort"
	CLIENT_PORT   = "clientPort"
	DUMMY_PORT    = "dummyPort"
	PROXY_PORT    = "proxyPort"
	EXTERNAL_PORT = "externalPort"

	INSTANCES = "instances"
)

type ClusterPhase string

const (
	// ClusterCreating indicates the cluster is to be created.
	ClusterCreating ClusterPhase = "Creating"
	// ClusterPhaseReady indicates the cluster has been created successfully.
	ClusterRunning ClusterPhase = "Running"
	// ClusterUpdating indicates the cluster is to update config because of some server config change
	ClusterUpdating ClusterPhase = "Updating"
	// ClusterUpgrading indicates the cluster is to upgrade becasue 'Image' filed changed of 'CurveVersion'
	ClusterUpgrading ClusterPhase = "Upgrading"
	// ClusterScaling indicates the cluster is to scale becasue some server config change for chunkserver/metaserver replicas.
	ClusterScaling ClusterPhase = "Scaling"
	// ClusterPhaseDeleting indicates the cluster is running to delete.
	ClusterDeleting ClusterPhase = "Deleting"
	// ClusterPhaseUnknown means that for some reason the state of cluster could not be obtained.
	ClusterPhaseUnknown ClusterPhase = "Unknown"
)

// ConditionType represents a resource's status
type ConditionType string

const (
	// ConditionProgressing represents Progressing state of an object
	ConditionProgressing ConditionType = "Progressing"
	// ConditionClusterReady indicates the cluster is ready
	ConditionClusterReady ConditionType = "Ready"
	// ConditionDeleting indicates it's deleting
	ConditionDeleting ConditionType = "Deleting"
	// ConditionFailure indicates it's failed
	ConditionFailure ConditionType = "Failed"
)

type ConditionStatus string

const (
	ConditionStatusTrue    ConditionStatus = "True"
	ConditionStatusFalse   ConditionStatus = "False"   //nolint:unused
	ConditionStatusUnknown ConditionStatus = "Unknown" //nolint:unused
)

type ConditionReason string

const (
	ConditionDeletingClusterReason ConditionReason = "Deleting"
	ConditionReconcileStarted      ConditionReason = "ReconcileStarted"
	ConditionReconcileSucceeded    ConditionReason = "ReconcileSucceeded"
	ConditionReconcileFailed       ConditionReason = "ReconcileFailed"
)

type ClusterCondition struct {
	// Type is the type of condition.
	Type ConditionType `json:"type,omitempty"`
	// Status is the status of condition
	// Can be True, False or Unknown.
	Status ConditionStatus `json:"status,omitempty"`
	// LastTransitionTime specifies last time the condition transitioned
	// from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	Reason ConditionReason `json:"reason,omitempty"`
	// Message is a human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// CurveVersionSpec represents the settings for the Curve version
type CurveVersionSpec struct {
	// +optional
	Image string `json:"image,omitempty"`
}

// EtcdSpec is the spec of etcd
type EtcdSpec struct {
	// +optional
	PeerPort *int `json:"peerPort,omitempty"`
	// +optional
	ClientPort *int `json:"clientPort,omitempty"`
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// MdsSpec is the spec of mds
type MdsSpec struct {
	// +optional
	Port *int `json:"port,omitempty"`
	// +optional
	DummyPort *int `json:"dummyPort,omitempty"`
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// StorageScopeSpec is the spec of storage scope
type StorageScopeSpec struct {
	// +optional
	Port *int `json:"port,omitempty"`
	// +optional
	Instances int `json:"instances,omitempty"`
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// SnapShotCloneSpec is the spec of snapshot clone
type SnapShotCloneSpec struct {
	// +optional
	Enable bool `json:"enable,omitempty"`
	// +optional
	Port *int `json:"port,omitempty"`
	// +optional
	DummyPort *int `json:"dummyPort,omitempty"`
	// +optional
	ProxyPort *int `json:"proxyPort,omitempty"`
	// +optional
	S3Config S3ConfigSpec `json:"s3,omitempty"`
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// S3ConfigSpec is the spec of s3 config
type S3ConfigSpec struct {
	AK                 string `json:"ak,omitempty"`
	SK                 string `json:"sk,omitempty"`
	NosAddress         string `json:"nosAddress,omitempty"`
	SnapShotBucketName string `json:"bucketName,omitempty"`
}

// MdsSpec is the spec of mds
type MetaServerSpec struct {
	// +optional
	Port *int `json:"port,omitempty"`
	// +optional
	ExternalPort *int `json:"externalPort,omitempty"`
	// +optional
	Instances int `json:"instances,omitempty"`
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type MonitorSpec struct {
	Enable bool `json:"enable,omitempty"`
	// +optional
	MonitorHost string `json:"monitorHost,omitempty"`
	// +optional
	Prometheus PrometheusSpec `json:"prometheus,omitempty"`
	// +optional
	Grafana GrafanaSpec `json:"grafana,omitempty"`
	// +optional
	NodeExporter NodeExporterSpec `json:"nodeExporter,omitempty"`
}

type PrometheusSpec struct {
	// +optional
	ContainerImage string `json:"containerImage,omitempty"`
	// +optional
	DataDir string `json:"dataDir,omitempty"`
	// +optional
	ListenPort int `json:"listenPort,omitempty"`
	// +optional
	RetentionTime string `json:"retentionTime,omitempty"`
	// +optional
	RetentionSize string `json:"retentionSize,omitempty"`
}

type GrafanaSpec struct {
	// +optional
	ContainerImage string `json:"containerImage,omitempty"`
	// +optional
	DataDir string `json:"dataDir,omitempty"`
	// +optional
	ListenPort int `json:"listenPort,omitempty"`
	// +optional
	UserName string `json:"userName,omitempty"`
	// +optional
	PassWord string `json:"passWord,omitempty"`
}

type NodeExporterSpec struct {
	// +optional
	ContainerImage string `json:"containerImage,omitempty"`
	// +optional
	ListenPort int `json:"listenPort,omitempty"`
}

type StorageStatusDir struct {
	// DataDir record the cluster data storage directory
	DataDir string `json:"dataDir,omitempty"`
	// LogDir record the cluster log storage directory
	LogDir string `json:"logDir,omitempty"`
}

type ModContext struct {
	// Role represents the service role of modification
	Role string `json:"role,omitempty"`
	// Parameter represents the parameters of modification
	Parameters map[string]string `json:"parameters,omitempty"`
}

type LastModContextSet struct {
	ModContextSet []ModContext `json:"modContextSet,omitempty"`
}
