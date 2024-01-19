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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CurvefsSpec defines the desired state of Curvefs
type CurvefsSpec struct {
	// +optional
	CurveVersion CurveVersionSpec `json:"curveVersion,omitempty"`
	// +optional
	Nodes []string `json:"nodes,omitempty"`
	// +optional
	DataDir string `json:"dataDir,omitempty"`
	// +optional
	LogDir string `json:"logDir,omitempty"`
	// +optional
	Copysets *int `json:"copysets,omitempty"`
	// +optional
	Etcd *EtcdSpec `json:"etcd,omitempty"`
	// +optional
	Mds *MdsSpec `json:"mds,omitempty"`
	// +optional
	MetaServer *MetaServerSpec `json:"metaserver,omitempty"`
}

// CurvefsStatus defines the observed state of Curvefs
type CurvefsStatus struct {
	// Phase is a summary of cluster state.
	// It can be translated from the last conditiontype
	// ClusterPending: The cluster has been accepted by system, but in the process
	// ClusterRunning: The cluster is healthy and is running process
	// ClusterDeleting: The cluster is in deleting process
	// ClusterUnknown: The cluster state is unknown
	Phase ClusterPhase `json:"phase,omitempty"`
	// Condition contains current service state of cluster such as progressing/Ready/Failure...
	Conditions []ClusterCondition `json:"conditions,omitempty"`
	// Message shows summary message of cluster from ClusterState
	// such as 'Curve Cluster Created successfully'
	Message string `json:"message,omitempty"`
	// CurveVersion shows curve version info on status field that judge iff upgrade
	CurveVersion CurveVersionSpec `json:"curveVersion,omitempty"`
	// LastModContextSet means that need to modify operatrion context
	LastModContextSet LastModContextSet `json:"lastModContextSet,omitempty"`
	// DataDir and LogDir is to compare and update
	StorageDir StorageStatusDir `json:"storageStatusDir,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="DataDir",JSONPath=".spec.dataDir",type=string
// +kubebuilder:printcolumn:name="LogDir",JSONPath=".spec.logDir",type=string
// +kubebuilder:printcolumn:name="Version",JSONPath=".spec.curveVersion.image",type=string
// +kubebuilder:printcolumn:name="Phase",JSONPath=".status.phase",type=string

// Curvefs is the Schema for the curvefsclusters API
type Curvefs struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CurvefsSpec   `json:"spec,omitempty"`
	Status CurvefsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CurvefsClusterList contains a list of CurvefsCluster
type CurvefsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Curvefs `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Curvefs{}, &CurvefsList{})
}
