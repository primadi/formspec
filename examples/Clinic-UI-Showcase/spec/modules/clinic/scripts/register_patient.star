# modules/clinic/scripts/register_patient.star
# Commit action wizard patient-registration: terima akumulasi state wizard,
# buat pasien bila belum ada, lalu buat kunjungan — atomik.

def execute(resource, params, ctx):
    patient_id = params.patient_id
    if patient_id == None:
        new_patient = patient.create({
            "nik": params.nik,
            "name": params.name,
            "birth_date": params.birth_date,
            "gender": params.gender,
            "phone": params.phone,
        })
        patient_id = new_patient.id

    resource.set("patient_id", patient_id)
    resource.set("polyclinic_id", params.polyclinic_id)
    resource.set("doctor_id", params.doctor_id)
    resource.set("complaint", params.complaint)
    resource.set("transaction_date", ctx.today())
    resource.save()

    ctx.log.info("visit.registered", {
        "visit_id": resource.id,
        "patient_id": patient_id,
    })
    return ok({"queue_number": resource.field.queue_number})
