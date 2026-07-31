package spec

// ─── Control Plane Kind Specs ───
// (platform/04-control-plane.md §2, §5)
//
// These types define the *contract* for kind: Environment and kind: Policy
// manifests. Control Plane runtime execution (forma-ctl) is still a separate
// effort — the structs exist so manifests parse, validate, and get JSON Schema
// autocomplete/validation today.

// EnvironmentSpec defines one deployment target (platform/04-control-plane.md §2).
type EnvironmentSpec struct {
	// Mode is the deployment environment mode — dev | prod.
	// @schema {description: "Environment mode", enum: ["dev", "prod"]}
	Mode EnvironmentMode `yaml:"mode" json:"mode"`
	// Tier classifies the deployment — standalone | cloud | enterprise.
	// @schema {description: "Deployment tier", enum: ["standalone", "cloud", "enterprise"]}
	Tier string `yaml:"tier" json:"tier"`
	// ResourcePool is the resource pool strategy — dev is always shared;
	// prod is shared (prod-shared) or exclusive.
	// @schema {description: "Resource pool strategy", enum: ["shared", "exclusive"]}
	ResourcePool string `yaml:"resource_pool" json:"resource_pool"`
	// ResourcePlanes lists the resource plane endpoints for this environment.
	ResourcePlanes []EnvironmentPlane `yaml:"resource_planes" json:"resource_planes"`
	// KeyRef is the location of the platform signing key (e.g. kms://prod-signing).
	KeyRef string `yaml:"key_ref" json:"key_ref"`
	// Policy names the kind: Policy that applies to this environment.
	Policy string `yaml:"policy" json:"policy"`
}

// EnvironmentPlane is one resource plane endpoint of an Environment.
type EnvironmentPlane struct {
	URL string `yaml:"url" json:"url"`
}

// PolicySpec defines governance rules evaluated on control-plane decisions
// (platform/04-control-plane.md §5). The escape hatch `rego` is compiled into
// the same OPA evaluation engine as the structured vocabulary.
type PolicySpec struct {
	RequireSigning      bool             `yaml:"require_signing" json:"require_signing"`
	RequireStagingFirst bool             `yaml:"require_staging_first" json:"require_staging_first"`
	RequireApproval     []PolicyApproval `yaml:"require_approval,omitempty" json:"require_approval,omitempty"`
	// Blocked is the policy floor — rules that cannot be configured away.
	Blocked []string `yaml:"blocked,omitempty" json:"blocked,omitempty"`
	// Rego is the escape hatch — full OPA policy body.
	Rego string `yaml:"rego,omitempty" json:"rego,omitempty"`
}

// PolicyApproval declares an approval rule for a class of implementation types.
type PolicyApproval struct {
	Impl          []ImplType `yaml:"impl" json:"impl"`
	Approvers     int        `yaml:"approvers" json:"approvers"`
	ApproverRoles []string   `yaml:"approver_roles" json:"approver_roles"`
}
