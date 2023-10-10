/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const CustomResourceGroup = "curve.opencurve.io"

// ConditionType represents a resource's status
type ConditionType string

const (
	// ClusterPhasePending indicates the cluster is running to create.
	ClusterPhasePending ConditionType = "Pending"
	// ClusterPhaseReady indicates the cluster has been created successfully.
	ClusterPhaseReady ConditionType = "Ready" //nolint:unused
	// ClusterPhaseDeleting indicates the cluster is running to delete.
	ClusterPhaseDeleting ConditionType = "Deleting"
	// ClusterPhaseError indicates the cluster created failed because of some reason.
	ClusterPhaseError ConditionType = "Failed" //nolint:unused
	// ClusterPhaseUnknown is unknown phase
	ClusterPhaseUnknown ConditionType = "Unknown" //nolint:unused
)

const (
	// ConditionTypeEtcdReady indicates the etcd is ready
	ConditionTypeEtcdReady ConditionType = "EtcdReady"
	// ConditionTypeMdsReady indicates the mds is ready
	ConditionTypeMdsReady ConditionType = "MdsReady"
	// ConditionTypeFormatedReady indicates the formated job is ready
	ConditionTypeFormatedReady ConditionType = "formatedReady"
	// ConditionTypeChunkServerReady indicates the chunk server is ready
	ConditionTypeChunkServerReady ConditionType = "ChunkServerReady"
	// ConditionTypeMetaServerReady indicates the meta server is ready
	ConditionTypeMetaServerReady ConditionType = "MetaServerReady"
	// ConditionTypeSnapShotCloneReady indicates the snapshot clone is ready
	ConditionTypeSnapShotCloneReady ConditionType = "SnapShotCloneReady"
	// ConditionTypeDeleting indicates it's deleting
	ConditionTypeDeleting ConditionType = "Deleting"
	// ConditionTypeClusterReady indicates the cluster is ready
	ConditionTypeClusterReady ConditionType = "Ready"
	// ConditionTypeFailure indicates it's failed
	ConditionTypeFailure ConditionType = "Failed"
	// ConditionTypeUnknown is unknown condition
	ConditionTypeUnknown ConditionType = "Unknown" //nolint:unused
)

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"   //nolint:unused
	ConditionUnknown ConditionStatus = "Unknown" //nolint:unused
)

type ConditionReason string

const (
	ConditionEtcdClusterCreatedReason          ConditionReason = "EtcdClusterCreated"
	ConditionMdsClusterCreatedReason           ConditionReason = "MdsClusterCreated"
	ConditionFormatingChunkfilePoolReason      ConditionReason = "FormatingChunkfilePool"
	ConditionFormatChunkfilePoolReason         ConditionReason = "FormatedChunkfilePool"
	ConditionMetaServerClusterCreatedReason    ConditionReason = "MetaServerClusterCreated"
	ConditionChunkServerClusterCreatedReason   ConditionReason = "ChunkServerClusterCreated"
	ConditionSnapShotCloneClusterCreatedReason ConditionReason = "SnapShotCloneClusterCreated"
	ConditionClusterCreatedReason              ConditionReason = "ClusterCreated" //nolint:unused
	ConditionReconcileSucceeded                ConditionReason = "ReconcileSucceeded"
	ConditionReconcileFailed                   ConditionReason = "ReconcileFailed"
	ConditionDeletingClusterReason             ConditionReason = "Deleting"
)

type ClusterCondition struct {
	// Type is the type of condition.
	Type ConditionType `json:"type,omitempty"`
	// Status is the status of condition
	// Can be True, False or Unknown.
	Status ConditionStatus `json:"status,omitempty"`
	// ObservedGeneration
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// LastTransitionTime specifies last time the condition transitioned
	// from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	Reason ConditionReason `json:"reason,omitempty"`
	// Message is a human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

type ClusterVersion struct {
	Image string `json:"image,omitempty"`
}

// CurveClusterSpec defines the desired state of CurveCluster
type CurveClusterSpec struct {
	// +optional
	CurveVersion CurveVersionSpec `json:"curveVersion,omitempty"`

	// +optional
	Nodes []string `json:"nodes,omitempty"`

	// +optional
	HostDataDir string `json:"hostDataDir,omitempty"`

	// +optional
	Etcd EtcdSpec `json:"etcd,omitempty"`

	// +optional
	Mds MdsSpec `json:"mds,omitempty"`

	// +optional
	SnapShotClone SnapShotCloneSpec `json:"snapShotClone,omitempty"`

	// +optional
	Storage StorageScopeSpec `json:"storage,omitempty"`

	// Indicates user intent when deleting a cluster; blocks orchestration and should not be set if cluster
	// deletion is not imminent.
	// +optional
	// +nullable
	CleanupConfirm string `json:"cleanupConfirm,omitempty"`

	// +optional
	Monitor MonitorSpec `json:"monitor,omitempty"`
}

// CurveClusterStatus defines the observed state of CurveCluster
type CurveClusterStatus struct {
	// Phase is a summary of cluster state.
	// It can be translated from the last conditiontype
	Phase ConditionType `json:"phase,omitempty"`

	// Condition contains current service state of cluster such as progressing/Ready/Failure...
	Conditions []ClusterCondition `json:"conditions,omitempty"`

	// Message shows summary message of cluster from ClusterState
	// such as 'Curve Cluster Created successfully'
	Message string `json:"message,omitempty"`

	// CurveVersion shows curve version info on status field
	CurveVersion ClusterVersion `json:"curveVersion,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="HostDataDir",JSONPath=".spec.hostDataDir",type=string
// +kubebuilder:printcolumn:name="Version",JSONPath=".spec.curveVersion.image",type=string
// +kubebuilder:printcolumn:name="Phase",JSONPath=".status.phase",type=string

// CurveCluster is the Schema for the curveclusters API
type CurveCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CurveClusterSpec   `json:"spec,omitempty"`
	Status CurveClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CurveClusterList contains a list of CurveCluster
type CurveClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CurveCluster `json:"items"`
}

// CurveVersionSpec represents the settings for the Curve version
type CurveVersionSpec struct {
	// +optional
	Image string `json:"image,omitempty"`

	// +kubebuilder:validation:Enum=IfNotPresent;Always;Never;""
	// +optional
	ImagePullPolicy v1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// EtcdSpec is the spec of etcd
type EtcdSpec struct {
	// +optional
	PeerPort int `json:"peerPort,omitempty"`

	// +optional
	ClientPort int `json:"clientPort,omitempty"`

	// +optional
	Config map[string]int `json:"config,omitempty"`
}

// MdsSpec is the spec of mds
type MdsSpec struct {
	// +optional
	Port int `json:"port,omitempty"`

	// +optional
	DummyPort int `json:"dummyPort,omitempty"`

	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// SnapShotCloneSpec is the spec of snapshot clone
type SnapShotCloneSpec struct {
	// +optional
	Enable bool `json:"enable,omitempty"`

	// +optional
	Port int `json:"port,omitempty"`

	// +optional
	DummyPort int `json:"dummyPort,omitempty"`

	// +optional
	ProxyPort int `json:"proxyPort,omitempty"`

	// +optional
	S3Config S3ConfigSpec `json:"s3Config,omitempty"`
}

// S3ConfigSpec is the spec of s3 config
type S3ConfigSpec struct {
	AK                 string `json:"ak,omitempty"`
	SK                 string `json:"sk,omitempty"`
	NosAddress         string `json:"nosAddress,omitempty"`
	SnapShotBucketName string `json:"bucketName,omitempty"`
}

// StorageScopeSpec is the spec of storage scope
type StorageScopeSpec struct {
	// +optional
	UseSelectedNodes bool `json:"useSelectedNodes,omitempty"`

	// +optional
	Nodes []string `json:"nodes,omitempty"`

	// +optional
	Port int `json:"port,omitempty"`

	// +optional
	CopySets int `json:"copySets,omitempty"`

	// +optional
	Devices []DevicesSpec `json:"devices,omitempty"`

	// +optional
	SelectedNodes []SelectedNodesSpec `json:"selectedNodes,omitempty"`
}

// DevicesSpec represents a disk to use in the cluster
type DevicesSpec struct {
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	MountPath string `json:"mountPath,omitempty"`

	// +optional
	Percentage int `json:"percentage,omitempty"`
}

type SelectedNodesSpec struct {
	Node    string        `json:"node,omitempty"`
	Devices []DevicesSpec `json:"devices,omitempty"`
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

func init() {
	SchemeBuilder.Register(&CurveCluster{}, &CurveClusterList{})
}
