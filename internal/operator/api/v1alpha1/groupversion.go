// Package v1alpha1 contains the FormSpec CRD API types (group formspec.dev)
// consumed by formspec-operator. The schemas mirror
// docs/architecture/06-k8s-operator.md §3; the CRD YAML manifests live in
// deploy/operator/crds/.
//
// Deepcopy methods in zz_generated_deepcopy.go are hand-written (the repo
// does not use controller-gen). When adding fields with pointers, maps, or
// slices, update the deepcopy methods accordingly.
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group/version for the FormSpec CRDs.
	GroupVersion = schema.GroupVersion{Group: "formspec.dev", Version: "v1alpha1"}

	// SchemeBuilder registers the FormSpec types into a runtime.Scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&ClusterClass{}, &ClusterClassList{},
		&Workspace{}, &WorkspaceList{},
		&Datastore{}, &DatastoreList{},
		&ResourceClaim{}, &ResourceClaimList{},
	)
}
