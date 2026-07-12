package clinic_test

import (
	"net/http"
	"testing"
)

// TestPrescriptionInternal_PatientIdDerivesPatientName proves the
// hooks: before/create script (prescription_derive_patient_name.star) — a
// cross-module resource.fetch from the pharmacy module into clinic.patient —
// actually runs and fills patient_name before the required-field check,
// even though patient_name is omitted from the request body entirely.
func TestPrescriptionInternal_PatientIdDerivesPatientName(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	polyclinicID, doctorID, patientID := createFixtures(t, handler)

	status, env := do(t, handler, "POST", "/demo/api/v1/clinic/visits", map[string]any{
		"transaction_date": "2026-07-12",
		"patient_id":       patientID,
		"polyclinic_id":    polyclinicID,
		"doctor_id":        doctorID,
		"complaint":        "Butuh resep",
	})
	if status != http.StatusCreated {
		t.Fatalf("create visit: status %d, body %+v", status, env)
	}
	visitID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/api/v1/pharmacy/medicines", map[string]any{
		"sku": "SKU-100", "name": "Amoxicillin 500mg", "unit": "tablet", "stock": 50, "price": 2000,
	})
	if status != http.StatusCreated {
		t.Fatalf("create medicine: status %d, body %+v", status, env)
	}
	medicineID := dataMap(t, env)["id"].(string)

	// Note: patient_name is deliberately omitted — required, but the
	// before/create hook must derive it from patient_id before the
	// required-field check runs, since it mutates the same data map Insert
	// validates against.
	status, env = do(t, handler, "POST", "/demo/api/v1/pharmacy/prescriptions", map[string]any{
		"transaction_date": "2026-07-12",
		"visit_id":         visitID,
		"patient_id":       patientID,
		"items": []map[string]any{
			{"line_number": 1, "medicine_id": medicineID, "quantity": 3, "dosage_instructions": "2x1 sehari"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create prescription: status %d, body %+v", status, env)
	}
	rx := dataMap(t, env)
	if got := rx["patient_name"]; got != "Jane Doe" {
		t.Errorf("patient_name = %v, want \"Jane Doe\" (derived from patient_id via before/create hook, not supplied in the request)", got)
	}
	if got := rx["source"]; got != "internal" {
		t.Errorf("source = %v, want \"internal\" (default)", got)
	}
}

// TestPrescriptionExternal_NoVisitId_Succeeds proves scenario (b) — a
// prescription written by an outside doctor, with no clinic visit at all —
// which previously 400'd because visit_id was required: true.
func TestPrescriptionExternal_NoVisitId_Succeeds(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	status, env := do(t, handler, "POST", "/demo/api/v1/pharmacy/medicines", map[string]any{
		"sku": "SKU-101", "name": "Ibuprofen 400mg", "unit": "tablet", "stock": 30, "price": 1500,
	})
	if status != http.StatusCreated {
		t.Fatalf("create medicine: status %d, body %+v", status, env)
	}
	medicineID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/api/v1/pharmacy/prescriptions", map[string]any{
		"transaction_date": "2026-07-12",
		"source":           "external",
		"prescriber_name":  "dr. Budi Santoso (RS Luar)",
		"patient_name":     "Walk-in Patient",
		"items": []map[string]any{
			{"line_number": 1, "medicine_id": medicineID, "quantity": 2, "dosage_instructions": "1x1 sehari"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create external prescription (no visit_id): status %d, body %+v", status, env)
	}
	rx := dataMap(t, env)
	if got := rx["visit_id"]; got != nil {
		t.Errorf("visit_id = %v, want nil/absent for an external prescription", got)
	}
	if got := rx["prescriber_name"]; got != "dr. Budi Santoso (RS Luar)" {
		t.Errorf("prescriber_name = %v, want the submitted value unchanged", got)
	}
}

// TestPrescriptionCreate_VisitIDOptional_Regression is an explicit guard for
// the schema relaxation itself: omitting visit_id (source left at its
// "internal" default, no patient_id either) must still succeed — the field
// is optional now, full stop, independent of the source enum's value.
func TestPrescriptionCreate_VisitIDOptional_Regression(t *testing.T) {
	app := newTestApp(t)
	handler := app.Handler()

	status, env := do(t, handler, "POST", "/demo/api/v1/pharmacy/medicines", map[string]any{
		"sku": "SKU-102", "name": "Cetirizine 10mg", "unit": "tablet", "stock": 20, "price": 2500,
	})
	if status != http.StatusCreated {
		t.Fatalf("create medicine: status %d, body %+v", status, env)
	}
	medicineID := dataMap(t, env)["id"].(string)

	status, env = do(t, handler, "POST", "/demo/api/v1/pharmacy/prescriptions", map[string]any{
		"transaction_date": "2026-07-12",
		"patient_name":     "No Visit Patient",
		"items": []map[string]any{
			{"line_number": 1, "medicine_id": medicineID, "quantity": 1, "dosage_instructions": "1x1 sehari"},
		},
	})
	if status != http.StatusCreated {
		t.Fatalf("create prescription without visit_id: status %d, body %+v", status, env)
	}
}
