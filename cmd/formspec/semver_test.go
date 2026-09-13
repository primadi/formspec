package main

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.0.7", "v0.0.7", 0},
		{"0.0.7", "v0.0.7", 0},
		{"v0.0.6", "v0.0.7", -1},
		{"v0.0.7", "v0.0.6", 1},
		{"v0.1.0", "v0.0.99", 1},
		{"v1.0.0", "v0.99.99", 1},
		{"v1.2.3", "v1.10.0", -1},
		{"v1.0.0-rc.1", "v1.0.0", -1}, // prerelease < release
		{"v1.0.0", "v1.0.0-rc.1", 1},
		{"v1.0.0-rc.1", "v1.0.0-rc.2", -1}, // identifier numerik
		{"v1.0.0-rc.2", "v1.0.0-rc.10", -1},
		{"v1.0.0-alpha", "v1.0.0-beta", -1}, // leksikal
		{"v1.0.0-alpha", "v1.0.0-alpha.1", -1},
		{"v1.0.0-rc.1", "v1.0.0-alpha.1", 1},
		{"v1.0.0-1", "v1.0.0-alpha", -1}, // numerik < alfanumerik
		{"v1.2.3+build.1", "v1.2.3+build.2", 0},
	}
	for _, c := range cases {
		got, err := compareVersions(c.a, c.b)
		if err != nil {
			t.Errorf("compareVersions(%q,%q) error: %v", c.a, c.b, err)
			continue
		}
		if got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCompareVersionsInvalid(t *testing.T) {
	for _, s := range []string{"dev", "", "v1.2", "v1.2.3.4", "vx.y.z", "v1.-1.0"} {
		if _, err := compareVersions(s, "v1.0.0"); err == nil {
			t.Errorf("compareVersions(%q,...): want error, got nil", s)
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, candidate string
		want               bool
	}{
		{"v0.0.6", "v0.0.7", true},
		{"v0.0.7", "v0.0.7", false},
		{"v0.0.8", "v0.0.7", false},
		{"v1.0.0-rc.1", "v1.0.0", true},
	}
	for _, c := range cases {
		got, err := isNewer(c.current, c.candidate)
		if err != nil {
			t.Fatalf("isNewer(%q,%q): %v", c.current, c.candidate, err)
		}
		if got != c.want {
			t.Errorf("isNewer(%q,%q) = %v, want %v", c.current, c.candidate, got, c.want)
		}
	}
	if _, err := isNewer("dev", "v0.0.7"); err == nil {
		t.Error("isNewer(dev,...): want error, got nil")
	}
}
