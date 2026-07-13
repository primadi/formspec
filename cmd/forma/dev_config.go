// ─── Config file loader for `forma dev` / `forma serve` ───
//
// Loads configuration from forma-app.yaml (or legacy forma-sidecar.yaml),
// then merges with CLI flags (CLI wins).
package main

import (
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// configFile represents the YAML structure of forma-app.yaml.
// Fields are pointers so we can detect which ones were explicitly set.
type configFile struct {
	Spec             *string  `yaml:"spec"`
	DSN              *string  `yaml:"dsn"`
	Addr             *string  `yaml:"addr"`
	Listen           *string  `yaml:"listen"`
	AppEndpoint      *string  `yaml:"app-endpoint"`
	ListenURL        *string  `yaml:"listen-url"`
	AppEndpointURL   *string  `yaml:"app-endpoint-url"`
	WorkspaceID      *string  `yaml:"workspace-id"`
	Runtime          *string  `yaml:"runtime"`
	StateDir         *string  `yaml:"state-dir"`
	Dev              *bool    `yaml:"dev"`
	DevUI            *bool    `yaml:"dev-ui"`
	Force            *bool    `yaml:"force"`
	WebDir           *string  `yaml:"web-dir"`
	InvokeTimeoutStr *string  `yaml:"invoke-timeout"`
	AppDir           *string  `yaml:"app-dir"`
	AppEntrypoint    *string  `yaml:"app-entrypoint"`
	ControlURL       *string  `yaml:"control-cluster-url"`
	WorkspaceIDAlt   *string  `yaml:"workspace-id-alt"` // unused reserved
	Themes           []string `yaml:"themes"`           // additional theme manifest directories
}

// mergeConfigFile tries to read forma-app.yaml and merge values into cfg.
// Config file values only apply when the CLI flag was left at its default.
// Returns the potentially-modified config.
func mergeConfigFile(cfg DevConfig) DevConfig {
	path := findConfigFile()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[forma] warning: cannot read config %s: %v", path, err)
		return cfg
	}

	var cf configFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		log.Printf("[forma] warning: cannot parse config %s: %v", path, err)
		return cfg
	}

	log.Printf("[forma] using config: %s", path)

	// Apply config file values (only if CLI didn't override)
	if cf.Spec != nil && cfg.SpecPath == defaultSpecPath {
		cfg.SpecPath = *cf.Spec
	}
	if cf.DSN != nil && cfg.DSN == defaultDSN {
		cfg.DSN = *cf.DSN
	}
	if cf.Addr != nil && cfg.Addr == defaultAddr {
		cfg.Addr = *cf.Addr
	}
	if cf.Listen != nil && cfg.Listen == defaultListen {
		cfg.Listen = *cf.Listen
	}
	if cf.AppEndpoint != nil && cfg.AppEndpoint == defaultAppEndpoint {
		cfg.AppEndpoint = *cf.AppEndpoint
	}
	if cf.ListenURL != nil && cfg.ListenURL == "" {
		cfg.ListenURL = *cf.ListenURL
	}
	if cf.AppEndpointURL != nil && cfg.AppEndpointURL == "" {
		cfg.AppEndpointURL = *cf.AppEndpointURL
	}
	if cf.WorkspaceID != nil && cfg.WorkspaceID == defaultWorkspaceID {
		cfg.WorkspaceID = *cf.WorkspaceID
	}
	if cf.Runtime != nil && cfg.Runtime == defaultRuntime {
		cfg.Runtime = *cf.Runtime
	}
	if cf.StateDir != nil && cfg.StateDir == defaultStateDir {
		cfg.StateDir = *cf.StateDir
	}
	if cf.Dev != nil && !cfg.DevMode && !cfg.DevUI {
		cfg.DevMode = *cf.Dev
	}
	if cf.DevUI != nil && !cfg.DevUI {
		cfg.DevUI = *cf.DevUI
		if cfg.DevUI {
			cfg.DevMode = true
		}
	}
	if cf.Force != nil && !cfg.Force {
		cfg.Force = *cf.Force
	}
	if cf.WebDir != nil && cfg.WebDir == "" {
		cfg.WebDir = *cf.WebDir
	}
	if cf.InvokeTimeoutStr != nil && cfg.InvokeTimeout == 30*time.Second {
		if d, err := time.ParseDuration(*cf.InvokeTimeoutStr); err == nil {
			cfg.InvokeTimeout = d
		}
	}
	if cf.AppDir != nil && cfg.AppDir == "" {
		cfg.AppDir = *cf.AppDir
	}
	if cf.AppEntrypoint != nil && cfg.AppEntrypoint == "" {
		cfg.AppEntrypoint = *cf.AppEntrypoint
	}
	if cf.ControlURL != nil && cfg.ControlURL == "" {
		cfg.ControlURL = *cf.ControlURL
	}
	// Merge theme dirs from config file (append if CLI didn't set any)
	if len(cf.Themes) > 0 && len(cfg.ThemeDirs) == 0 {
		cfg.ThemeDirs = cf.Themes
	}

	return cfg
}

// findConfigFile looks for forma-app.yaml in CWD, then falls back to
// the legacy name forma-sidecar.yaml for backward compatibility.
func findConfigFile() string {
	candidates := []string{
		"forma-app.yaml",
		"forma-app.yml",
		"forma-sidecar.yaml", // legacy, backward compat
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
