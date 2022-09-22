/*
Copyright 2022.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Status string

const (
	Pending Status = "pending"
	//Succeed  Status = "succeed"
	//Failed   Status = "failed"
	Finished Status = "finished"
)

type Task struct {
	Name        string            `json:"name,omitempty"`
	NameSpace   string            `json:"nameSpace,omitempty"`
	Status      Status            `json:"status,omitempty"`
	ImageStatus map[string]Status `json:"imageStatus,omitempty"`
}

type Selector struct {
	Names       []string          `json:"names,omitempty"`
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// ImagePullJobSpec defines the desired state of ImagePullJob
type ImagePullJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ImagePullJob. Edit imagepulljob_types.go to remove/update
	Foo string `json:"foo,omitempty"`
	//+option
	Selector Selector `json:"selector,omitempty"`

	Images []string `json:"images,omitempty"`
	// TODO timeout
}

// ImagePullJobStatus defines the observed state of ImagePullJob
type ImagePullJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status Status           `json:"status,omitempty"`
	Nodes  []string         `json:"nodes,omitempty"`
	Tasks  map[string]*Task `json:"tasks,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ImagePullJob is the Schema for the imagepulljobs API
type ImagePullJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImagePullJobSpec   `json:"spec,omitempty"`
	Status ImagePullJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ImagePullJobList contains a list of ImagePullJob
type ImagePullJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImagePullJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ImagePullJob{}, &ImagePullJobList{})
}
