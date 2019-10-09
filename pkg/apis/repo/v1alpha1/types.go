package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Repo is a specification for a Repo resource
type Repo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepoSpec   `json:"spec"`
	Status RepoStatus `json:"status"`
}

// RepoSpec is the spec for a Repo resource
type RepoSpec struct {
	Url string `json:"url"`
}

// RepoStatus is the status for a Repo resource
type RepoStatus struct {
	LastScanned string `json:"lastScanned"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RepoList is a list of Repo resources
type RepoList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata"`

	Items []Repo `json:"items"`
}
