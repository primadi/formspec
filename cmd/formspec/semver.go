// Comparator semver minimal (subset yang dipakai tag rilis FormSpec:
// `vMAJOR.MINOR.PATCH[-prerelease][+build]`). Ditulis in-repo karena
// golang.org/x/mod tidak ada di go.mod dan menambah dependency hanya untuk
// 40 baris ini tidak sepadan.
//
// Plan: docs_internal/plan/formspec-upgrade-command.md §Fase 2.
package main

import (
	"fmt"
	"strconv"
	"strings"
)

type semver struct {
	major, minor, patch int
	pre                 []string // identifier prerelease, kosong = release
}

// parseSemver mem-parse "v1.2.3", "1.2.3", "v1.2.3-rc.1", "v1.2.3+build".
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
		v.pre = strings.Split(pre, ".")
	}
	return v, nil
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
	return comparePre(va.pre, vb.pre), nil
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
