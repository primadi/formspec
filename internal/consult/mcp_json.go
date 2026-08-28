package consult

import (
	"encoding/json"
)

// marshalJSONSchema serializes the MCP SDK's schema type to raw JSON.
func marshalJSONSchema(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}
