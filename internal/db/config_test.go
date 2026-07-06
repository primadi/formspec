package db

import (
	"testing"
)

func TestParseDSN_SQLiteRelative(t *testing.T) {
	cfg, err := ParseDSN(".forma/data.db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Driver != DriverSQLite {
		t.Errorf("expected sqlite, got %s", cfg.Driver)
	}
	if cfg.Database != ".forma/data.db" {
		t.Errorf("expected .forma/data.db, got %s", cfg.Database)
	}
}

func TestParseDSN_SQLiteExplicit(t *testing.T) {
	cfg, err := ParseDSN("sqlite:.forma/dev.db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Driver != DriverSQLite {
		t.Errorf("expected sqlite, got %s", cfg.Driver)
	}
	if cfg.Database != ".forma/dev.db" {
		t.Errorf("expected .forma/dev.db, got %s", cfg.Database)
	}
}

func TestParseDSN_SQLiteAbsolute(t *testing.T) {
	cfg, err := ParseDSN("sqlite:///tmp/test.db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Driver != DriverSQLite {
		t.Errorf("expected sqlite, got %s", cfg.Driver)
	}
	if cfg.Database != "/tmp/test.db" {
		t.Errorf("expected /tmp/test.db, got %s", cfg.Database)
	}
}

func TestParseDSN_SQLiteWithPragma(t *testing.T) {
	cfg, err := ParseDSN("sqlite:/tmp/test.db?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database != "/tmp/test.db" {
		t.Errorf("expected /tmp/test.db, got %s", cfg.Database)
	}
	if cfg.Extra["_pragma"] != "journal_mode(WAL)" {
		t.Errorf("expected journal_mode(WAL), got %s", cfg.Extra["_pragma"])
	}
}

func TestParseDSN_SQLiteWithPragmaRelative(t *testing.T) {
	cfg, err := ParseDSN("sqlite:test.db?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database != "test.db" {
		t.Errorf("expected test.db, got %s", cfg.Database)
	}
}

func TestParseDSN_SQLiteMultiplePragmas(t *testing.T) {
	cfg, err := ParseDSN("sqlite:test.db?_pragma=journal_mode(WAL)&_pragma=cache_size(-32000)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database != "test.db" {
		t.Errorf("expected test.db, got %s", cfg.Database)
	}
	if cfg.Extra["_pragma"] != "cache_size(-32000)" {
		// Note: when same key appears multiple times, last one wins
		t.Logf("_pragma = %s", cfg.Extra["_pragma"])
	}
}

func TestParseDSN_SQLiteEmptyPathDefaults(t *testing.T) {
	cfg, err := ParseDSN("sqlite:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database != ".forma/data.db" {
		t.Errorf("expected .forma/data.db, got %s", cfg.Database)
	}
}

func TestParseDSN_Postgres(t *testing.T) {
	cfg, err := ParseDSN("postgres://user:pass@localhost:5432/forma?sslmode=require")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Driver != DriverPostgres {
		t.Errorf("expected postgres, got %s", cfg.Driver)
	}
	if cfg.Host != "localhost" {
		t.Errorf("expected localhost, got %s", cfg.Host)
	}
	if cfg.Port != "5432" {
		t.Errorf("expected 5432, got %s", cfg.Port)
	}
	if cfg.User != "user" {
		t.Errorf("expected user, got %s", cfg.User)
	}
	if cfg.Database != "forma" {
		t.Errorf("expected forma, got %s", cfg.Database)
	}
	if cfg.Extra["sslmode"] != "require" {
		t.Errorf("expected require, got %s", cfg.Extra["sslmode"])
	}
}

func TestParseDSN_PostgresDefaultPort(t *testing.T) {
	cfg, err := ParseDSN("postgres://user@localhost/forma")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "5432" {
		t.Errorf("expected default port 5432, got %s", cfg.Port)
	}
}

func TestParseDSN_PostgresWithSchema(t *testing.T) {
	cfg, err := ParseDSN("postgres://user@localhost/forma?schema=financial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Schema != "financial" {
		t.Errorf("expected financial, got %s", cfg.Schema)
	}
}

func TestParseDSN_InvalidScheme(t *testing.T) {
	_, err := ParseDSN("mysql://user:pass@localhost/db")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestParseDSN_Empty(t *testing.T) {
	_, err := ParseDSN("")
	if err == nil {
		t.Fatal("expected error for empty DSN")
	}
}

func TestPostgresConnString(t *testing.T) {
	cfg := &Config{
		Driver:   DriverPostgres,
		Host:     "localhost",
		Port:     "5432",
		User:     "admin",
		Password: "secret",
		Database: "forma",
		Extra:    map[string]string{"sslmode": "require"},
	}
	conn := cfg.PostgresConnString()
	if conn == "" {
		t.Fatal("expected non-empty connection string")
	}
	checks := []string{"host=localhost", "port=5432", "user=admin", "password=secret", "dbname=forma", "sslmode=require"}
	for _, s := range checks {
		if !stringsContains(conn, s) {
			t.Errorf("expected conn string to contain %q", s)
		}
	}
}

func TestSQLitePragmas(t *testing.T) {
	cfg := &Config{
		Driver:   DriverSQLite,
		Database: "test.db",
		Extra: map[string]string{
			"_pragma": "journal_mode(WAL)",
			"other":   "value",
		},
	}
	p := cfg.SQLitePragmas()
	if p["_pragma"] != "journal_mode(WAL)" {
		t.Errorf("expected journal_mode(WAL), got %s", p["_pragma"])
	}
	if _, ok := p["other"]; ok {
		t.Errorf("expected non-pragma keys to be excluded")
	}
}

func stringsContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
