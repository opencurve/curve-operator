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
	HostDataDir string `json:"hostDataDir,omitempty"`

	// +optional
	Etcd EtcdSpec `json:"etcd,omitempty"`

	// +optional
	Mds MdsSpec `json:"mds,omitempty"`

	// +optional
	MetaServer MetaServerSpec `json:"metaserver,omitempty"`

	// +optional
	SnapShotClone SnapShotCloneSpec `json:"snapShotClone,omitempty"`

	// Indicates user intent when deleting a cluster; blocks orchestration and should not be set if cluster
	// deletion is not imminent.
	// +optional
	// +nullable
	CleanupConfirm string `json:"cleanupConfirm,omitempty"`
}

// MdsSpec is the spec of mds
type MetaServerSpec struct {
	// +optional
	Port int `json:"port,omitempty"`

	// +optional
	ExternalPort int `json:"externalPort,omitempty"`

	// +optional
	CopySets int `json:"copySets,omitempty"`

	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// CurvefsStatus defines the observed state of Curvefs
type CurvefsStatus struct {
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
