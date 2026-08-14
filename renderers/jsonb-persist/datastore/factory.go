package datastore

import (
	"fmt"

	"github.com/primadi/formspec/pkg/spec"
)

// ConnectionFactory creates connection pools for a specific driver.
type ConnectionFactory interface {
	// Open creates a new connection pool from the datastore spec.
	Open(spec spec.DatastoreSpec) (*ConnectionPool, error)
}

// NewFactory returns a ConnectionFactory for the given driver.
// Returns an error if the driver is not supported.
func NewFactory(driver spec.DatastoreDriver) (ConnectionFactory, error) {
	switch driver {
	case spec.DatastoreDriverSQLite:
		return &sqliteFactory{}, nil
	case spec.DatastoreDriverPostgres:
		return &postgresFactory{}, nil
	case spec.DatastoreDriverValkey:
		return &valkeyFactory{}, nil
	case spec.DatastoreDriverRedis:
		return &redisFactory{}, nil
	case spec.DatastoreDriverS3:
		return &s3Factory{}, nil
	case spec.DatastoreDriverMinio:
		return &minioFactory{}, nil
	case spec.DatastoreDriverNATS:
		return &natsFactory{}, nil
	case spec.DatastoreDriverMemory:
		return &memoryFactory{}, nil
	case spec.DatastoreDriverFS:
		return &fsFactory{}, nil
	default:
		return nil, fmt.Errorf("datastore factory: unsupported driver %q", driver)
	}
}

// sqliteFactory creates SQLite connection pools.
type sqliteFactory struct{}

func (f *sqliteFactory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	// TODO: integrate with renderers/jsonb-persist package for SQLite connections
	return NewConnectionPool(ConnectionConfig{
		Driver:  "sqlite",
		DSN:     ds.Connection.Database,
		MaxOpen: getMaxOpen(ds.Connection.Pool),
		MaxIdle: getMaxIdle(ds.Connection.Pool),
	}), nil
}

// postgresFactory creates PostgreSQL connection pools.
type postgresFactory struct{}

func (f *postgresFactory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	conn := ds.Connection
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s sslmode=require",
		conn.Host, conn.Port, conn.Database)
	if conn.Extra != nil {
		for k, v := range conn.Extra {
			dsn += fmt.Sprintf(" %s=%s", k, v)
		}
	}
	return NewConnectionPool(ConnectionConfig{
		Driver:  "postgres",
		DSN:     dsn,
		MaxOpen: getMaxOpen(conn.Pool),
		MaxIdle: getMaxIdle(conn.Pool),
	}), nil
}

// valkeyFactory creates Valkey connection pools.
type valkeyFactory struct{}

func (f *valkeyFactory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	return NewConnectionPool(ConnectionConfig{
		Driver:  "valkey",
		DSN:     fmt.Sprintf("%s:%d", ds.Connection.Host, ds.Connection.Port),
		MaxOpen: getMaxOpen(ds.Connection.Pool),
		MaxIdle: getMaxIdle(ds.Connection.Pool),
	}), nil
}

// redisFactory creates Redis connection pools.
type redisFactory struct{}

func (f *redisFactory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	return NewConnectionPool(ConnectionConfig{
		Driver:  "redis",
		DSN:     fmt.Sprintf("%s:%d", ds.Connection.Host, ds.Connection.Port),
		MaxOpen: getMaxOpen(ds.Connection.Pool),
		MaxIdle: getMaxIdle(ds.Connection.Pool),
	}), nil
}

// s3Factory creates S3 connection pools.
type s3Factory struct{}

func (f *s3Factory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	return NewConnectionPool(ConnectionConfig{
		Driver:  "s3",
		DSN:     fmt.Sprintf("s3://%s", ds.Connection.Database),
		MaxOpen: getMaxOpen(ds.Connection.Pool),
		MaxIdle: getMaxIdle(ds.Connection.Pool),
	}), nil
}

// minioFactory creates MinIO connection pools.
type minioFactory struct{}

func (f *minioFactory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	return NewConnectionPool(ConnectionConfig{
		Driver:  "minio",
		DSN:     fmt.Sprintf("%s:%d/%s", ds.Connection.Host, ds.Connection.Port, ds.Connection.Database),
		MaxOpen: getMaxOpen(ds.Connection.Pool),
		MaxIdle: getMaxIdle(ds.Connection.Pool),
	}), nil
}

// natsFactory creates NATS connection pools.
type natsFactory struct{}

func (f *natsFactory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	return NewConnectionPool(ConnectionConfig{
		Driver:  "nats",
		DSN:     fmt.Sprintf("nats://%s:%d", ds.Connection.Host, ds.Connection.Port),
		MaxOpen: getMaxOpen(ds.Connection.Pool),
		MaxIdle: getMaxIdle(ds.Connection.Pool),
	}), nil
}

// memoryFactory creates in-memory connection pools (dev mode).
type memoryFactory struct{}

func (f *memoryFactory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	return NewConnectionPool(ConnectionConfig{
		Driver:  "memory",
		DSN:     "memory://",
		MaxOpen: 1,
		MaxIdle: 1,
	}), nil
}

// fsFactory creates filesystem connection pools (dev mode).
type fsFactory struct{}

func (f *fsFactory) Open(ds spec.DatastoreSpec) (*ConnectionPool, error) {
	return NewConnectionPool(ConnectionConfig{
		Driver:  "fs",
		DSN:     ds.Connection.Database,
		MaxOpen: 1,
		MaxIdle: 1,
	}), nil
}

func getMaxOpen(pool *spec.DatastorePool) int {
	if pool == nil || pool.MaxOpen == 0 {
		return 10
	}
	return pool.MaxOpen
}

func getMaxIdle(pool *spec.DatastorePool) int {
	if pool == nil || pool.MaxIdle == 0 {
		return 5
	}
	return pool.MaxIdle
}
