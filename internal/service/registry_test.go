package service

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func TestRegistry_AddGet(t *testing.T) {
	reg := NewRegistry()
	svc := &spec.ServiceSpec{
		Version: "v1",
		Actions: []spec.Action{
			{Name: "calculate", RequiredPermission: "billing.tax-calculator.calculate"},
		},
	}
	reg.Add("billing", "tax-calculator", svc)

	got, ok := reg.Get("billing", "tax-calculator")
	if !ok {
		t.Fatal("expected service to be registered")
	}
	if got != svc {
		t.Errorf("Get returned different spec")
	}

	if _, ok := reg.Get("billing", "missing"); ok {
		t.Errorf("expected missing service to not be found")
	}
}

func TestRegistry_GetAction(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "tax-calculator", &spec.ServiceSpec{
		Actions: []spec.Action{
			{Name: "calculate"},
			{Name: "validate"},
		},
	})

	a, ok := reg.GetAction("billing", "tax-calculator", "calculate")
	if !ok || a.Name != "calculate" {
		t.Errorf("expected calculate action, got %v ok=%v", a, ok)
	}

	if _, ok := reg.GetAction("billing", "tax-calculator", "missing"); ok {
		t.Errorf("expected missing action to not be found")
	}

	if _, ok := reg.GetAction("billing", "missing", "calculate"); ok {
		t.Errorf("expected missing service action to not be found")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "tax-calculator", &spec.ServiceSpec{
		Actions: []spec.Action{{Name: "calculate"}},
	})
	reg.Add("notify", "sms", &spec.ServiceSpec{
		Actions: []spec.Action{{Name: "send"}},
	})

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 services, got %d", len(list))
	}
	// Sorted by module then name.
	if list[0].Module != "billing" || list[0].Name != "tax-calculator" {
		t.Errorf("first service = %+v, want billing/tax-calculator", list[0])
	}
	if list[1].Module != "notify" || list[1].Name != "sms" {
		t.Errorf("second service = %+v, want notify/sms", list[1])
	}
	if len(list[0].Actions) != 1 || list[0].Actions[0] != "calculate" {
		t.Errorf("actions = %v, want [calculate]", list[0].Actions)
	}
}
