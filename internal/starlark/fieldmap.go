package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
)

// FieldMap is a Starlark value that wraps a Go map and supports BOTH
// attribute access (resource.amount) and dict access (resource["amount"]).
//
// It's used for guard/when expressions that reference `resource.<field>`
// (e.g. workflow `when: "resource.amount > 100000000"`, state-machine guards)
// where the plain starlark.Dict produced by toStarlark would reject dot
// notation.
type FieldMap struct {
	data map[string]any
}

// NewFieldMap wraps a Go map in a FieldMap.
func NewFieldMap(data map[string]any) *FieldMap {
	return &FieldMap{data: data}
}

var _ starlark.Value = (*FieldMap)(nil)
var _ starlark.HasAttrs = (*FieldMap)(nil)
var _ starlark.Mapping = (*FieldMap)(nil)

func (m *FieldMap) String() string        { return "<resource>" }
func (m *FieldMap) Type() string          { return "resource" }
func (m *FieldMap) Freeze()               {}
func (m *FieldMap) Truth() starlark.Bool  { return starlark.True }
func (m *FieldMap) Hash() (uint32, error) { return 0, fmt.Errorf("resource is not hashable") }

// Attr returns a field value via dot notation (resource.amount).
func (m *FieldMap) Attr(name string) (starlark.Value, error) {
	val, ok := m.data[name]
	if !ok {
		return starlark.None, nil // field not set → None
	}
	return toStarlark(val)
}

// AttrNames lists the field names.
func (m *FieldMap) AttrNames() []string {
	names := make([]string, 0, len(m.data))
	for k := range m.data {
		names = append(names, k)
	}
	return names
}

// Get returns a field value via dict access (resource["amount"]).
func (m *FieldMap) Get(k starlark.Value) (starlark.Value, bool, error) {
	key, ok := starlark.AsString(k)
	if !ok {
		return nil, false, fmt.Errorf("resource key must be a string")
	}
	val, ok := m.data[key]
	if !ok {
		return starlark.None, true, nil
	}
	sv, err := toStarlark(val)
	if err != nil {
		return nil, false, err
	}
	return sv, true, nil
}
