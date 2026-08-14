package db

import (
	"fmt"
	"net/url"
	"strings"
)

// DriverType identifies the database driver.
type DriverType string

const (
	DriverSQLite   DriverType = "sqlite"
	DriverPostgres DriverType = "postgres"
)

// Config holds parsed database connection configuration.
type Config struct {
	Driver   DriverType
	DSN      string // Raw DSN string
	Database string
	Host     string
	Port     string
	User     string
	Password string
	Schema   string            // PostgreSQL schema (default: "public")
	Extra    map[string]string // Extra query parameters
}

// ParseDSN parses a FormSpec DSN string into a Config.
//
// Format:
//
//	sqlite:///absolute/path/to/data.db
//	sqlite:relative/path/data.db
//	sqlite:relative.db?_pragma=journal_mode(WAL)
//	postgres://user:pass@localhost:5432/formspec?sslmode=require
//	postgres://user@localhost/formspec?sslmode=require&schema=financial
//
// When no scheme is provided, defaults to sqlite.
func ParseDSN(dsn string) (*Config, error) {
	if dsn == "" {
		return nil, fmt.Errorf("db config: empty DSN")
	}

	cfg := &Config{
		DSN:   dsn,
		Extra: make(map[string]string),
	}

	// Detect scheme manually to handle file paths cleanly
	scheme, rest, hasScheme := strings.Cut(dsn, ":")

	switch {
	case !hasScheme:
		// No scheme — default to sqlite
		cfg.Driver = DriverSQLite
		cfg.Database = dsn

	case scheme == "sqlite":
		cfg.Driver = DriverSQLite
		// rest is in the form: [//]path[?query]
		// "sqlite:///absolute/path" → rest="//absolute/path" → "/absolute/path"
		// "sqlite:/absolute/path"   → rest="/absolute/path"  → "/absolute/path"
		// "sqlite:relative/path"    → rest="relative/path"   → "relative/path"
		path := rest
		if strings.HasPrefix(path, "//") {
			// Three-slash pattern: sqlite:///path → path = /path
			path = "/" + strings.TrimLeft(path, "/")
		}
		// If path starts with `/` it's already absolute — keep as-is
		// Split on ? for query params
		if idx := strings.IndexByte(path, '?'); idx >= 0 {
			cfg.Database = path[:idx]
			parseQuery(path[idx+1:], cfg.Extra)
		} else {
			cfg.Database = path
		}
		if cfg.Database == "" {
			cfg.Database = ".formspec/data.db"
		}

	case scheme == "postgres", scheme == "postgresql":
		cfg.Driver = DriverPostgres
		u, err := url.Parse(dsn)
		if err != nil {
			return nil, fmt.Errorf("db config: invalid postgres DSN %q: %w", dsn, err)
		}
		cfg.Host = u.Hostname()
		cfg.Port = u.Port()
		if cfg.Port == "" {
			cfg.Port = "5432"
		}
		cfg.User = u.User.Username()
		cfg.Password, _ = u.User.Password()
		cfg.Database = strings.TrimPrefix(u.Path, "/")
		cfg.Schema = "public"
		for k, v := range u.Query() {
			if k == "schema" {
				cfg.Schema = v[0]
			} else {
				cfg.Extra[k] = v[0]
			}
		}
		if cfg.Database == "" {
			return nil, fmt.Errorf("db config: missing database name in postgres DSN")
		}

	default:
		return nil, fmt.Errorf("db config: unsupported scheme %q (supported: sqlite, postgres)", scheme)
	}

	return cfg, nil
}

// parseQuery parses URL query parameters into a map.
func parseQuery(query string, target map[string]string) {
	for _, pair := range strings.Split(query, "&") {
		if k, v, ok := strings.Cut(pair, "="); ok {
			target[k] = v
		}
	}
}

// PostgresConnString returns a libpq-compatible connection string.
func (c *Config) PostgresConnString() string {
	var b strings.Builder
	b.WriteString("host=")
	b.WriteString(c.Host)
	if c.Port != "" {
		b.WriteString(" port=")
		b.WriteString(c.Port)
	}
	if c.User != "" {
		b.WriteString(" user=")
		b.WriteString(c.User)
	}
	if c.Password != "" {
		b.WriteString(" password=")
		b.WriteString(c.Password)
	}
	b.WriteString(" dbname=")
	b.WriteString(c.Database)
	for k, v := range c.Extra {
		b.WriteString(" ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
	}
	return b.String()
}

// SQLitePragmas extracts pragma parameters for SQLite.
func (c *Config) SQLitePragmas() map[string]string {
	p := make(map[string]string)
	for k, v := range c.Extra {
		if strings.HasPrefix(k, "_pragma") {
			p[k] = v
		}
	}
	return p
}
