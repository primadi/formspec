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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the group/version for the FormSpec CRDs.
	GroupVersion = schema.GroupVersion{Group: "formspec.dev", Version: "v1alpha1"}

	// SchemeBuilder registers the FormSpec types into a runtime.Scheme. It uses
	// the apimachinery builder directly (not controller-runtime's scheme.Builder,
	// which is deprecated): an api package should depend only on apimachinery.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// addKnownTypes is the SchemeBuilder hook: it registers every CRD type and
// pins the group-version's metadata (metav1.AddToGroupVersion).
func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion,
		&ClusterClass{}, &ClusterClassList{},
		&Workspace{}, &WorkspaceList{},
		&Datastore{}, &DatastoreList{},
		&ResourceClaim{}, &ResourceClaimList{},
	)
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}
