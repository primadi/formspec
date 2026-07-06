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
}

// Dependency declares a module dependency.
type Dependency struct {
	Module string `yaml:"module" json:"module"`
}

// AppSpec is the root project manifest (Core §4.4).
type AppSpec struct {
	Version   string   `yaml:"version" json:"version"`
	Vendor    string   `yaml:"vendor" json:"vendor"`
	Modules   []string `yaml:"modules" json:"modules"`
	Publishes []string `yaml:"publishes,omitempty" json:"publishes,omitempty"`
	Consumes  []string `yaml:"consumes,omitempty" json:"consumes,omitempty"`
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
