package formspec

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/api"
)

// `docs/spec/backend/05-field-types.md` §5.3 is explicit that a field-level
// `required_permission` guards BOTH directions:
//
//	"Pemanggil boleh diizinkan memanggil `update`/`read`, tapi tetap tidak boleh
//	 melihat atau menyetel field sensitif ini tanpa permission tambahan. Tanpa
//	 permission: field di-strip dari respons (read) dan penyetelannya di payload
//	 DITOLAK (write)."
//
// The read half is implemented (`internal/api/fieldsec.go`, pinned by
// TestAuthAuthz_E2E step 5). This pins the write half.
//
// Why it matters beyond tidiness: a field a caller cannot READ but CAN WRITE is
// the worse of the two failures. The read guard keeps a secret out of sight; a
// missing write guard lets the same caller SET it and then lose the ability to
// see what they set — and for a field like `salary` the whole point of the
// declaration is that only an authorized role decides its value.
func TestFieldPermission_WriteIsGuarded(t *testing.T) {
	dir := t.TempDir()
	buildAuthSpecDir(t, dir)
	api.ResetAuthRateLimiters()

	app, err := New(Config{
		SpecPath:  dir,
		DSN:       "sqlite:" + filepath.Join(t.TempDir(), "field_write.db"),
		ProdMode:  true,
		JWTSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// `limited` deliberately holds list+view only — NOT acme.customers.salary.view.
	seedUser(t, app, "admin", "admin", []string{"*"})
	seedUser(t, app, "limited", "limited", []string{
		"acme.customers.list", "acme.customers.view", "acme.customers.create", "acme.customers.update",
	})

	adminTok := login(t, app, "admin", "admin")
	limitedTok := login(t, app, "limited", "limited")

	t.Run("create", func(t *testing.T) {
		status, out := doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/customer", limitedTok, map[string]any{
			"name": "Bob", "salary": 999999,
		})
		if status != http.StatusCreated {
			// A rejected create is also an acceptable enforcement shape (the
			// request as a whole is refused), as long as it is a 4xx and not a
			// silent accept.
			if status < 400 || status >= 500 {
				t.Fatalf("create with a forbidden field: expected 2xx-with-strip or 4xx, got %d (%v)", status, out)
			}
			return
		}
		data, _ := out["data"].(map[string]any)
		if v, ok := data["salary"]; ok && v != nil {
			t.Errorf("the create RESPONSE echoed salary (%v) to a caller without the permission", v)
		}

		// Read it back as ADMIN — the value that actually landed in storage is
		// the question, and `limited` cannot see the field to answer it.
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("no id in create response: %v", out)
		}
		stored := readSalary(t, app, adminTok, id)
		if stored == 999999 {
			t.Errorf("a caller without acme.customers.salary.view SET salary to %v; "+
				"spec §5.3 requires the write to be refused", stored)
		}
	})

	t.Run("update", func(t *testing.T) {
		// Admin creates with an authorized value…
		status, out := doAuthed(t, app, http.MethodPost, "/default/_ui/entity/acme/customer", adminTok, map[string]any{
			"name": "Carol", "salary": 100000,
		})
		if status != http.StatusCreated {
			t.Fatalf("admin create: %d (%v)", status, out)
		}
		id, _ := out["data"].(map[string]any)["id"].(string)

		// …then `limited` tries to change it.
		//
		// `If-Match` is REQUIRED: ProdMode is strict about optimistic locking and
		// a header-less PATCH answers 409 before any field logic runs. Without
		// the header this subtest passed for the wrong reason — the write was
		// blocked by the concurrency guard, not by §5.3.
		version := readVersion(t, app, adminTok, id)
		status, out = doAuthedWithHeaders(t, app, http.MethodPatch,
			"/default/_ui/entity/acme/customer/"+id, limitedTok,
			map[string]any{"salary": 1},
			map[string]string{"If-Match": fmt.Sprintf("version=%d", version)})
		if status >= 500 {
			t.Fatalf("update with a forbidden field must not 500, got %d (%v)", status, out)
		}
		if status == http.StatusConflict {
			t.Fatalf("the update never ran (409) — the assertion below would be vacuous: %v", out)
		}

		stored := readSalary(t, app, adminTok, id)
		if stored == 1 {
			t.Errorf("a caller without acme.customers.salary.view CHANGED salary to 1; "+
				"spec §5.3 requires the write to be refused (stored %v)", stored)
		}
		if stored != 100000 {
			t.Errorf("salary = %v, want the admin's 100000 left untouched", stored)
		}
	})
}

// readSalary reads a customer's salary as an admin (who holds the field
// permission). Fails the test when the field is absent — an absent field cannot
// distinguish "the write was refused" from "the field was stripped".
func readSalary(t *testing.T, app *App, adminTok, id string) float64 {
	t.Helper()
	status, out := doAuthed(t, app, http.MethodGet, "/default/_ui/entity/acme/customer/"+id, adminTok, nil)
	if status != http.StatusOK {
		t.Fatalf("admin read: %d (%v)", status, out)
	}
	data, _ := out["data"].(map[string]any)
	v, ok := data["salary"]
	if !ok {
		t.Fatalf("admin must see salary to judge the write; fields present: %v", keysOf(data))
	}
	f, _ := v.(float64)
	return f
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// readVersion reads a record's optimistic-locking version as an admin.
func readVersion(t *testing.T, app *App, adminTok, id string) int {
	t.Helper()
	status, out := doAuthed(t, app, http.MethodGet, "/default/_ui/entity/acme/customer/"+id, adminTok, nil)
	if status != http.StatusOK {
		t.Fatalf("admin read for version: %d (%v)", status, out)
	}
	v, _ := out["data"].(map[string]any)["version"].(float64)
	return int(v)
}
