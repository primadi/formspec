// Comparator semver minimal (subset yang dipakai tag rilis FormSpec:
// `vMAJOR.MINOR.PATCH[-prerelease][+build]`). Ditulis in-repo karena
// golang.org/x/mod tidak ada di go.mod dan menambah dependency hanya untuk
// 40 baris ini tidak sepadan.
//
// Kekhususan FormSpec: string git-describe (`v0.0.8-4-gceaaf2a`, 4 commit
// setelah tag v0.0.8) dikenali TERPISAH dari prerelease semver — snapshot itu
// lebih baru dari rilis core-nya, bukan lebih lama. Lihat classifyPre().
//
// Plan: docs_internal/plan/formspec-upgrade-command.md §Fase 2,
// docs_internal/plan/release-version-auto.md.
package main

import (
	"fmt"
	"strconv"
	"strings"
)

type semver struct {
	major, minor, patch int
	pre                 []string // identifier prerelease, kosong = release
	post                int      // >0 = suffix git-describe "-<n>-g<hash>": n commit SETELAH tag core
}

// parseSemver mem-parse "v1.2.3", "1.2.3", "v1.2.3-rc.1", "v1.2.3+build",
// serta output `git describe --tags` ("v1.2.3-4-gceaaf2a", "…-dirty").
func parseSemver(s string) (semver, error) {
	var v semver
	orig := s
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 { // build metadata diabaikan saat compare
		s = s[:i]
	}
	pre := ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("versi bukan semver: %q", orig)
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, fmt.Errorf("versi bukan semver: %q", orig)
		}
		nums[i] = n
	}
	v.major, v.minor, v.patch = nums[0], nums[1], nums[2]
	if pre != "" {
		v.pre, v.post = classifyPre(pre)
	}
	return v, nil
}

// classifyPre memisahkan suffix git-describe dari prerelease semver biasa.
//
// `v0.0.8-4-gceaaf2a` bukan prerelease v0.0.8: artinya "4 commit setelah tag
// v0.0.8", jadi urutannya SETELAH v0.0.8 (dan setelah v0.0.8-rc.1). Dibaca
// sebagai prerelease, rilis yang isinya lebih baru justru tampak rollback —
// pernah terjadi saat tag describe ikut ter-publish
// (docs_internal/plan/release-version-auto.md).
//
// Pemisah identifier describe adalah '-' (bukan '.') sehingga "<n>-g<hash>"
// tidak bisa dikenali setelah pre di-split per titik.
func classifyPre(pre string) (ids []string, post int) {
	trimmed := strings.TrimSuffix(pre, "-dirty")
	if i := strings.LastIndexByte(trimmed, '-'); i > 0 {
		if count, err := strconv.Atoi(trimmed[:i]); err == nil && isShortHash(trimmed[i+1:]) {
			return nil, count
		}
	}
	if trimmed == "dirty" { // worktree kotor tidak mengubah urutan versi
		return nil, 0
	}
	return strings.Split(pre, "."), 0
}

// isShortHash mengenali identifier "g<hex>" — prefix yang dipakai git describe.
func isShortHash(s string) bool {
	if len(s) < 5 || s[0] != 'g' {
		return false
	}
	for _, c := range s[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// compareVersions mengembalikan -1 bila a<b, 0 bila sama, +1 bila a>b.
// Aturan semver: prerelease lebih rendah dari release (1.0.0-rc.1 < 1.0.0).
func compareVersions(a, b string) (int, error) {
	va, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	vb, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	if c := compareInt(va.major, vb.major); c != 0 {
		return c, nil
	}
	if c := compareInt(va.minor, vb.minor); c != 0 {
		return c, nil
	}
	if c := compareInt(va.patch, vb.patch); c != 0 {
		return c, nil
	}
	// Snapshot git-describe berada SETELAH rilis core-nya (dan setelah
	// prerelease-nya): v0.0.8-4-gceaaf2a > v0.0.8 > v0.0.8-rc.1.
	if va.post != 0 || vb.post != 0 {
		return comparePost(va.post, vb.post), nil
	}
	return comparePre(va.pre, vb.pre), nil
}

// comparePost mengurutkan snapshot git-describe pada core versi yang sama:
// non-snapshot lebih rendah dari snapshot, antar-snapshot dibanding jumlah
// commit-nya.
func comparePost(a, b int) int {
	switch {
	case a == b:
		return 0
	case a == 0:
		return -1
	case b == 0:
		return 1
	default:
		return compareInt(a, b)
	}
}

// isNewer melaporkan apakah candidate lebih baru dari current.
func isNewer(current, candidate string) (bool, error) {
	c, err := compareVersions(current, candidate)
	if err != nil {
		return false, err
	}
	return c < 0, nil
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// comparePre: tanpa prerelease > dengan prerelease; identifier dibanding
// per-elemen (numerik secara numerik, alfanumerik secara leksikal, numerik
// selalu lebih rendah dari alfanumerik).
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := comparePreIdent(a[i], b[i]); c != 0 {
			return c
		}
	}
	return compareInt(len(a), len(b))
}

func comparePreIdent(a, b string) int {
	an, aerr := strconv.Atoi(a)
	bn, berr := strconv.Atoi(b)
	switch {
	case aerr == nil && berr == nil:
		return compareInt(an, bn)
	case aerr == nil:
		return -1 // numerik < alfanumerik
	case berr == nil:
		return 1
	default:
		return strings.Compare(a, b)
	}
}
