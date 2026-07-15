package spec

// ServiceSpec defines a stateless, pure computation resource (Core §4.2).
type ServiceSpec struct {
	Version string      `yaml:"version" json:"version"`
	Actions []Action    `yaml:"actions" json:"actions"`
	Auth    *EntityAuth `yaml:"auth,omitempty" json:"auth,omitempty"`
}

// ModuleSpec defines a package of manifests (Core §4.3, Ref D19).
type ModuleSpec struct {
	Version string       `yaml:"version" json:"version"`
	Depends []Dependency `yaml:"depends,omitempty" json:"depends,omitempty"`
	// Menu is a default navigation suggestion, module-relative (no `Module`
	// field on its items — it's implicitly this module). An App adopts it
	// wholesale via a `type: module` MenuItem (Core §4.4/§4.5).
	Menu []MenuItem `yaml:"menu,omitempty" json:"menu,omitempty"`
}

// Dependency declares a module dependency.
type Dependency struct {
	Module string `yaml:"module" json:"module"`
}

// AppSpec is the root project manifest (Core §4.4). A Workspace MAY contain
// more than one App; all Apps in a workspace run simultaneously, mounted at
// their own RootURL.
type AppSpec struct {
	Version   string     `yaml:"version" json:"version"`
	Vendor    string     `yaml:"vendor" json:"vendor"`
	RootURL   string     `yaml:"root_url" json:"root_url"`
	Modules   []string   `yaml:"modules" json:"modules"`
	Menu      []MenuItem `yaml:"menu,omitempty" json:"menu,omitempty"`
	Publishes []string   `yaml:"publishes,omitempty" json:"publishes,omitempty"`
	Consumes  []string   `yaml:"consumes,omitempty" json:"consumes,omitempty"`
}

// MenuItem is one navigation entry, embedded in App.spec.menu or
// Module.spec.menu (Core §4.4/§4.5) — there is no standalone kind: Menu.
//
// Every item is exactly one of three shapes, enforced by the loader/resolver
// (see internal/manifest), not by separate Go types:
//
//   - Adopt node (Type == "module"): only Module set (+ optional Order).
//     Splices that Module's own Menu wholesale at this position. Level 1 only.
//   - Group node (len(Children) > 0): Label + Children. Module/View/Route
//     forbidden on the group itself — only its descendants carry Module.
//   - Leaf node (no Children, Type != "module"): Label + Module + exactly one
//     of View/Route. Level 3 leaves cannot have Children (3-level cap).
type MenuItem struct {
	Type     string     `yaml:"type,omitempty" json:"type,omitempty"` // "module" = adopt-shorthand node
	Label    string     `yaml:"label,omitempty" json:"label,omitempty"`
	Icon     string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Module   string     `yaml:"module,omitempty" json:"module,omitempty"`
	View     string     `yaml:"view,omitempty" json:"view,omitempty"`   // name of a registered View resource
	Route    string     `yaml:"route,omitempty" json:"route,omitempty"` // raw URL escape hatch (no registered View)
	When     string     `yaml:"when,omitempty" json:"when,omitempty"`   // FormaExpr business condition
	Order    int        `yaml:"order,omitempty" json:"order,omitempty"`
	Children []MenuItem `yaml:"children,omitempty" json:"children,omitempty"`
}

// ConfigSpec defines runtime configuration values (Core §4.5).
type ConfigSpec struct {
	Values map[string]any `yaml:"values" json:"values"`
	Schema map[string]any `yaml:"schema,omitempty" json:"schema,omitempty"`
}

// SubscriptionSpec subscribes to another resource's events (Core §4.6, Ref D35).
type SubscriptionSpec struct {
	Events  []string `yaml:"events" json:"events"`
	Handler ImplDecl `yaml:"handler" json:"handler"`
}
