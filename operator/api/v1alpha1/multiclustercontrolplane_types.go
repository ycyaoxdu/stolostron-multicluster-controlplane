/*
Copyright 2023.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opreatorv1 "open-cluster-management.io/api/operator/v1"
)

// StorageMode represents the mode of data storage
type StorageMode string

const (
	// StorageModeEmbedded is the default storage mode.
	// The controlplane will be deployed using embedded etcd as stoarge.
	StorageModeEmbedded StorageMode = "Embedded"

	// StorageModeExternal means storage data outside of the controlplane pod.
	// The controlplane will be deployed using external etcd address as storage.
	StorageModeExternal StorageMode = "External"
)

type ExternalEtcdConfiguration struct {
}

// ControlplaneStorageOption describes the storage options
type ControlplaneStorageOption struct {
	// Mode can be Embedded or External.
	// In Embedded mode, the controlplane saves all data in embedded etcd.
	// In External mode, the controlplane connect to external etcd and saves all data in it.
	// Note: Do not modify the Mode field once it's applied.
	// +required
	// +default=Embedded
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Embedded
	// +kubebuilder:validation:Enum=Embedded;External
	Mode StorageMode `json:"mode,omitempty"`

	// External includes configurations we needs for connect to external etcd in the External mode.
	// +optional
	External *ExternalEtcdConfiguration `json:"external,omitempty"`
}

// MulticlusterControlplaneSpec defines the desired state of MulticlusterControlplane
type MulticlusterControlplaneSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ControlplaneImagePullSpec represents the desired image of multicluster controlplane.
	// +optional
	// +kubebuilder:default=quay.io/open-cluster-management/multicluster-controlplane
	ControlplaneImagePullSpec string `json:"controlplaneImagePullSpec,omitempty"`

	// StorageOption contains the options of storage.
	// Embedded mode is used if StorageOption is not set.
	// +optional
	// +kubebuilder:default={mode: Embedded}
	StorageOption ControlplaneStorageOption `json:"storageOption,omitempty"`
}

// MulticlusterControlplaneStatus defines the observed state of MulticlusterControlplane
type MulticlusterControlplaneStatus struct {
	// ObservedGeneration is the last generation change you've dealt with
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions contain the different condition statuses for this ClusterManager.
	// Valid condition types are:
	// Applied: Components in hub are applied.
	// Available: Components in hub are available and ready to serve.
	// Progressing: Components in hub are in a transitioning state.
	// Degraded: Components in hub do not match the desired configuration and only provide
	// degraded service.
	Conditions []metav1.Condition `json:"conditions"`

	// Generations are used to determine when an item needs to be reconciled or has changed in a way that needs a reaction.
	// +optional
	Generations []opreatorv1.GenerationStatus `json:"generations,omitempty"`
}

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MulticlusterControlplane is the Schema for the multiclustercontrolplanes API
type MulticlusterControlplane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MulticlusterControlplaneSpec   `json:"spec,omitempty"`
	Status MulticlusterControlplaneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MulticlusterControlplaneList contains a list of MulticlusterControlplane
type MulticlusterControlplaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MulticlusterControlplane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MulticlusterControlplane{}, &MulticlusterControlplaneList{})
}
