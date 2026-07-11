// Hand-written deepcopy implementations (this repo does not run
// controller-gen). Keep in sync with types.go: any new pointer, slice, or
// map field needs explicit copying here.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ─── ClusterClass ───

func (in *ClusterClassSpec) DeepCopyInto(out *ClusterClassSpec) {
	*out = *in
	if in.Features != nil {
		out.Features = make([]string, len(in.Features))
		copy(out.Features, in.Features)
	}
	if in.Pricing != nil {
		out.Pricing = new(ClusterClassPricing)
		*out.Pricing = *in.Pricing
	}
}

func (in *ClusterClass) DeepCopyInto(out *ClusterClass) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

func (in *ClusterClass) DeepCopy() *ClusterClass {
	if in == nil {
		return nil
	}
	out := new(ClusterClass)
	in.DeepCopyInto(out)
	return out
}

func (in *ClusterClass) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *ClusterClassList) DeepCopyInto(out *ClusterClassList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]ClusterClass, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *ClusterClassList) DeepCopy() *ClusterClassList {
	if in == nil {
		return nil
	}
	out := new(ClusterClassList)
	in.DeepCopyInto(out)
	return out
}

func (in *ClusterClassList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// ─── Workspace ───

func (in *WorkspaceSpec) DeepCopyInto(out *WorkspaceSpec) {
	*out = *in
	if in.Resources != nil {
		out.Resources = new(WorkspaceResources)
		*out.Resources = *in.Resources
	}
	if in.Datastores != nil {
		out.Datastores = make([]WorkspaceDatastoreRef, len(in.Datastores))
		copy(out.Datastores, in.Datastores)
	}
	if in.Cache != nil {
		out.Cache = make([]WorkspaceDatastoreRef, len(in.Cache))
		copy(out.Cache, in.Cache)
	}
}

func (in *WorkspaceStatus) DeepCopyInto(out *WorkspaceStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

func (in *Workspace) DeepCopyInto(out *Workspace) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *Workspace) DeepCopy() *Workspace {
	if in == nil {
		return nil
	}
	out := new(Workspace)
	in.DeepCopyInto(out)
	return out
}

func (in *Workspace) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *WorkspaceList) DeepCopyInto(out *WorkspaceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Workspace, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *WorkspaceList) DeepCopy() *WorkspaceList {
	if in == nil {
		return nil
	}
	out := new(WorkspaceList)
	in.DeepCopyInto(out)
	return out
}

func (in *WorkspaceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// ─── Datastore ───

func (in *DatastoreSpec) DeepCopyInto(out *DatastoreSpec) {
	*out = *in
	if in.AllowedTenants != nil {
		out.AllowedTenants = make([]string, len(in.AllowedTenants))
		copy(out.AllowedTenants, in.AllowedTenants)
	}
	if in.Capacity != nil {
		out.Capacity = new(DatastoreCapacity)
		*out.Capacity = *in.Capacity
	}
}

func (in *DatastoreStatus) DeepCopyInto(out *DatastoreStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

func (in *Datastore) DeepCopyInto(out *Datastore) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *Datastore) DeepCopy() *Datastore {
	if in == nil {
		return nil
	}
	out := new(Datastore)
	in.DeepCopyInto(out)
	return out
}

func (in *Datastore) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *DatastoreList) DeepCopyInto(out *DatastoreList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Datastore, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *DatastoreList) DeepCopy() *DatastoreList {
	if in == nil {
		return nil
	}
	out := new(DatastoreList)
	in.DeepCopyInto(out)
	return out
}

func (in *DatastoreList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// ─── ResourceClaim ───

func (in *ResourceClaimStatus) DeepCopyInto(out *ResourceClaimStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

func (in *ResourceClaim) DeepCopyInto(out *ResourceClaim) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

func (in *ResourceClaim) DeepCopy() *ResourceClaim {
	if in == nil {
		return nil
	}
	out := new(ResourceClaim)
	in.DeepCopyInto(out)
	return out
}

func (in *ResourceClaim) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *ResourceClaimList) DeepCopyInto(out *ResourceClaimList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]ResourceClaim, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *ResourceClaimList) DeepCopy() *ResourceClaimList {
	if in == nil {
		return nil
	}
	out := new(ResourceClaimList)
	in.DeepCopyInto(out)
	return out
}

func (in *ResourceClaimList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
