package spec

import "testing"

func TestValidateWorkspaceSpec(t *testing.T) {
	tests := []struct {
		name    string
		ws      WorkspaceSpec
		resName string
		wantErr bool
	}{
		{name: "plain slug", ws: WorkspaceSpec{}, resName: "cafe", wantErr: false},
		{name: "hyphenated slug", ws: WorkspaceSpec{}, resName: "kopi-kita", wantErr: false},
		{name: "digits slug", ws: WorkspaceSpec{}, resName: "branch-2", wantErr: false},
		{name: "explicit slug wins", ws: WorkspaceSpec{Slug: "kopi"}, resName: "cafe", wantErr: false},
		{name: "uppercase", ws: WorkspaceSpec{}, resName: "Cafe", wantErr: true},
		{name: "spaces", ws: WorkspaceSpec{}, resName: "kopi kita", wantErr: true},
		{name: "underscore", ws: WorkspaceSpec{}, resName: "kopi_kita", wantErr: true},
		{name: "leading hyphen", ws: WorkspaceSpec{}, resName: "-kopi", wantErr: true},
		{name: "trailing hyphen", ws: WorkspaceSpec{}, resName: "kopi-", wantErr: true},
		{name: "reserved ui", ws: WorkspaceSpec{}, resName: "_ui", wantErr: true},
		{name: "reserved api", ws: WorkspaceSpec{}, resName: "api", wantErr: true},
		{name: "reserved admin", ws: WorkspaceSpec{}, resName: "_admin", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspaceSpec(&tt.ws, tt.resName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateWorkspaceSpec(%q) error = %v, wantErr %v", tt.resName, err, tt.wantErr)
			}
		})
	}
}

func TestMountsWithin(t *testing.T) {
	all := &AppSpec{} // field absent
	if !all.MountsWithin("anything") {
		t.Fatal("absent allowlist must mount in all workspaces")
	}
	empty := []string{}
	staged := &AppSpec{Workspaces: &empty}
	if staged.MountsWithin("cafe") {
		t.Fatal("explicitly empty allowlist (staged) must mount nowhere")
	}
	listed := []string{"cafe", "kopi"}
	scoped := &AppSpec{Workspaces: &listed}
	if !scoped.MountsWithin("cafe") || !scoped.MountsWithin("kopi") {
		t.Fatal("listed workspaces must be allowed")
	}
	if scoped.MountsWithin("default") {
		t.Fatal("unlisted workspaces must be rejected")
	}
}

func TestValidateAppSpec_Workspaces(t *testing.T) {
	empty := []string{}
	scoped := []string{"cafe", "kopi-kita"}
	reserved := []string{"api"}
	bad := []string{"Bad Slug"}

	tests := []struct {
		name    string
		ws      *[]string
		wantErr bool
	}{
		{name: "absent = all", ws: nil, wantErr: false},
		{name: "staged = mounted nowhere", ws: &empty, wantErr: false},
		{name: "valid slugs", ws: &scoped, wantErr: false},
		{name: "reserved segment", ws: &reserved, wantErr: true},
		{name: "invalid slug", ws: &bad, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAppSpec(&AppSpec{Workspaces: tt.ws})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateAppSpec error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAppSpec_Version(t *testing.T) {
	for _, v := range []string{"", "1.0.0", "1.2.3-beta.1", "2.0.0+build.5"} {
		if err := ValidateAppSpec(&AppSpec{Version: v}); err != nil {
			t.Errorf("version %q: unexpected error %v", v, err)
		}
	}
	for _, v := range []string{"1", "1.0", "v1.0.0", "abc"} {
		if err := ValidateAppSpec(&AppSpec{Version: v}); err == nil {
			t.Errorf("version %q: expected error, got nil", v)
		}
	}
}

func TestEffectiveSlug(t *testing.T) {
	ws := &WorkspaceSpec{Slug: "kopi"}
	if got := ws.EffectiveSlug("cafe"); got != "kopi" {
		t.Fatalf("EffectiveSlug = %q, want %q", got, "kopi")
	}
	ws2 := &WorkspaceSpec{}
	if got := ws2.EffectiveSlug("cafe"); got != "cafe" {
		t.Fatalf("EffectiveSlug fallback = %q, want %q", got, "cafe")
	}
}
