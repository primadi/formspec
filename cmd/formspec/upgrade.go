// Command formspec upgrade — self-update binary dari GitHub Releases.
//
// Alur: resolve versi target (latest / --version) → cek writable → download
// artifact formspec-<os>-<arch>.{tar.gz,zip} + SHA256SUMS.txt → verify checksum
// → smoke test binary baru → swap atomik di path os.Executable().
//
// Prinsip (konsisten dengan `formspec spa install`): eksplisit, bukan
// silent-magic; rilis resmi saja; tanpa `sudo`; tidak ada auto-update.
//
// Plan: docs_internal/plan/formspec-upgrade-command.md §Fase 3–4.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func runUpgrade(args []string) {
	fs := flag.NewFlagSet("formspec upgrade", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	target := fs.String("version", "", "upgrade/rollback ke tag tertentu (default: latest)")
	check := fs.Bool("check", false, "hanya cek versi terbaru; tidak mengubah binary")
	dryRun := fs.Bool("dry-run", false, "tampilkan rencana tanpa download")
	force := fs.Bool("force", false, "paksa walau versi sama (re-install binary)")
	yes := fs.Bool("yes", false, "non-interaktif: jangan minta konfirmasi")
	fs.Usage = upgradeUsage
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "❌ Argumen tidak dikenal: %s\n\n", strings.Join(fs.Args(), " "))
		upgradeUsage()
		os.Exit(2)
	}
	if err := upgradeRun(*target, *check, *dryRun, *force, *yes); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

func upgradeUsage() {
	_, _ = fmt.Fprint(os.Stderr, `Usage: formspec upgrade [flags]

Self-update binary dari GitHub Releases (tanpa install ulang).

Flags:
  --version <tag>   Upgrade/rollback ke tag tertentu (default: latest)
  --check           Hanya cek versi terbaru; tidak mengubah binary
  --dry-run         Tampilkan rencana (versi, URL, path) tanpa download
  --force           Paksa walau versi sama (re-install binary)
  --yes             Non-interaktif (jangan minta konfirmasi)

Contoh:
  formspec upgrade                     # ke versi terbaru
  formspec upgrade --check             # cek tanpa mengubah
  formspec upgrade --version v0.0.7    # pin / rollback
`)
}

// upgradeRun menjalankan seluruh alur upgrade dan mengembalikan error (bukan
// memanggil os.Exit) supaya bisa diuji.
func upgradeRun(wantTag string, checkOnly, dryRun, force, assumeYes bool) error {
	if version == "dev" {
		return fmt.Errorf(`binary ini build "dev" (tanpa tag rilis) — tidak ada versi untuk di-upgrade.

Untuk development, jalankan dari repo checkout atau build ulang:
  make build-formspec
Untuk install rilis resmi: curl -fsSL https://formspec.dev/install.sh | sh`)
	}

	exePath, err := resolveUpgradeExePath()
	if err != nil {
		return err
	}

	base := releaseBaseURL()
	target := wantTag
	if target == "" {
		fmt.Println("🔎 Mencari versi terbaru...")
		target, err = latestTag()
		if err != nil {
			return fmt.Errorf("%w\n   (atau pin versi: formspec upgrade --version vX.Y.Z)", err)
		}
	}

	asset, err := releaseAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	// Bandingkan versi. Tag identik = sama; kalau tidak, parse semver penuh.
	cmp := 0
	var cmpErr error
	if version == target {
		cmp = 0
	} else {
		cmp, cmpErr = compareVersions(version, target)
	}

	if checkOnly {
		switch {
		case cmpErr != nil:
			fmt.Printf("\n(bandingkan versi gagal: %v)\n", cmpErr)
		case cmp == 0:
			fmt.Printf("\n✅ Sudah versi terbaru (%s).\n", version)
		case cmp < 0:
			fmt.Printf("\n⬆️  Versi lebih baru tersedia: %s → %s\n", version, target)
		default:
			fmt.Printf("\n⬇️  Target lebih lama: %s → %s (rollback)\n", version, target)
		}
		return nil
	}

	if cmp == 0 && !force {
		fmt.Printf("✅ Sudah versi terbaru (%s). Gunakan --force untuk re-install binary.\n", version)
		return nil
	}
	if cmpErr != nil {
		return fmt.Errorf("tidak bisa membandingkan versi binary %q dengan target %q: %w\n   (pin eksplisit: formspec upgrade --version <tag>, atau --force)", version, target, cmpErr)
	}

	fmt.Printf("  current : %s\n  target  : %s\n  asset   : %s\n  path    : %s\n", version, target, asset, exePath)

	if dryRun {
		fmt.Printf("\n🧪 Dry-run — tidak ada perubahan.\n   artifact : %s/%s/%s\n   checksum : %s/%s/SHA256SUMS.txt\n", base, target, asset, base, target)
		return nil
	}

	if err := ensureWritableDir(filepath.Dir(exePath)); err != nil {
		return err
	}
	cleanupUpgradeOld(exePath)

	if cmp > 0 && !assumeYes && isTTY(os.Stdin) {
		if !confirm(fmt.Sprintf("Target %s lebih lama dari %s (rollback). Lanjutkan?", target, version)) {
			fmt.Println("Dibatalkan.")
			return nil
		}
	}

	fmt.Printf("\n⬇️  Download %s/%s/%s\n", base, target, asset)
	newPath, err := downloadVerifiedBinary(base, target, asset, filepath.Dir(exePath))
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(newPath) }()

	if err := smokeTestVersion(newPath, target); err != nil {
		return err
	}
	fmt.Println("🔒 Checksum SHA256 OK")
	fmt.Println("🧪 Smoke test binary baru OK")

	if err := swapBinary(exePath, newPath); err != nil {
		return err
	}

	fmt.Printf("✅ Upgrade %s → %s\n   %s\n", version, target, exePath)
	if spaCacheInstalled(version) {
		fmt.Printf("ℹ️  Cache SPA untuk %s masih ada; versi baru butuh: formspec spa install\n", version)
	}
	return nil
}

// resolveUpgradeExePath = var agar test bisa mengarah ke binary di temp dir
// tanpa menyentuh binary test yang sedang berjalan.
var resolveUpgradeExePath = resolveExePath

// resolveExePath mengembalikan path binary ini (target symlink-nya, bukan
// symlink) supaya self-replace mengenai file sebenarnya.
func resolveExePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve path binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

// binaryName = nama member di dalam arsip rilis (dan nama binary terpasang).
func binaryName(goos string) string {
	if goos == "windows" {
		return "formspec.exe"
	}
	return "formspec"
}

// upgradeTempPath = path file sementara DI DIREKTORI BINARY (satu filesystem
// agar rename atomik). Di Windows wajib berakhiran .exe supaya bisa dijalankan
// untuk smoke test.
func upgradeTempPath(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "formspec-new.exe")
	}
	return filepath.Join(dir, ".formspec-new")
}

// ensureWritableDir memastikan direktori binary bisa ditulis SEBELUM download
// (fail cepat dengan pesan actionable). Tidak pernah menyarankan sudo.
func ensureWritableDir(dir string) error {
	f, err := os.CreateTemp(dir, ".formspec-write-test-")
	if err != nil {
		return fmt.Errorf(`direktori binary tidak writable: %s
   Upgrade butuh izin tulis ke folder tersebut. Alternatif:
     - install ulang lewat installer (menimpa slot yang sama):
       curl -fsSL https://formspec.dev/install.sh | sh
     - atau pindahkan binary ke folder milik user (mis. ~/.local/bin)
     - bila dikelola package manager, upgrade lewat package manager Anda`, dir)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

// isTTY melaporkan apakah file adalah terminal (untuk gate konfirmasi).
func isTTY(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// confirm meminta konfirmasi y/N dari stdin.
func confirm(prompt string) bool {
	_, _ = fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	var ans string
	if _, err := fmt.Scanln(&ans); err != nil {
		// stdin tertutup / EOF: perlakukan sebagai "tidak". Langkah yang meminta
		// konfirmasi bersifat destruktif, jadi prompt yang gagal dibaca tidak
		// boleh berubah menjadi persetujuan.
		return false
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	return ans == "y" || ans == "yes"
}

// downloadVerifiedBinary mengunduh artifact + SHA256SUMS, verify checksum, lalu
// mengekstrak binary ke file sementara di dir. Mengembalikan path file baru.
// Binary lama tidak disentuh sampai swap.
func downloadVerifiedBinary(baseURL, tag, asset, dir string) (string, error) {
	artifact, err := releaseGet(fmt.Sprintf("%s/%s/%s", baseURL, tag, asset))
	if err != nil {
		return "", err
	}
	sums, err := releaseGet(fmt.Sprintf("%s/%s/SHA256SUMS.txt", baseURL, tag))
	if err != nil {
		return "", err
	}
	if err := verifyArtifact(artifact, string(sums), asset); err != nil {
		return "", err
	}
	dest := upgradeTempPath(dir)
	if err := extractArchiveMember(artifact, asset, binaryName(runtime.GOOS), dest); err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}
	return dest, nil
}

// smokeTestVersion menjalankan binary baru dan memastikan versinya == tag
// target. Mencegah swap ke binary yang rusak (checksum cocok tapi isi salah).
func smokeTestVersion(path, tag string) error {
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		return fmt.Errorf("binary baru gagal dijalankan: %w — upgrade dibatalkan", err)
	}
	got := strings.TrimSpace(string(out))
	if !strings.Contains(got, tag) {
		return fmt.Errorf("binary baru melaporkan %q, bukan %q — upgrade dibatalkan", got, tag)
	}
	return nil
}

// swapBinary menggantikan binary lama dengan yang baru.
//
// Unix: os.Rename atomik (proses yang sedang jalan tetap memegang inode lama).
// Windows: .exe yang berjalan tidak bisa dihapus/di-overwrite, tapi BISA
// di-rename — rename lama ke .old, pasang baru, lalu coba hapus .old (gagal
// dihapus bukan error; disapu saat upgrade berikutnya).
func swapBinary(exePath, newPath string) error {
	if runtime.GOOS == "windows" {
		return swapBinaryWindows(exePath, newPath)
	}
	if err := os.Rename(newPath, exePath); err != nil {
		return fmt.Errorf("gagal memasang binary baru: %w", err)
	}
	return nil
}

func swapBinaryWindows(exePath, newPath string) error {
	old := exePath + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exePath, old); err != nil {
		return fmt.Errorf("gagal memindahkan binary lama: %w", err)
	}
	if err := os.Rename(newPath, exePath); err != nil {
		_ = os.Rename(old, exePath) // pulihkan
		return fmt.Errorf("gagal memasang binary baru: %w", err)
	}
	_ = os.Remove(old)
	return nil
}

// cleanupUpgradeOld menyapu sisa *.old dari upgrade Windows sebelumnya.
func cleanupUpgradeOld(exePath string) {
	_ = os.Remove(exePath + ".old")
}
