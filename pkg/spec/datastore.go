// Package spec — Datastore Kind
//
// Defines the Go types for kind: Datastore, a Control Plane resource
// that provisions named infrastructure backends for Forma primitives.
//
// Datastores are defined and managed exclusively by the Control Plane.
// The Resource Plane receives authorized datastores via the Plane Protocol
// snapshot and routes ctx.* primitive calls to them by name.
package spec

// DatastoreDriver identifies the backend technology.
// @schema {description: "Backend database/driver technology", enum: ["sqlite", "postgres", "valkey", "redis", "s3", "minio", "nats", "memory", "fs"]}
type DatastoreDriver string

const (
	DatastoreDriverSQLite   DatastoreDriver = "sqlite"
	DatastoreDriverPostgres DatastoreDriver = "postgres"
	DatastoreDriverValkey   DatastoreDriver = "valkey"
	DatastoreDriverRedis    DatastoreDriver = "redis"
	DatastoreDriverS3       DatastoreDriver = "s3"
	DatastoreDriverMinio    DatastoreDriver = "minio"
	DatastoreDriverNATS     DatastoreDriver = "nats"
	DatastoreDriverMemory   DatastoreDriver = "memory"
	DatastoreDriverFS       DatastoreDriver = "fs"
)

// PrimitiveType identifies which ctx.* primitive a datastore backs.
// @schema {description: "ctx.* primitive type this datastore backs", enum: ["db", "cache", "lock", "queue", "pubsub", "storage", "config", "kvstore", "log"]}
type PrimitiveType string

const (
	PrimitiveDB      PrimitiveType = "db"
	PrimitiveCache   PrimitiveType = "cache"
	PrimitiveLock    PrimitiveType = "lock"
	PrimitiveQueue   PrimitiveType = "queue"
	PrimitivePubSub  PrimitiveType = "pubsub"
	PrimitiveStorage PrimitiveType = "storage"
	PrimitiveConfig  PrimitiveType = "config"
	PrimitiveKVStore PrimitiveType = "kvstore"
	PrimitiveLog     PrimitiveType = "log"
)

// AccessPermission is the permission level for a datastore.
// @schema {description: "Access permission level", enum: ["read", "write", "read_write"]}
type AccessPermission string

const (
	AccessRead      AccessPermission = "read"
	AccessWrite     AccessPermission = "write"
	AccessReadWrite AccessPermission = "read_write"
)

// DatastoreSpec is the spec body for kind: Datastore.
type DatastoreSpec struct {
	// Serves lists which ctx.* primitives this datastore backs.
	// One physical backend can serve multiple primitive types.
	// Example: a Valkey instance can serve [cache, lock, kvstore, queue, pubsub].
	Serves []PrimitiveType `yaml:"serves" json:"serves"`

	// Driver identifies the backend technology.
	Driver DatastoreDriver `yaml:"driver" json:"driver"`

	// Connection holds connection parameters for the backend.
	Connection DatastoreConnection `yaml:"connection" json:"connection"`

	// CredentialRef is a reference to KMS/Vault for credentials.
	// Credentials MUST NOT be inlined in the YAML.
	CredentialRef string `yaml:"credential_ref,omitempty" json:"credential_ref,omitempty"`

	// Access controls who (filter) can use this datastore and what
	// operations (permission) they are allowed. If omitted, the
	// datastore is available to all workspaces with read_write access.
	Access *DatastoreAccess `yaml:"access,omitempty" json:"access,omitempty"`
}

// DatastoreConnection holds the connection parameters.
type DatastoreConnection struct {
	Host     string            `yaml:"host,omitempty" json:"host,omitempty"`
	Port     int               `yaml:"port,omitempty" json:"port,omitempty"`
	Database string            `yaml:"database,omitempty" json:"database,omitempty"`
	Pool     *DatastorePool    `yaml:"pool,omitempty" json:"pool,omitempty"`
	Lazy     bool              `yaml:"lazy,omitempty" json:"lazy,omitempty"` // connect on first use
	Extra    map[string]string `yaml:"extra,omitempty" json:"extra,omitempty"`
}

// DatastorePool configures connection pooling.
type DatastorePool struct {
	MaxOpen     int    `yaml:"max_open,omitempty" json:"max_open,omitempty"`
	MaxIdle     int    `yaml:"max_idle,omitempty" json:"max_idle,omitempty"`
	MaxLifetime string `yaml:"max_lifetime,omitempty" json:"max_lifetime,omitempty"` // e.g. "1h", "30m"
}

// DatastoreAccess controls who can use this datastore and what they can do.
type DatastoreAccess struct {
	// Filter determines which workspaces can access this datastore.
	// If nil or all fields empty, the datastore is available to all workspaces.
	Filter *DatastoreAccessFilter `yaml:"filter,omitempty" json:"filter,omitempty"`

	// Permission is the ceiling for operations on this datastore.
	// If nil, defaults to read_write for all scopes.
	Permission *DatastorePermission `yaml:"permission,omitempty" json:"permission,omitempty"`
}

// DatastoreAccessFilter restricts datastore access to specific workspaces.
// All fields are optional and combined with AND logic.
type DatastoreAccessFilter struct {
	// Environment restricts to workspaces in the named environment (e.g. "production").
	Environment string `yaml:"environment,omitempty" json:"environment,omitempty"`

	// Workspaces restricts to a specific list of workspace IDs.
	Workspaces []string `yaml:"workspaces,omitempty" json:"workspaces,omitempty"`

	// Labels restricts to workspaces with all the given labels.
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// DatastorePermission defines the operation ceiling for this datastore.
type DatastorePermission struct {
	// Default is the baseline access level. Default: read_write.
	Default AccessPermission `yaml:"default,omitempty" json:"default,omitempty"`

	// Rules provide granular overrides for specific scopes.
	// Scope uses glob patterns matching module.table format.
	// The most specific matching rule wins.
	Rules []DatastorePermissionRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// DatastorePermissionRule overrides the default permission for a scope.
type DatastorePermissionRule struct {
	// Scope is a glob pattern, e.g. "store.*", "billing.invoice", "*.*".
	Scope string `yaml:"scope" json:"scope"`

	// Access is the permission level for this scope.
	Access AccessPermission `yaml:"access" json:"access"`
}

// DefaultDatastorePermission returns a default read_write permission spec.
func DefaultDatastorePermission() *DatastorePermission {
	return &DatastorePermission{
		Default: AccessReadWrite,
	}
}

// IsValidPrimitiveType returns true if p is a known primitive type.
func IsValidPrimitiveType(p PrimitiveType) bool {
	switch p {
	case PrimitiveDB, PrimitiveCache, PrimitiveLock, PrimitiveQueue,
		PrimitivePubSub, PrimitiveStorage, PrimitiveConfig, PrimitiveKVStore, PrimitiveLog:
		return true
	default:
		return false
	}
}

// DriverServers returns which primitive types a driver is compatible with.
func (d DatastoreDriver) Serves() []PrimitiveType {
	switch d {
	case DatastoreDriverSQLite, DatastoreDriverPostgres:
		return []PrimitiveType{PrimitiveDB, PrimitiveKVStore}
	case DatastoreDriverValkey, DatastoreDriverRedis:
		return []PrimitiveType{PrimitiveCache, PrimitiveLock, PrimitiveKVStore, PrimitiveQueue, PrimitivePubSub}
	case DatastoreDriverS3, DatastoreDriverMinio:
		return []PrimitiveType{PrimitiveStorage}
	case DatastoreDriverNATS:
		return []PrimitiveType{PrimitiveQueue, PrimitivePubSub}
	case DatastoreDriverMemory:
		return []PrimitiveType{PrimitiveCache, PrimitiveLock, PrimitiveQueue, PrimitivePubSub, PrimitiveKVStore}
	case DatastoreDriverFS:
		return []PrimitiveType{PrimitiveStorage}
	default:
		return nil
	}
}
