// Helper bersama untuk "download + verify + extract" artifact rilis FormSpec
// dari GitHub Releases. Dipakai oleh `formspec spa install` (artifact
// spa-<version>.tar.gz) dan `formspec upgrade` (artifact
// formspec-<os>-<arch>.tar.gz|.zip).
//
// Prinsip: HTTPS hanya ke sumber rilis, wajib cocok dengan SHA256SUMS.txt
// sebelum artifact dipercaya, dan tidak ada auto-download di background.
//
// Plan: docs_internal/plan/formspec-upgrade-command.md §Fase 1.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// defaultReleaseBaseURL — base URL download artifact rilis. Paritas dengan
// installer script (site/public/install.sh) yang juga menghormati
// FORMSPEC_RELEASE_BASE.
const defaultReleaseBaseURL = "https://github.com/primadi/formspec/releases/download"

// defaultGitHubAPIBase — base REST API untuk resolusi `releases/latest`.
const defaultGitHubAPIBase = "https://api.github.com/repos/primadi/formspec"

// releaseBaseURL mengembalikan base URL download (tanpa trailing slash),
// di-override via FORMSPEC_RELEASE_BASE (mirror/proxy/enterprise).
func releaseBaseURL() string {
	if b := os.Getenv("FORMSPEC_RELEASE_BASE"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return defaultReleaseBaseURL
}

// githubAPIBase mengembalikan base REST API, di-override via
// FORMSPEC_RELEASE_API (dipakai test + mirror enterprise).
func githubAPIBase() string {
	if b := os.Getenv("FORMSPEC_RELEASE_API"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return defaultGitHubAPIBase
}

// releaseGet fetch satu URL, memetakan status jadi pesan yang jelas
// (404 → artifact/release tidak ada).
func releaseGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download gagal: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == 404:
		return nil, fmt.Errorf("artifact tidak ditemukan (404): %s", url)
	case resp.StatusCode == 403:
		return nil, fmt.Errorf("HTTP 403 (rate-limit GitHub?) — coba `formspec upgrade --version <tag>`: %s", url)
	case resp.StatusCode != 200:
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// latestTag resolve tag rilis terbaru (stable) dari GitHub API.
func latestTag() (string, error) {
	return latestTagFrom(githubAPIBase())
}

// latestTagFrom = latestTag terhadap base API eksplisit (test/mirror).
func latestTagFrom(apiBase string) (string, error) {
	data, err := releaseGet(strings.TrimRight(apiBase, "/") + "/releases/latest")
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(data, &rel); err != nil {
		return "", fmt.Errorf("parse release JSON: %w", err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("release terbaru tidak punya tag_name")
	}
	return rel.TagName, nil
}

// releaseAssetName memetakan platform ke nama artifact rilis. Matriks yang
// didukung identik dengan Makefile RELEASE_TARGETS.
func releaseAssetName(goos, goarch string) (string, error) {
	var ext string
	switch goos {
	case "linux", "darwin":
		ext = "tar.gz"
	case "windows":
		ext = "zip"
	default:
		return "", fmt.Errorf("OS tidak didukung: %s (tersedia: linux, darwin, windows)", goos)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("arsitektur tidak didukung: %s (tersedia: amd64, arm64)", goarch)
	}
	return fmt.Sprintf("formspec-%s-%s.%s", goos, goarch, ext), nil
}

// checksumFromSums mengekstrak digest untuk `name` dari file SHA256SUMS.txt
// (format "  <hex>  <name>"). "" bila tidak ada.
func checksumFromSums(sums, name string) string {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && filepath.Base(fields[1]) == name {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

// verifyArtifact memastikan sha256(data) cocok dengan entri `name` di
// SHA256SUMS.txt. Mismatch / entri absent = error (artifact tidak dipercaya).
func verifyArtifact(data []byte, sums, name string) error {
	want := checksumFromSums(sums, name)
	if want == "" {
		return fmt.Errorf("SHA256SUMS.txt tidak berisi entri untuk %s — download dibatalkan", name)
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch untuk %s — download dibatalkan\n   want %s\n   got  %s",
			name, want, hex.EncodeToString(got[:]))
	}
	return nil
}

// extractTarGz extract tar.gz ke dest, opsional membuang prefix level atas
// (mis. "spa/"), dengan path-traversal guard.
func extractTarGz(data []byte, dest, stripPrefix string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := hdr.Name
		if stripPrefix != "" {
			name = strings.TrimPrefix(name, stripPrefix)
		}
		if name == "" || name == "." {
			continue
		}
		clean := filepath.Clean(name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("path tidak valid di tar: %s", hdr.Name)
		}
		target := filepath.Join(dest, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			// Flush before declaring success: an unchecked close reports a
			// truncated extraction as a completed one.
			if err := out.Close(); err != nil {
				return fmt.Errorf("write %s: %w", target, err)
			}
		}
	}
}

// extractArchiveMember mengekstrak satu member (dicocokkan via base name)
// dari artifact `.tar.gz` atau `.zip`, lalu menulisnya sebagai file executable
// di destPath. Dipakai `formspec upgrade` untuk mengambil binary `formspec`
// dari arsip rilis.
func extractArchiveMember(data []byte, archiveName, member, destPath string) error {
	var content []byte
	var err error
	if strings.HasSuffix(archiveName, ".zip") {
		content, err = zipMember(data, member)
	} else {
		content, err = tarGzMember(data, member)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(destPath, content, 0o755); err != nil {
		return err
	}
	return os.Chmod(destPath, 0o755)
}

// tarGzMember mengembalikan isi member pertama yang base name-nya == member.
func tarGzMember(data []byte, member string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != member {
			continue
		}
		return io.ReadAll(tr)
	}
	return nil, fmt.Errorf("arsip tidak berisi %s", member)
}

// zipMember mengembalikan isi member pertama yang base name-nya == member.
func zipMember(data []byte, member string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || filepath.Base(f.Name) != member {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		return content, nil
	}
	return nil, fmt.Errorf("arsip tidak berisi %s", member)
}
