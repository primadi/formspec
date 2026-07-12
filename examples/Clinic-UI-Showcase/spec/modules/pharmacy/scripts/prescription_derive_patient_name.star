# modules/pharmacy/scripts/prescription_derive_patient_name.star
# hooks: before/create on prescription — if patient_id is set, cross-module
# fetch (clinic.patient, different module than this script's own pharmacy)
# and auto-fill patient_name so the doctor/staff doesn't retype it. Leaves
# patient_name untouched for external prescriptions with no patient_id.

def execute(resource, params, ctx):
    patient_id = resource.field.patient_id
    if patient_id != None and patient_id != "":
        patient = resource.fetch("clinic.patient", patient_id)
        resource.set("patient_name", patient.field.name)
    return ok()
