package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupSettingsSeedEnv builds a registry with a reference `app-setting`
// entity (natural key "name") and a factory wired with a resolved settings
// namespace, mirroring the Configuration Page pattern (spec §10).
func setupSettingsSeedEnv(t *testing.T) (*entity.Registry, db.DB, *HandlerFactory) {
	t.Helper()

	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "settings_seed.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)

	entitySpec := spec.EntitySpec{
		Version:        "v1",
		Plural:         "app-settings",
		Characteristic: spec.CharReference,
		Fields: []spec.Field{
			{Name: "name", Type: spec.FieldString, NaturalKey: true},
			{Name: "currency_code", Type: spec.FieldString},
			{Name: "currency_decimal_places", Type: spec.FieldInteger},
			{Name: "currency_symbol", Type: spec.FieldString},
			{Name: "locale", Type: spec.FieldString},
			{Name: "timezone", Type: spec.FieldString},
			{Name: "date_format", Type: spec.FieldString},
			{Name: "decimal_scale", Type: spec.FieldInteger},
			{Name: "rounding", Type: spec.FieldString},
		},
	}
	registerTestEntity(t, d, reg, "formspec.core", "app-setting", entitySpec)

	factory := NewHandlerFactory(reg)
	factory.SetSpecLookup(func(module, name string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})

	zero := 0
	factory.SetSettings(&spec.Settings{
		Currency: &spec.CurrencySettings{
			Code:          "IDR",
			DecimalPlaces: &zero,
			Symbol:        "Rp",
		},
		Locale:       "id-ID",
		Timezone:     "Asia/Jakarta",
		DateFormat:   "DD/MM/YYYY",
		DecimalScale: 2,
		Rounding:     "half_even",
	})

	return reg, d, factory
}

// TestHandleFind_SeedsAppSettingFromSettings verifies that a find-or-create
// on the `app-setting` reference entity seeds the record with the resolved
// settings namespace, so the Configuration Page form shows defaults instead
// of empty fields on first access.
func TestHandleFind_SeedsAppSettingFromSettings(t *testing.T) {
	_, _, factory := setupSettingsSeedEnv(t)

	handler := factory.HandleFind("formspec.core", "app-setting")
	req := httptest.NewRequest("GET", "/formspec.core/app-settings/global", nil)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp SingleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data map, got %T", resp.Data)
	}

	checks := map[string]any{
		"currency_code":           "IDR",
		"currency_decimal_places": float64(0),
		"currency_symbol":         "Rp",
		"locale":                  "id-ID",
		"timezone":                "Asia/Jakarta",
		"date_format":             "DD/MM/YYYY",
		"decimal_scale":           float64(2),
		"rounding":                "half_even",
	}
	for field, want := range checks {
		if got, ok := data[field]; !ok || got != want {
			t.Errorf("field %q: expected %v, got %v (present=%v)", field, want, got, ok)
		}
	}
}

// TestHandleFind_SeedsOnlyKeyForOtherReference verifies that find-or-create
// on a non-app-setting reference entity seeds ONLY the natural key — the
// settings seeding is scoped to `formspec.core/app-setting`.
func TestHandleFind_SeedsOnlyKeyForOtherReference(t *testing.T) {
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "settings_seed_other.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	entitySpec := spec.EntitySpec{
		Version:        "v1",
		Plural:         "provinces",
		Characteristic: spec.CharReference,
		Fields: []spec.Field{
			{Name: "code", Type: spec.FieldString, NaturalKey: true},
			{Name: "name", Type: spec.FieldString},
		},
	}
	registerTestEntity(t, d, reg, "geo", "province", entitySpec)

	factory := NewHandlerFactory(reg)
	factory.SetSpecLookup(func(module, name string) (*spec.EntitySpec, bool) {
		info, ok := reg.GetEntity(module, name)
		if !ok || info.EntitySpec == nil {
			return nil, false
		}
		return info.EntitySpec, true
	})
	factory.SetSettings(spec.ResolveSettings(&spec.Settings{Locale: "id-ID"}))

	handler := factory.HandleFind("geo", "province")
	req := httptest.NewRequest("GET", "/geo/provinces/JKT", nil)
	req = req.WithContext(WithWorkspace(req.Context(), "t1"))
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp SingleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data map, got %T", resp.Data)
	}
	if _, ok := data["locale"]; ok {
		t.Errorf("expected NO settings seeding for non-app-setting entity, got locale=%v", data["locale"])
	}
	if _, ok := data["currency_code"]; ok {
		t.Errorf("expected NO settings seeding for non-app-setting entity, got currency_code=%v", data["currency_code"])
	}
}
