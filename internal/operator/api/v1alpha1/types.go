package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types used across the FormSpec CRDs
// (docs/runtimes/03-formspec-operator.md §3.3).
const (
	ConditionReady       = "Ready"
	ConditionProgressing = "Progressing"
	ConditionDegraded    = "Degraded"
	ConditionValidated   = "Validated"
	ConditionDenied      = "Denied"
)

// ─── ClusterClass ───

// ClusterClassScaling controls replica policy for workspaces in this class
// (docs/architecture/05-failover.md §3.2, D-ARCH-31).
type ClusterClassScaling struct {
	// MinReplicas is the replica floor (economy: 0 with scale-to-zero,
	// standard: 1, premium: 2+).
	MinReplicas int32 `json:"minReplicas"`
	// ScaleToZero allows idle workspaces to be scaled to 0 replicas.
	ScaleToZero bool `json:"scaleToZero,omitempty"`
}

// ClusterClassPricing is the published price card for the class.
type ClusterClassPricing struct {
	Currency     string `json:"currency,omitempty"`
	BaseMonthly  int64  `json:"baseMonthly,omitempty"`
	PerWorkspace int64  `json:"perWorkspace,omitempty"`
	PerGBStorage int64  `json:"perGBStorage,omitempty"`
}

// ClusterClassSpec defines an SLA/pricing tier, authored by the Cloud Owner.
type ClusterClassSpec struct {
	SLA           string               `json:"sla,omitempty"`
	Region        string               `json:"region,omitempty"`
	Availability  string               `json:"availability,omitempty"`
	NodeType      string               `json:"nodeType,omitempty"`
	Storage       string               `json:"storage,omitempty"`
	MaxWorkspaces int32                `json:"maxWorkspaces,omitempty"`
	Features      []string             `json:"features,omitempty"`
	Scaling       ClusterClassScaling  `json:"scaling,omitempty"`
	Pricing       *ClusterClassPricing `json:"pricing,omitempty"`
}

// ClusterClass is pure configuration — it has no status subresource and the
// operator never reconciles K8s resources from it directly; Workspace
// reconciliation reads it for scaling/features.
type ClusterClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterClassSpec `json:"spec,omitempty"`
}

// ClusterClassList contains a list of ClusterClass.
type ClusterClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterClass `json:"items"`
}

// HasFeature reports whether the class declares a feature (e.g. "auto-scaling").
func (c *ClusterClass) HasFeature(name string) bool {
	for _, f := range c.Spec.Features {
		if f == name {
			return true
		}
	}
	return false
}

// ─── Workspace ───

// WorkspaceResources is the compute request for the workspace pod.
type WorkspaceResources struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// WorkspaceDatastoreRef names a datastore the workspace wants to use.
type WorkspaceDatastoreRef struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// WorkspaceSpec is created when a Workspace Owner provisions a workspace.
type WorkspaceSpec struct {
	// Owner is the workspace owner key fingerprint.
	Owner string `json:"owner,omitempty"`
	// Region for data residency (matched against formspec.dev/region node labels).
	Region string `json:"region,omitempty"`
	// ClusterClass selects the SLA/pricing tier.
	ClusterClass string `json:"clusterClass"`
	// Cluster pins a specific cluster (enterprise); usually empty.
	Cluster string `json:"cluster,omitempty"`
	// Environment is prod/staging/dev (matched against formspec.dev/environment).
	Environment string                  `json:"environment,omitempty"`
	Resources   *WorkspaceResources     `json:"resources,omitempty"`
	Datastores  []WorkspaceDatastoreRef `json:"datastores,omitempty"`
	Cache       []WorkspaceDatastoreRef `json:"cache,omitempty"`
}

// WorkspaceStatus is maintained by the WorkspaceReconciler.
type WorkspaceStatus struct {
	// Phase is a one-word summary: Pending, Ready, Degraded.
	Phase string `json:"phase,omitempty"`
	// Conditions: Ready, Progressing, Degraded.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ReadyReplicas mirrors the underlying Deployment.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// ObservedGeneration is the spec generation last reconciled.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// Workspace maps 1:1 to a formspec-resource Deployment in the cluster.
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

// WorkspaceList contains a list of Workspace.
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

// ─── Datastore ───

// SecretKeyRef points at one key inside a Secret.
type SecretKeyRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// DatastoreCapacity declares provisioned limits, used for registry accounting.
type DatastoreCapacity struct {
	MaxConnections int32 `json:"maxConnections,omitempty"`
	StorageGB      int32 `json:"storageGB,omitempty"`
}

// DatastoreSpec registers an external DB/cache/queue with the cluster.
type DatastoreSpec struct {
	// Driver: postgres, valkey, redis, ... (mirrors pkg/spec DatastoreDriver).
	Driver string `json:"driver"`
	// EndpointSecretRef locates the connection string.
	EndpointSecretRef SecretKeyRef `json:"endpointSecretRef"`
	// AllowedTenants lists principals allowed to claim this datastore,
	// e.g. "workspace:bank-mandiri-prod".
	AllowedTenants []string `json:"allowedTenants,omitempty"`
	// Owner is the registering principal (e.g. "cloud-owner").
	Owner string `json:"owner,omitempty"`
	// OwnerPublicKey is the hex-encoded ed25519 public key of the owner,
	// used to verify ResourceClaim signatures (docs/architecture/04 §5.2).
	OwnerPublicKey string             `json:"ownerPublicKey,omitempty"`
	Capacity       *DatastoreCapacity `json:"capacity,omitempty"`
}

// DatastoreStatus is maintained by the DatastoreReconciler.
type DatastoreStatus struct {
	// Conditions: Validated, Denied.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Datastore is a registered external data resource.
type Datastore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatastoreSpec   `json:"spec,omitempty"`
	Status DatastoreStatus `json:"status,omitempty"`
}

// DatastoreList contains a list of Datastore.
type DatastoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Datastore `json:"items"`
}

// ─── ResourceClaim ───

// ResourceClaimSpec grants a workspace access to a datastore. The grant is
// signed by the resource owner; the operator verifies the signature and the
// datastore's allowedTenants before injecting credentials.
type ResourceClaimSpec struct {
	Datastore  string `json:"datastore"`
	Workspace  string `json:"workspace"`
	Permission string `json:"permission,omitempty"` // read-only | read-write
	GrantedBy  string `json:"grantedBy,omitempty"`
	GrantedAt  string `json:"grantedAt,omitempty"`
	// Signature is the hex-encoded ed25519 signature over SignedMessage().
	Signature string `json:"signature,omitempty"`
}

// SignedMessage is the canonical byte string the owner signs:
// "datastore|workspace|permission|grantedBy|grantedAt".
func (s *ResourceClaimSpec) SignedMessage() []byte {
	return []byte(s.Datastore + "|" + s.Workspace + "|" + s.Permission + "|" + s.GrantedBy + "|" + s.GrantedAt)
}

// ResourceClaimStatus is maintained by the ResourceClaimReconciler.
type ResourceClaimStatus struct {
	// Conditions: Ready, Denied.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ResourceClaim is a signed permission grant: which datastore a workspace
// may access, and how.
type ResourceClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceClaimSpec   `json:"spec,omitempty"`
	Status ResourceClaimStatus `json:"status,omitempty"`
}

// ResourceClaimList contains a list of ResourceClaim.
type ResourceClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceClaim `json:"items"`
}
