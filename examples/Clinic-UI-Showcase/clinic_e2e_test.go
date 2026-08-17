// Package clinic_test exercises examples/Clinic-UI-Showcase end-to-end,
// through the real HTTP handler down to a real (temp-file) SQLite database —
// proving the action-scripting engine (Starlark script_ref actions, natural
// keys, CAS) actually works for this manifest, not just that it parses.
package clinic_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	formspec "github.com/primadi/formspec/resource"
)

// recentDate returns a transaction_date within the default backdate window
// (3 days) so tests don't fail as wall-clock time advances past a hardcoded
// date. The visit entity has an override_permission, but otc-sale and
// prescription use the default 3-day limit.
func recentDate() string {
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

func newTestApp(t *testing.T) *formspec.App {
	t.Helper()
	dsn := "sqlite:" + filepath.Join(t.TempDir(), "clinic_e2e.db")
	app, err := formspec.New(formspec.Config{
		SpecPath: "./spec",
		DSN:      dsn,
	})
	if err != nil {
		t.Fatalf("formspec.New: %v", err)
	}
	return app
}

// envelope mirrors internal/api.SingleResponse loosely enough for tests —
// Data is decoded generically since its shape varies per endpoint.
type envelope struct {
	Data any `json:"data"`
	Meta any `json:"meta"`
	responseErr
}

type responseErr struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// do performs an HTTP request against the app's handler and decodes the
// envelope. It fails the test on transport/JSON errors, not on non-2xx
// status — callers assert status explicitly.
func do(t *testing.T, handler http.Handler, method, path string, body any) (int, envelope) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var env envelope
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode response body (status %d): %v\nbody: %s", rec.Code, err, rec.Body.String())
		}
	}
	return rec.Code, env
}

func (e envelope) String() string {
	if e.Error != nil {
		return fmt.Sprintf("error=%s: %s", e.Error.Code, e.Error.Message)
	}
	return fmt.Sprintf("data=%+v", e.Data)
}

func dataMap(t *testing.T, env envelope) map[string]any {
	t.Helper()
	m, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be a JSON object, got %T: %+v", env.Data, env.Data)
	}
	return m
}

// createFixtures sets up a polyclinic, doctor, and patient shared by the
// lifecycle tests below.
func createFixtures(t *testing.T, handler http.Handler) (polyclinicID, doctorID, patientID string) {
	t.Helper()

	status, env := do(t, handler, "POST", "/demo/_ui/entity/clinic/polyclinic", map[string]any{
		"name": "Poli Umum",
		"code": "UMUM",
	})
	if status != http.StatusCreated {
		t.Fatalf("create polyclinic: status %d, body %+v", status, env)
	}
	polyclinicID = dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/clinic/doctor", map[string]any{
		"name":             "Dr. Andi",
		"polyclinic_id":    polyclinicID,
		"license_number":   "LIC-0001",
		"consultation_fee": 50000,
	})
	if status != http.StatusCreated {
		t.Fatalf("create doctor: status %d, body %+v", status, env)
	}
	doctorID = dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/clinic/patient", map[string]any{
		"nik":        "1234567890123456",
		"name":       "Jane Doe",
		"birth_date": "1990-01-01",
		"gender":     "female",
	})
	if status != http.StatusCreated {
		t.Fatalf("create patient: status %d, body %+v", status, env)
	}
	patientID = dataMap(t, env)["id"].(string)

	return polyclinicID, doctorID, patientID
}

func TestVisitLifecycle_EndToEnd(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	polyclinicID, doctorID, patientID := createFixtures(t, handler)

	// Create the visit directly — the patient-registration wizard creates
	// the patient eagerly in its own step (a plain patient.create,
	// independent of wizard completion), so by the time the visit itself is
	// created every required field (patient_id, polyclinic_id, doctor_id,
	// complaint, transaction_date) is already known; no separate commit
	// action is needed. queue_number is generated automatically here
	// (natural_key_rule).
	status, env := do(t, handler, "POST", "/demo/_ui/entity/clinic/visit", map[string]any{
		"transaction_date": recentDate(),
		"patient_id":       patientID,
		"polyclinic_id":    polyclinicID,
		"doctor_id":        doctorID,
		"complaint":        "Demam dan batuk",
	})
	if status != http.StatusCreated {
		t.Fatalf("create visit: status %d, body %+v", status, env)
	}
	visit := dataMap(t, env)
	visitID := visit["id"].(string)

	queueNumber, _ := visit["queue_number"].(string)
	if matched, _ := regexp.MatchString(`^Q\d{8}-\d{3}$`, queueNumber); !matched {
		t.Errorf("queue_number %q does not match QYYYYMMDD-NNN format", queueNumber)
	}
	versionAfterCreate := int(visit["version"].(float64))

	// start-consultation: first status transition + first save on an
	// already-existing record — this is where the CAS bug (hardcoded
	// Version: 0) used to fail outright.
	status, env = do(t, handler, "POST", "/demo/_ui/entity/clinic/visit/"+visitID+"/start-consultation", nil)
	if status != http.StatusOK {
		t.Fatalf("start-consultation: status %d, body %+v", status, env)
	}

	status, env = do(t, handler, "GET", "/demo/_ui/entity/clinic/visit/"+visitID, nil)
	if status != http.StatusOK {
		t.Fatalf("get visit after start-consultation: status %d, body %+v", status, env)
	}
	afterStart := dataMap(t, env)
	if afterStart["status"] != "in_consultation" {
		t.Fatalf("expected status in_consultation, got %v", afterStart["status"])
	}
	versionAfterStart := int(afterStart["version"].(float64))
	if versionAfterStart <= versionAfterCreate {
		t.Errorf("expected version to increase after start-consultation: %d -> %d", versionAfterCreate, versionAfterStart)
	}

	// Add diagnosis + treatments via the standard update action (allowed:
	// visit.update's condition permits status in [waiting, in_consultation]).
	status, env = do(t, handler, "PATCH", "/demo/_ui/entity/clinic/visit/"+visitID, map[string]any{
		"diagnosis": "ISPA ringan",
		"treatments": []map[string]any{
			{"line_number": 1, "treatment_name": "Paracetamol", "quantity": 2, "price": 5000},
			{"line_number": 2, "treatment_name": "Vitamin C", "quantity": 1, "price": 3000},
		},
	})
	if status != http.StatusOK {
		t.Fatalf("update visit with diagnosis+treatments: status %d, body %+v", status, env)
	}

	// complete: second sequential script-driven save on the same record —
	// the actual CAS regression check — and it must compute total from the
	// child rows using t["quantity"]/t["price"] (Dict indexing, not dot access).
	status, env = do(t, handler, "POST", "/demo/_ui/entity/clinic/visit/"+visitID+"/complete", nil)
	if status != http.StatusOK {
		t.Fatalf("complete: status %d, body %+v", status, env)
	}

	status, env = do(t, handler, "GET", "/demo/_ui/entity/clinic/visit/"+visitID, nil)
	if status != http.StatusOK {
		t.Fatalf("get visit after complete: status %d, body %+v", status, env)
	}
	final := dataMap(t, env)
	if final["status"] != "completed" {
		t.Fatalf("expected status completed, got %v", final["status"])
	}
	wantTotal := float64(2*5000 + 1*3000)
	if got := final["total"]; got != wantTotal {
		t.Errorf("expected total %v, got %v", wantTotal, got)
	}
	versionAfterComplete := int(final["version"].(float64))
	if versionAfterComplete <= versionAfterStart {
		t.Errorf("expected version to increase after complete: %d -> %d", versionAfterStart, versionAfterComplete)
	}
}

// TestVisitComplete_RejectsMissingDiagnosis proves the state-machine guard
// (in_consultation -> completed requires non-empty diagnosis) is actually
// enforced — this only works once the script engine bugs are fixed, since
// the guard runs inline in db.EntityStore.Update on every resource.save().
func TestVisitComplete_RejectsMissingDiagnosis(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	polyclinicID, doctorID, patientID := createFixtures(t, handler)

	status, env := do(t, handler, "POST", "/demo/_ui/entity/clinic/visit", map[string]any{
		"transaction_date": recentDate(),
		"patient_id":       patientID,
		"polyclinic_id":    polyclinicID,
		"doctor_id":        doctorID,
		"complaint":        "Sakit kepala",
	})
	if status != http.StatusCreated {
		t.Fatalf("create visit: status %d, body %+v", status, env)
	}
	visitID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/clinic/visit/"+visitID+"/start-consultation", nil)
	if status != http.StatusOK {
		t.Fatalf("start-consultation: status %d, body %+v", status, env)
	}

	status, _ = do(t, handler, "POST", "/demo/_ui/entity/clinic/visit/"+visitID+"/complete", nil)
	if status == http.StatusOK {
		t.Fatal("expected complete to fail without diagnosis, but it succeeded")
	}
}

func TestPrescriptionLifecycle_EndToEnd(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	polyclinicID, doctorID, patientID := createFixtures(t, handler)

	status, env := do(t, handler, "POST", "/demo/_ui/entity/clinic/visit", map[string]any{
		"transaction_date": recentDate(),
		"patient_id":       patientID,
		"polyclinic_id":    polyclinicID,
		"doctor_id":        doctorID,
		"complaint":        "Butuh resep",
	})
	if status != http.StatusCreated {
		t.Fatalf("create visit: status %d, body %+v", status, env)
	}
	visitID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/pharmacy/medicine", map[string]any{
		"sku":   "SKU-001",
		"name":  "Paracetamol 500mg",
		"unit":  "tablet",
		"stock": 100,
		"price": 1000,
	})
	if status != http.StatusCreated {
		t.Fatalf("create medicine: status %d, body %+v", status, env)
	}
	medicineID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/_ui/entity/pharmacy/prescription", map[string]any{
		"transaction_date": recentDate(),
		"visit_id":         visitID,
		"patient_name":     "Jane Doe",
		"items": []map[string]any{
			{"line_number": 1, "medicine_id": medicineID, "quantity": 5, "dosage_instructions": "3x1 sehari"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create prescription: status %d, body %+v", status, env)
	}
	prescription := dataMap(t, env)
	prescriptionID := prescription["id"].(string)

	number, _ := prescription["number"].(string)
	if matched, _ := regexp.MatchString(`^RX-\d{8}-\d{3}$`, number); !matched {
		t.Errorf("prescription number %q does not match RX-YYYYMMDD-NNN format", number)
	}

	for _, action := range []string{"start-compounding", "mark-ready", "dispense"} {
		status, env = do(t, handler, "POST", fmt.Sprintf("/demo/_ui/entity/pharmacy/prescription/%s/%s", prescriptionID, action), nil)
		if status != http.StatusOK {
			t.Fatalf("%s: status %d, body %+v", action, status, env)
		}
	}

	status, env = do(t, handler, "GET", "/demo/_ui/entity/pharmacy/prescription/"+prescriptionID, nil)
	if status != http.StatusOK {
		t.Fatalf("get prescription: status %d, body %+v", status, env)
	}
	if got := dataMap(t, env)["status"]; got != "dispensed" {
		t.Fatalf("expected status dispensed, got %v", got)
	}

	// dispense's resource.load("medicine", ...) + med.save() must have
	// actually decremented stock — this is the loaded-resource CAS/save
	// propagation fix (a resource.load() result must be independently
	// saveable, not just readable).
	status, env = do(t, handler, "GET", "/demo/_ui/entity/pharmacy/medicine/"+medicineID, nil)
	if status != http.StatusOK {
		t.Fatalf("get medicine: status %d, body %+v", status, env)
	}
	if got := dataMap(t, env)["stock"]; got != float64(95) {
		t.Errorf("expected medicine stock 95 after dispense, got %v", got)
	}
}
