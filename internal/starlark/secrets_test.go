package starlark

import (
	"testing"

	"go.starlark.net/starlark"
)

func TestSecretsAPI_GetDeclared(t *testing.T) {
	var audited []string
	s := NewSecretsAPI(
		map[string]string{"api_key": "secret123"},
		[]string{"api_key"},
		func(k string) { audited = append(audited, k) },
	)

	v, err := s.builtinGet().CallInternal(&starlark.Thread{}, starlark.Tuple{starlark.String("api_key")}, nil)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(v.(starlark.String)) != "secret123" {
		t.Errorf("expected secret123, got %s", v.String())
	}
	if len(audited) != 1 || audited[0] != "api_key" {
		t.Errorf("expected audit of api_key, got %v", audited)
	}
}

func TestSecretsAPI_UndeclaredBlocked(t *testing.T) {
	s := NewSecretsAPI(map[string]string{"api_key": "secret123"}, []string{"other"}, nil)
	_, err := s.builtinGet().CallInternal(&starlark.Thread{}, starlark.Tuple{starlark.String("api_key")}, nil)
	if err == nil {
		t.Fatal("expected error for undeclared secret (uses.secrets enforcement)")
	}
}

func TestSecretsAPI_MissingKeyReturnsNone(t *testing.T) {
	s := NewSecretsAPI(map[string]string{}, []string{"api_key"}, nil)
	v, err := s.builtinGet().CallInternal(&starlark.Thread{}, starlark.Tuple{starlark.String("api_key")}, nil)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if v != starlark.None {
		t.Errorf("expected None for missing key, got %v", v)
	}
}
