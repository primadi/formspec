package formspec

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// ─── Otoritas nilai yang MEMPENGARUHI UANG pada `order` (kafe 10.81 ⏸️) ───
//
// Test di bawah menuliskan perilaku yang SEHARUSNYA, dan di-skip sementara
// karena perilaku hari ini justru sebaliknya. Angka-angka yang mengukurnya
// tercatat di item **10.81** (`examples/kafe/gaps_found/TODO.md`) dan changelog
// `2026-10-07-004`; hapus `t.Skip` di sini begitu item itu ditutup, dan test ini
// menjadi test regresi untuk perbaikannya.
//
// Kenapa dikumpulkan di satu file: keempatnya satu keluarga — nilai yang
// menentukan uang diterima apa adanya dari pemanggil, padahal bisa diturunkan
// atau dibatasi. `discount_amount`, `points_value`, dan `manual_discount_amount`
// masuk ke `total_amount`; `lines[].discount_amount` seharusnya masuk ke
// `line_total`.

const (
	itemDiscountAuthority = "tutup kafe 10.81 lalu hapus t.Skip ini"
	// Alasan yang sama untuk semuanya: `max_quantity`/`clampQuantity` di sisi
	// klien membuktikan polanya sudah dikenal, hanya belum ditegakkan server.
	itemWhySkipped = "kafe 10.81 ⏸️ — perilaku hari ini menerima nilai ini dari pemanggil"
)

// discountSession membuat sesi meja supaya `create_scope` (cabang dari meja)
// terpenuhi dan payload-nya sah dalam segala hal selain diskonnya.
func discountSession(t *testing.T, app *App, ids kafeSeedIDs, token string) string {
	t.Helper()
	return insertRecord(t, app, "cafe-order", "table-session", map[string]any{
		"transaction_date": time.Now().UTC().Format(time.RFC3339),
		"dining_table_id":  ids.tableID,
		"branch_id":        ids.branchID,
		"guest_token":      token,
	})
}

func moneyIDR(amount string) map[string]any {
	return map[string]any{"amount": amount, "currency": "IDR"}
}

// TestKafe_DiscountAmountIsDerivedNotAuthored — "Total Diskon" menurut
// `docs/domain-model.md` adalah "diskon promo + manual", yaitu TURUNAN. Hari ini
// ia field money biasa, jadi tamu anonim bisa mem-POST 40000 pada pesanan 45000:
//
//	create → 201; discount_amount=40000; total_amount=11975 (dari 51975)
func TestKafe_DiscountAmountIsDerivedNotAuthored(t *testing.T) {
	t.Skip(itemWhySkipped + " — " + itemDiscountAuthority)
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	sessionID := discountSession(t, app, ids, "DISC-AUTH-1")

	body := orderBody(ids, sessionID, "DISC-AUTH-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 1)
	body["discount_amount"] = moneyIDR("40000")

	// Diskon yang tidak berasal dari promo/manual yang sah harus DITOLAK, bukan
	// disimpan: `total_amount` dikurangkan olehnya, dan jurnal memakai nilai itu.
	status, out := doJSON(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", body)
	if status == http.StatusCreated {
		data, _ := out["data"].(map[string]any)
		t.Fatalf("diskon yang dikarang pemanggil tersimpan: discount_amount=%v total_amount=%v",
			numberOf(data["discount_amount"]), numberOf(data["total_amount"]))
	}
}

// TestKafe_PointsValueIsDerivedNotAuthored — `points_value` (nilai rupiah poin)
// juga turunan: `points_redeemed` × nilai poin. Hari ini pemanggil mengirim
// keduanya, jadi poin bisa dikarang:
//
//	create → 201; points_value=44000; total_amount=7975
func TestKafe_PointsValueIsDerivedNotAuthored(t *testing.T) {
	t.Skip(itemWhySkipped + " — " + itemDiscountAuthority)
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	sessionID := discountSession(t, app, ids, "PTS-AUTH-1")

	body := orderBody(ids, sessionID, "PTS-AUTH-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 1)
	body["points_redeemed"] = 999999
	body["points_value"] = moneyIDR("44000")

	status, out := doJSON(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", body)
	if status == http.StatusCreated {
		data, _ := out["data"].(map[string]any)
		msg := "nilai poin yang dikarang pemanggil tersimpan"
		if points, ok := data["points_redeemed"]; ok {
			t.Fatalf("%s: points_redeemed=%v points_value=%v total_amount=%v",
				msg, points, numberOf(data["points_value"]), numberOf(data["total_amount"]))
		}
		t.Fatalf("%s: points_value=%v total_amount=%v", msg,
			numberOf(data["points_value"]), numberOf(data["total_amount"]))
	}
}

// TestKafe_ManualDiscountRequiresAReason — `required_when` di ENTITY kini
// ditegakkan server (2026-10-07). Sebelumnya deklarasi itu hanya dipakai klien
// dan `formspec check`, sehingga pemanggil API langsung melewatinya:
//
//	create dengan manual_discount_amount → 201, reason=nil   (sebelum)
//	create dengan manual_discount_amount → 422               (sesudah)
//
// Diuji lewat permukaan HTTP dan ANONIM, karena itulah jalur yang tidak pernah
// melewati form.
func TestKafe_ManualDiscountRequiresAReason(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	sessionID := discountSession(t, app, ids, "MAN-REASON-1")

	body := orderBody(ids, sessionID, "MAN-REASON-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 1)
	body["manual_discount_amount"] = moneyIDR("5000") // di bawah batas; yang kurang hanya alasan

	status, out := doJSON(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", body)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("diskon manual tanpa alasan harus 422, got %d (%v)", status, out)
	}
	errObj, _ := out["error"].(map[string]any)
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "manual_discount_reason") {
		t.Errorf("pesan harus menyebut field yang kurang, got %q", msg)
	}

	// Dan yang lengkap tetap diterima — gerbang yang memblokir jalur benar akan
	// dimatikan orang.
	body["manual_discount_reason"] = "komplain pelanggan"
	status, out = doJSON(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", body)
	if status != http.StatusCreated {
		t.Fatalf("diskon manual DENGAN alasan harus diterima, got %d (%v)", status, out)
	}
}

// TestKafe_ManualDiscountRequiresAReasonOnUpdate — kondisi yang sama di jalur
// PATCH, dievaluasi atas pandangan GABUNGAN (record tersimpan + body), sehingga:
//
//	PATCH discount tanpa reason (record belum punya) → 422
//	PATCH discount tanpa reason (record SUDAH punya)  → 200  (syaratnya sudah terpenuhi)
func TestKafe_ManualDiscountRequiresAReasonOnUpdate(t *testing.T) {
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	admin := seedKafeAdminToken(t, app)
	sessionID := discountSession(t, app, ids, "MAN-REASON-2")

	body := orderBody(ids, sessionID, "MAN-REASON-2", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 1)
	status, out := doAuthed(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", admin, body)
	if status != http.StatusCreated {
		t.Fatalf("create: %d (%v)", status, out)
	}
	data, _ := out["data"].(map[string]any)
	orderID, _ := data["id"].(string)

	// Tanpa alasan, keduanya baru → ditolak.
	status, out = doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-order/order/"+orderID, admin,
		map[string]any{"manual_discount_amount": moneyIDR("7000")})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("PATCH diskon tanpa alasan harus 422, got %d (%v)", status, out)
	}

	// Dengan alasan → diterima, dan sesudah itu PATCH yang tidak menyentuh
	// syaratnya tetap boleh.
	status, out = doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-order/order/"+orderID, admin,
		map[string]any{"manual_discount_amount": moneyIDR("7000"), "manual_discount_reason": "disetujui manajer"})
	if status != http.StatusOK {
		t.Fatalf("PATCH dengan alasan: %d (%v)", status, out)
	}
	status, out = doAuthed(t, app, http.MethodPatch,
		"/kafe/_ui/entity/cafe-order/order/"+orderID, admin,
		map[string]any{"guest_note": "catatan lain"})
	if status != http.StatusOK {
		t.Fatalf("PATCH yang tidak menyentuh syarat tetap boleh: %d (%v)", status, out)
	}
}

// TestKafe_ManualDiscountRespectsItsDeclaredLimit — BATAS diskon manual sudah
// DINYATAKAN di Config (`manual_discount_limit_percent: 10`,
// `manual_discount_limit_amount: 50000`) tetapi **masih belum ditegakkan**
// (bagian (c) item 10.81 yang tersisa — kewajiban alasannya sudah ditutup
// 2026-10-07, angkanya belum):
//
//	create dengan manual_discount_amount=40500 (90%) → 201
func TestKafe_ManualDiscountRespectsItsDeclaredLimit(t *testing.T) {
	t.Skip("kafe 10.81 ⏸️ (c) — batas Config belum ditegakkan; kewajiban alasan sudah")
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	sessionID := discountSession(t, app, ids, "MAN-LIMIT-1")

	body := orderBody(ids, sessionID, "MAN-LIMIT-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 1)
	body["manual_discount_amount"] = moneyIDR("40500") // 90%
	body["manual_discount_reason"] = "alasan ada, batas yang dilanggar"

	status, out := doJSON(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", body)
	if status == http.StatusCreated {
		data, _ := out["data"].(map[string]any)
		t.Fatalf("diskon manual di atas batas Config diterima: %v (total=%v)",
			numberOf(data["manual_discount_amount"]), numberOf(data["total_amount"]))
	}
}

// TestKafe_LineDiscountReachesLineTotal — `lines[].discount_amount` ada dan
// dikirim, tetapi `line_total` (`quantity × unit_price_snapshot`) tidak
// menguranginya, sehingga subtotal pun tidak terpengaruh:
//
//	create → 201; line_total=90000 padahal diskon barisnya 40000 (harus 50000)
//
// Field yang menjanjikan efek dan tidak punya efek lebih buruk daripada field
// yang tidak ada: kasir mengisi diskon, struk mencetak total penuh, dan tidak ada
// yang memberi tahu bahwa diskonnya hilang.
func TestKafe_LineDiscountReachesLineTotal(t *testing.T) {
	t.Skip(itemWhySkipped + " — " + itemDiscountAuthority)
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	sessionID := discountSession(t, app, ids, "LINE-DISC-1")

	body := orderBody(ids, sessionID, "LINE-DISC-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 2)
	body["lines"] = []any{map[string]any{
		"line_no":             1,
		"menu_item_id":        ids.nasiGoreng,
		"name_snapshot":       "Nasi Goreng Spesial",
		"unit_price_snapshot": moneyIDR("45000"),
		"quantity":            2,
		"prep_station":        "kitchen",
		"discount_amount":     moneyIDR("40000"),
	}}

	status, out := doJSON(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", body)
	if status != http.StatusCreated {
		t.Fatalf("create: %d (%v)", status, out)
	}
	data, _ := out["data"].(map[string]any)
	line := lineByNumber(t, data, 1)
	if got := numberOf(line["line_total"]); got != 50000 {
		t.Fatalf("diskon baris tidak sampai ke line_total: got %v, want 50000 (2 × 45000 − 40000)", got)
	}
}

// TestKafe_ForgedDiscountCannotPostABalancedJournal — mengapa jurnal BUKAN
// penjaganya, dan kenapa ini penting.
//
// Jurnal dibangun dari komponen payload dan harus seimbang. Diskon ditambahkan
// sebagai DEBIT akun diskon, sementara `total_amount` (yang dipakai sebagai
// debit Kas) sudah dikurangi diskon — jadi kedua sisi sama dengan atau tanpa
// diskon palsu. Terukur:
//
//	Kas            debit 11975
//	Diskon         debit 40000
//	Omzet          credit 45000
//	Pajak          credit  4725
//	Service charge credit  2250   → debit 51975 = credit 51975, status posted
//
// Buku SEIMBANG, pesanan 45000 dibayar 11975, dan akun diskon menyerap 40000
// yang tidak disetujui aturan bisnis mana pun. Karena itu penegakannya harus di
// jalur tulis (siapa yang boleh menentukan nilainya), bukan di jurnal.
func TestKafe_ForgedDiscountCannotPostABalancedJournal(t *testing.T) {
	t.Skip(itemWhySkipped + " — " + itemDiscountAuthority)
	app := bootKafe(t)
	ids := seedKafeTableScenario(t, app)
	seedKafeAccounts(t, app)
	admin := seedKafeAdminToken(t, app)
	sessionID := discountSession(t, app, ids, "JRN-DISC-1")

	body := orderBody(ids, sessionID, "JRN-DISC-1", ids.nasiGoreng, "Nasi Goreng Spesial", "45000", 1)
	body["discount_amount"] = moneyIDR("40000")

	status, out := doJSON(t, app, http.MethodPost, "/kafe/_ui/entity/cafe-order/order", body)
	if status != http.StatusCreated {
		// Setelah 10.81 ditutup, jalur ini memang harus berhenti di sini.
		t.Skipf("create dengan diskon karangan ditolak (%d) — perilaku yang diinginkan", status)
	}
	data, _ := out["data"].(map[string]any)
	orderID, _ := data["id"].(string)

	patchOrderStatus(t, app, admin, orderID, "awaiting_payment")
	patchOrderStatus(t, app, admin, orderID, "paid")
	waitForJournal(t, app)

	entry := findJournalBySource(t, app, orderID)
	if entry == nil {
		t.Fatal("tidak ada jurnal — periksa apakah subscription menolak karena tidak seimbang")
	}
	debit, credit := 0.0, 0.0
	for _, ln := range journalLines(t, app, entry) {
		debit += numberOf(ln["debit"])
		credit += numberOf(ln["credit"])
	}
	total := numberOf(data["total_amount"])
	if debit == credit {
		t.Fatalf("jurnal seimbang (%v) untuk pesanan %v — jurnal tidak bisa menjadi penjaga diskon", debit, total)
	}
}

// lineByNumber finds a child row by `line_no` rather than by position, so the
// assertion does not depend on ordering.
func lineByNumber(t *testing.T, rec map[string]any, lineNo int) map[string]any {
	t.Helper()
	rows, _ := rec["lines"].([]any)
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if int(numberOf(row["line_no"])) == lineNo {
			return row
		}
	}
	t.Fatalf("record carries no line_no=%d: %v", lineNo, rows)
	return nil
}
