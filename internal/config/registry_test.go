package config

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestRegistry_NonSecretAndSecrets(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", &spec.ConfigSpec{
		Keys: map[string]spec.ConfigKey{
			"invoice_due_days": {Type: "int", Default: 30},
			"smtp_host":        {Type: "string", Default: "smtp.example.com", Secret: true},
			"debug":            {Type: "bool", Default: false},
			"tax_rate":         {Type: "decimal", Default: 0.11},
		},
	})

	nonSecret := reg.NonSecret()
	if got := nonSecret["invoice_due_days"]; got != int64(30) {
		t.Errorf("invoice_due_days = %v (%T), want int64(30)", got, got)
	}
	if got := nonSecret["debug"]; got != false {
		t.Errorf("debug = %v, want false", got)
	}
	if got := nonSecret["tax_rate"]; got != 0.11 {
		t.Errorf("tax_rate = %v, want 0.11", got)
	}
	if _, ok := nonSecret["smtp_host"]; ok {
		t.Errorf("secret key smtp_host leaked into NonSecret()")
	}

	secrets := reg.Secrets()
	if got := secrets["smtp_host"]; got != "smtp.example.com" {
		t.Errorf("smtp_host = %q, want smtp.example.com", got)
	}
	if _, ok := secrets["invoice_due_days"]; ok {
		t.Errorf("non-secret key invoice_due_days leaked into Secrets()")
	}
}

func TestRegistry_StringIntCoercion(t *testing.T) {
	reg := NewRegistry()
	reg.Add("app", &spec.ConfigSpec{
		Keys: map[string]spec.ConfigKey{
			"port": {Type: "int", Default: "8080"},
		},
	})
	got := reg.NonSecret()["port"]
	if got != int64(8080) {
		t.Errorf("port = %v (%T), want int64(8080)", got, got)
	}
}

func TestRegistry_Empty(t *testing.T) {
	reg := NewRegistry()
	if len(reg.NonSecret()) != 0 {
		t.Errorf("NonSecret() on empty registry should be empty")
	}
	if len(reg.Secrets()) != 0 {
		t.Errorf("Secrets() on empty registry should be empty")
	}
}
