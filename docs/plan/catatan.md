**forma-sidecar**

*cmd/forma-sidecar/main.go* — embed engine via resource.New (registry, schema sync, REST API generator — sama persis dengan app Go native). Dua mode sumber manifest: --spec (dev, langsung dari disk) atau --control-cluster-url (pull artifact via deployer internal/resource, YAML dimaterialisasi ke --state-dir/spec; artifact baru setelah boot ditulis ke disk dan di-log restart required).
*internal/action/sidecar.go* — stub diganti implementasi nyata (§4.2): POST /invoke/{module}/{entity}/{action} over unix socket atau localhost TCP, marshal langsung ExecuteParams/ExecuteResult, timeout configurable dengan pesan gateway-timeout. Executor tanpa endpoint tetap gagal dengan error "not configured", jadi perilaku app Go native tidak berubah — diaktifkan lewat field baru resource.Config.SidecarEndpoint.
*internal/sidecar/* — arah App→Sidecar: POST /ctx/{prim}/{op} (§4.3) memakai kontrak resolver yang identik dengan internal/starlark (func(primitiveType, name) (any, error)), dispatch via capability interface (Querier/KVGetter/Locker/...). Plus GET /health agregasi: ping app tiap 10 detik, 3 kegagalan berturut-turut → degraded (§7).

Catatan penting: backend primitive ctx.* memang masih stub di seluruh codebase (starlark pun begitu), jadi operasi ke datastore yang belum punya implementasi op balas 501 dengan pesan jelas — plumbing dan wire format-nya sudah jadi, tinggal backend-nya.

**forma-operator**

*internal/operator/api/v1alpha1/* — CRD types Workspace, Datastore, ResourceClaim, ClusterClass (group forma.dev), deepcopy ditulis tangan karena repo tidak memakai controller-gen.
*workspace_controller.go* — Deployment (image generic via --resource-image, env CONTROL_CLUSTER_URL/WORKSPACE_ID, nodeSelector forma.dev/*, anti-affinity saat minReplicas >= 2) + Service + ConfigMap + HPA (feature auto-scaling), injeksi kredensial dari claim ber-status Ready sebagai secretKeyRef (operator tidak pernah membaca nilai kredensial), status Ready/Progressing/Degraded. Delete lewat owner references; Secret sengaja tidak di-own sehingga retained sesuai §2.3. ClusterClass tidak punya reconciler sendiri — perubahannya di-fan-out ke Workspace via watch (cache-only, §2.1).
*resourceclaim_controller.go* — verifikasi ed25519 atas pesan kanonik datastore|workspace|permission|grantedBy|grantedAt terhadap spec.ownerPublicKey di Datastore, plus cek allowedTenants → Ready/Denied. Ada --insecure-skip-signature-verify untuk dev.
*reporter.go* — POST /v1/node-health (15s) dan /v1/workspace-status (on-change) ke Cluster Control; karena forma-ctl belum serve endpoint ini (§3.2 masih "diusulkan"), kegagalan di-log rate-limited tanpa memblokir reconcile.
*deploy/operator/* — 4 CRD manifest YAML, RBAC persis §3.4 (+ nodes read untuk reporting), dan Deployment 2-replica leader-election.

**Verifikasi & keputusan yang perlu Anda tahu**

Test baru: round-trip protokol invoke over unix socket (termasuk error app dan timeout), dispatch proxy ctx.*, dan reconcile ResourceClaim dengan signature asli/tampered/tenant ditolak (fake client). Semua lulus bersama suite lama.
Doc menyebut "satu socket untuk dua arah", tapi HTTP butuh listener per arah — saya pakai dua endpoint: --listen (sidecar dengar, ctx calls) dan --app-endpoint (app dengar, invoke). Kunci publik pemilik resource saya letakkan di Datastore.spec.ownerPublicKey karena docs belum menentukan sumbernya.
Kedua bagian "Status Implementasi Hari Ini" di docs/runtimes sudah saya perbarui, termasuk daftar gap yang tersisa (lib-forma-* belum ada, poller restart_required belum men-stamp annotation, mode child-process §5 belum dibuat).
Satu hal untuk dipertimbangkan: docs menyebut operator seharusnya closed source di repo terpisah (D-ARCH-15). Sesuai permintaan Anda saya taruh di cmd/ repo ini — kalau nanti mau dipisah, seluruh kodenya terisolasi di cmd/forma-operator + internal/operator + deploy/operator sehingga mudah diekstrak.