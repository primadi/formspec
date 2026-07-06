# modules/billing/scripts/midtrans_create_session.star
#
# PERHATIAN: Script ini adalah STUB — tidak bisa dijalankan apa adanya.
# Core Basic tidak menyediakan HTTP client di sandbox Starlark (no network).
# Panggilan HTTP ke Midtrans API HARUS melalui impl: native.
#
# Lihat impl/billing/midtrans.go untuk implementasi sebenarnya.
# Script ini didokumentasikan untuk menunjukkan kontrak action (params & response shape).

def execute(resource, params, ctx):
    # HANYA dipanggil saat mock_enabled: false (framework routing)
    # Implementasi sebenarnya di impl/billing/midtrans.go → PaymentGateway.CreateSession
    pass
