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

// ClusterPhase represents lifecycle phases
type ClusterPhase string

const (
	// ClusterPending
	// cluster is Pending phase when creating or deleting or updating(not implement).
	ClusterPhaseCreating ClusterPhase = "Pending"
	// ClusterReady
	// Cluster is Ready phase when all servers are in Ready.
	ClusterPhaseReady ClusterPhase = "Ready"
	// ClusterPhaseError
	// Cluster is Failed phase when any condition is Failed type.
	ClusterPhaseError ClusterPhase = "Failed"
	// ClusterPhaseUnknown
	// Cluster is unknown phase because of unknown reason.
	ClusterPhaseUnknown ClusterPhase = "Unknown"
)

// ConditionType represent a resource's status
type ConditionType string

const (
	// ConditionEtcdReady
	ConditionEtcdReady ConditionType = "EtcdReady"
	// ConditionMdsReady
	ConditionMdsReady ConditionType = "MdsReady"
	// ConditionSnapShotCloneReady
	ConditionSnapShotCloneReady ConditionType = "SnapShotCloneReady"
	// ConditionChunkServerReady
	ConditionChunkServerReady ConditionType = "ChunkServerReady"
	// ConditionReady represents Ready state of an object when cluster is created successed.
	ConditionReady ConditionType = "Ready"
	// ConditionFailure represents Failure state of an object
	ConditionFailure ConditionType = "Failed"
	// ConditionDeletionIsBlocked represents when deletion of the object is blocked.
	ConditionDeletionIsBlocked ConditionType = "DeletionIsBlocked"
)

// Cluster represents state of a cluster.
// Only represents the ongoing state of the entire cluster.
type ClusterState string

const (
	// ClusterStateCreating
	// Cluster is being created.
	ClusterStateCreating ClusterState = "Creating"
	// ClusterStateCreated
	// Cluster has been created and in Ready.
	ClusterStateCreated ClusterState = "Created"
	// ClusterStateUpdating(Not implement temporary)
	// Cluster is being updated.
	ClusterStateUpdating ClusterState = "Updating"
	// ClusterStateDeleting
	// Cluster is being deleted
	ClusterStateDeleting ClusterState = "Deleting"
)

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
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
	Reason string `json:"reason,omitempty"`
	// Message is a human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

type ClusterVersion struct {
	Image   string `json:"image,omitempty"`
	Version string `json:"version,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CurveClusterSpec defines the desired state of CurveCluster
type CurveClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	CurveVersion CurveVersionSpec `json:"curveVersion,omitempty"`

	// +optional
	Nodes []string `json:"nodes,omitempty"`

	// +optional
	DataDirHostPath string `json:"dataDirHostPath,omitempty"`

	// +optional
	LogDirHostPath string `json:"logDirHostPath,omitempty"`

	// +optional
	Etcd EtcdSpec `json:"etcd,omitempty"`

	// +optional
	Mds MdsSpec `json:"mds,omitempty"`

	// +optional
	SnapShotClone SnapShotCloneSpec `json:"snapShotClone,omitempty"`

	// +optional
	Storage StorageScopeSpec `json:"storage,omitempty"`
}

// CurveClusterStatus defines the observed state of CurveCluster
type CurveClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase is a summary of cluster state.
	// It can be translate from the last conditiontype
	Phase ClusterPhase `json:"phase,omitempty"`

	// State represents the state of a cluster.
	State ClusterState `json:"state,omitempty"`

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
// +kubebuilder:printcolumn:name="DataDirHostPath",JSONPath=".spec.dataDirHostPath",type=string
// +kubebuilder:printcolumn:name="LogDirHostPath",JSONPath=".spec.logDirHostPath",type=string
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

// EtcdSpec
type EtcdSpec struct {
	// +optional
	Port int `json:"port,omitempty"`

	// +optional
	ListenPort int `json:"listenPort,omitempty"`

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
	SnapShotBucketName string `json:"snapShotBucketName,omitempty"`
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
