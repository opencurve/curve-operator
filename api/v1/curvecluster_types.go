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

// ConditionType represent a resource's status
type ConditionType string

const (
	// ClusterPhasePending: The cluster is running to create.
	// ClusterPhaseReady: The cluster has been created successfully.
	// ClusterPhaseDeleting: The cluster is running to delete.
	// ClusterPhaseError: The cluster created failed becasue some reason.
	// ClusterPhaseUnknown: Unknow phase
	// ClusterPending
	ClusterPhasePending ConditionType = "Pending"
	// ClusterReady
	ClusterPhaseReady ConditionType = "Ready"
	// ClusterPhaseDeleting
	ClusterPhaseDeleting ConditionType = "Deleting"
	// ClusterPhaseError
	ClusterPhaseError ConditionType = "Failed"
	// ClusterPhaseUnknown
	ClusterPhaseUnknown ConditionType = "Unknown"
)

const (
	// ConditionTypeEtcdReady
	ConditionTypeEtcdReady ConditionType = "EtcdReady"
	// ConditionTypeMdsReady
	ConditionTypeMdsReady ConditionType = "MdsReady"
	// ConditionTypeFormatedReady
	ConditionTypeFormatedReady ConditionType = "formatedReady"
	// ConditionTypeChunkServerReady
	ConditionTypeChunkServerReady ConditionType = "ChunkServerReady"
	// ConditionTypeSnapShotCloneReady
	ConditionTypeSnapShotCloneReady ConditionType = "SnapShotCloneReady"
	// ConditionTypeDeleting
	ConditionTypeDeleting ConditionType = "Deleting"
	// ConditionTypeClusterReady
	ConditionTypeClusterReady ConditionType = "Ready"
	// ConditionTypeFailure
	ConditionTypeFailure ConditionType = "Failed"
	// ConditionTypeUnknown
	ConditionTypeUnknown ConditionType = "Unknow"
)

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

type ConditionReason string

const (
	ConditionEtcdClusterCreatedReason          ConditionReason = "EtcdClusterCreated"
	ConditionMdsClusterCreatedReason           ConditionReason = "MdsClusterCreated"
	ConditionFormatingChunkfilePoolReason      ConditionReason = "FormatingChunkfilePool"
	ConditionFormatChunkfilePoolReason         ConditionReason = "FormatedChunkfilePool"
	ConditionChunkServerClusterCreatedReason   ConditionReason = "ChunkServerClusterCreated"
	ConditionSnapShotCloneClusterCreatedReason ConditionReason = "SnapShotCloneClusterCreated"
	ConditionClusterCreatedReason              ConditionReason = "ClusterCreated"
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
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

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
}

// CurveClusterStatus defines the observed state of CurveCluster
type CurveClusterStatus struct {
	// Phase is a summary of cluster state.
	// It can be translate from the last conditiontype
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

	Spec   *CurveClusterSpec  `json:"spec,omitempty"`
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

// EtcdSpec
type EtcdSpec struct {
	// +optional
	PeerPort int `json:"peerPort,omitempty"`

	// +optional
	ClientPort int `json:"clientPort,omitempty"`

	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// MdsSpec
type MdsSpec struct {
	// +optional
	Port int `json:"port,omitempty"`

	// +optional
	DummyPort int `json:"dummyPort,omitempty"`

	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// SnapShotCloneSpec
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

// S3Config
type S3ConfigSpec struct {
	AK                 string `json:"ak,omitempty"`
	SK                 string `json:"sk,omitempty"`
	NosAddress         string `json:"nosAddress,omitempty"`
	SnapShotBucketName string `json:"bucketName,omitempty"`
}

// StorageScopeSpec
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

// Device represents a disk to use in the cluster
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

func init() {
	SchemeBuilder.Register(&CurveCluster{}, &CurveClusterList{})
}
