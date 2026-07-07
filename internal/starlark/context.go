// Package starlark — Script Context API
//
// This file provides the ctx.* object exposed to Starlark scripts.
// Scripts access runtime primitives via:
//
//	ctx.tenant.id
//	ctx.user.id
//	ctx.now()
//	ctx.log.info("event", {"key": "val"})
//	ctx.next_key("field_name")
package starlark

import (
	"fmt"
	"time"

	"go.starlark.net/starlark"
)

// CtxAPI is the Starlark-callable ctx object.
// It provides access to tenant info, user info, logging, and primitives.
type CtxAPI struct {
	Tenant  *tenantInfo
	User    *userInfo
	Auth    *authInfo
	Now     func() time.Time
	Log     *logAPI
	NextKey func(fieldName string) (string, error)
	Config  *configAPI
}

var _ starlark.Value = (*CtxAPI)(nil)

// NewCtxAPI creates a ctx object with the given tenant and user info.
func NewCtxAPI(tenantID, tenantName, userID, userRole string, userPerms []string) *CtxAPI {
	return &CtxAPI{
		Tenant: &tenantInfo{ID: tenantID, Name: tenantName},
		User:   &userInfo{ID: userID, Role: userRole, Permissions: userPerms},
		Auth:   &authInfo{},
		Now:    now,
		Log:    newLogAPI(),
	}
}

// ─── starlark.Value interface ───

func (c *CtxAPI) String() string        { return "<ctx>" }
func (c *CtxAPI) Type() string          { return "ctx" }
func (c *CtxAPI) Freeze()               {}
func (c *CtxAPI) Truth() starlark.Bool  { return starlark.True }
func (c *CtxAPI) Hash() (uint32, error) { return 0, fmt.Errorf("ctx is not hashable") }

// Attr returns ctx attributes: .tenant, .user, .auth, .now, .log, .next_key, .config
func (c *CtxAPI) Attr(name string) (starlark.Value, error) {
	switch name {
	case "tenant":
		return c.Tenant, nil
	case "user":
		return c.User, nil
	case "auth":
		return c.Auth, nil
	case "now":
		return c.builtinNow(), nil
	case "log":
		return c.Log, nil
	case "next_key":
		return c.builtinNextKey(), nil
	case "config":
		return c.Config, nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("ctx has no .%s attribute", name))
	}
}

func (c *CtxAPI) AttrNames() []string {
	return []string{"tenant", "user", "auth", "now", "log", "next_key", "config"}
}

func (c *CtxAPI) builtinNow() *starlark.Builtin {
	return starlark.NewBuiltin("ctx.now", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		if c.Now == nil {
			return starlark.String(time.Now().UTC().Format(time.RFC3339)), nil
		}
		return starlark.String(c.Now().UTC().Format(time.RFC3339)), nil
	})
}

func (c *CtxAPI) builtinNextKey() *starlark.Builtin {
	return starlark.NewBuiltin("ctx.next_key", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var fieldName string
		if err := starlark.UnpackArgs("next_key", args, kwargs, "field", &fieldName); err != nil {
			return nil, err
		}

		if c.NextKey == nil {
			return nil, fmt.Errorf("ctx.next_key: no key generator registered")
		}

		key, err := c.NextKey(fieldName)
		if err != nil {
			return nil, fmt.Errorf("ctx.next_key: %w", err)
		}
		return starlark.String(key), nil
	})
}

// ─── Sub-types ───

type tenantInfo struct {
	ID   string
	Name string
}

var _ starlark.Value = (*tenantInfo)(nil)

func (t *tenantInfo) String() string        { return fmt.Sprintf("<tenant %s>", t.ID) }
func (t *tenantInfo) Type() string          { return "tenant" }
func (t *tenantInfo) Freeze()               {}
func (t *tenantInfo) Truth() starlark.Bool  { return starlark.True }
func (t *tenantInfo) Hash() (uint32, error) { return 0, fmt.Errorf("tenant is not hashable") }

func (t *tenantInfo) Attr(name string) (starlark.Value, error) {
	switch name {
	case "id":
		return starlark.String(t.ID), nil
	case "name":
		return starlark.String(t.Name), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("tenant has no .%s", name))
	}
}

func (t *tenantInfo) AttrNames() []string { return []string{"id", "name"} }

type userInfo struct {
	ID          string
	Role        string
	Permissions []string
}

var _ starlark.Value = (*userInfo)(nil)

func (u *userInfo) String() string        { return fmt.Sprintf("<user %s>", u.ID) }
func (u *userInfo) Type() string          { return "user" }
func (u *userInfo) Freeze()               {}
func (u *userInfo) Truth() starlark.Bool  { return starlark.True }
func (u *userInfo) Hash() (uint32, error) { return 0, fmt.Errorf("user is not hashable") }

func (u *userInfo) Attr(name string) (starlark.Value, error) {
	switch name {
	case "id":
		return starlark.String(u.ID), nil
	case "role":
		return starlark.String(u.Role), nil
	case "permissions":
		perms := make([]starlark.Value, len(u.Permissions))
		for i, p := range u.Permissions {
			perms[i] = starlark.String(p)
		}
		return starlark.NewList(perms), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("user has no .%s", name))
	}
}

func (u *userInfo) AttrNames() []string { return []string{"id", "role", "permissions"} }

type authInfo struct{}

var _ starlark.Value = (*authInfo)(nil)

func (a *authInfo) String() string        { return "<auth>" }
func (a *authInfo) Type() string          { return "auth" }
func (a *authInfo) Freeze()               {}
func (a *authInfo) Truth() starlark.Bool  { return starlark.True }
func (a *authInfo) Hash() (uint32, error) { return 0, fmt.Errorf("auth is not hashable") }

func (a *authInfo) Attr(name string) (starlark.Value, error) {
	if name == "has" {
		return starlark.NewBuiltin("auth.has", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			return starlark.True, nil // TODO: permission check via ctx.auth.has(perm)
		}), nil
	}
	return nil, starlark.NoSuchAttrError(fmt.Sprintf("auth has no .%s", name))
}

func (a *authInfo) AttrNames() []string { return []string{"has"} }

// ─── ctx.log ───

type logAPI struct {
	entries []LogEntry
}

// LogEntry is a recorded log message from a script execution.
type LogEntry struct {
	Level string
	Event string
	Meta  map[string]any
}

func newLogAPI() *logAPI {
	return &logAPI{}
}

var _ starlark.Value = (*logAPI)(nil)

func (l *logAPI) String() string        { return "<log>" }
func (l *logAPI) Type() string          { return "log" }
func (l *logAPI) Freeze()               {}
func (l *logAPI) Truth() starlark.Bool  { return starlark.True }
func (l *logAPI) Hash() (uint32, error) { return 0, fmt.Errorf("log is not hashable") }

func (l *logAPI) Attr(name string) (starlark.Value, error) {
	switch name {
	case "info":
		return l.builtinLog("info"), nil
	case "warn":
		return l.builtinLog("warn"), nil
	case "error":
		return l.builtinLog("error"), nil
	default:
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("log has no .%s", name))
	}
}

func (l *logAPI) AttrNames() []string { return []string{"info", "warn", "error"} }

func (l *logAPI) builtinLog(level string) *starlark.Builtin {
	return starlark.NewBuiltin("log."+level, func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var event string
		var meta starlark.Value = starlark.None
		if err := starlark.UnpackArgs("log", args, kwargs,
			"event", &event,
			"meta?", &meta,
		); err != nil {
			return nil, err
		}

		l.entries = append(l.entries, LogEntry{
			Level: level,
			Event: event,
			Meta:  starlarkValueToMap(meta),
		})
		return starlark.None, nil
	})
}

// Entries returns all recorded log entries.
func (l *logAPI) Entries() []LogEntry { return l.entries }

// ─── ctx.config ───

type configAPI struct {
	store map[string]any
}

// NewConfigAPI creates a config context backed by the given key-value store.
func NewConfigAPI(store map[string]any) *configAPI {
	return &configAPI{store: store}
}

var _ starlark.Value = (*configAPI)(nil)

func (c *configAPI) String() string        { return "<config>" }
func (c *configAPI) Type() string          { return "config" }
func (c *configAPI) Freeze()               {}
func (c *configAPI) Truth() starlark.Bool  { return starlark.True }
func (c *configAPI) Hash() (uint32, error) { return 0, fmt.Errorf("config is not hashable") }

func (c *configAPI) Attr(name string) (starlark.Value, error) {
	if name == "get" {
		return c.builtinGet(), nil
	}
	return nil, starlark.NoSuchAttrError(fmt.Sprintf("config has no .%s", name))
}

func (c *configAPI) AttrNames() []string { return []string{"get"} }

func (c *configAPI) builtinGet() *starlark.Builtin {
	return starlark.NewBuiltin("config.get", func(
		thread *starlark.Thread,
		fn *starlark.Builtin,
		args starlark.Tuple,
		kwargs []starlark.Tuple,
	) (starlark.Value, error) {
		var key string
		var defaultVal starlark.Value = starlark.None
		if err := starlark.UnpackArgs("get", args, kwargs,
			"key", &key,
			"default?", &defaultVal,
		); err != nil {
			return nil, err
		}

		if c.store == nil {
			return defaultVal, nil
		}

		val, ok := c.store[key]
		if !ok {
			return defaultVal, nil
		}
		return toStarlark(val)
	})
}
