package formspec

import "testing"

// TestCheckCrossModuleUses exercises the uses.resources enforcement helper
// (todo 2.6.4): same-module and unqualified access is always allowed;
// cross-module access requires a declaration in uses.resources.
func TestCheckCrossModuleUses(t *testing.T) {
	tests := []struct {
		name          string
		fromModule    string
		targetModule  string
		targetEntity  string
		declared      []string
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:         "same-module always allowed",
			fromModule:   "pharmacy",
			targetModule: "pharmacy",
			targetEntity: "medicine",
			declared:     nil,
		},
		{
			name:         "unqualified target module allowed",
			fromModule:   "pharmacy",
			targetModule: "",
			targetEntity: "medicine",
			declared:     nil,
		},
		{
			name:          "cross-module undeclared blocked",
			fromModule:    "pharmacy",
			targetModule:  "clinic",
			targetEntity:  "patient",
			declared:      nil,
			wantErr:       true,
			wantErrSubstr: "USES_VIOLATION: undeclared cross-module access to clinic.patient from module pharmacy",
		},
		{
			name:         "cross-module exact dot declaration allowed",
			fromModule:   "pharmacy",
			targetModule: "clinic",
			targetEntity: "patient",
			declared:     []string{"clinic.patient"},
		},
		{
			name:         "cross-module exact slash declaration allowed",
			fromModule:   "pharmacy",
			targetModule: "clinic",
			targetEntity: "patient",
			declared:     []string{"clinic/patient"},
		},
		{
			name:         "cross-module module wildcard allowed",
			fromModule:   "pharmacy",
			targetModule: "clinic",
			targetEntity: "patient",
			declared:     []string{"clinic.*"},
		},
		{
			name:         "cross-module global wildcard allowed",
			fromModule:   "pharmacy",
			targetModule: "clinic",
			targetEntity: "patient",
			declared:     []string{"*"},
		},
		{
			name:         "unrelated declaration still blocks",
			fromModule:   "pharmacy",
			targetModule: "clinic",
			targetEntity: "patient",
			declared:     []string{"billing.invoice"},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkCrossModuleUses(tt.fromModule, tt.targetModule, tt.targetEntity, tt.declared)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrSubstr != "" {
					got := err.Error()
					if len(got) < len(tt.wantErrSubstr) || got[:len(tt.wantErrSubstr)] != tt.wantErrSubstr {
						t.Fatalf("expected error starting with %q, got %q", tt.wantErrSubstr, got)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
