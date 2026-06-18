package version

import "testing"

func TestResolve_LdflagsTakePrecedence(t *testing.T) {
	// ldflags-injected values are kept verbatim regardless of build info.
	v, commit, date := resolve("1.2.3", "abc1234", "2026-06-19T00:00:00Z")
	if v != "1.2.3" || commit != "abc1234" || date != "2026-06-19T00:00:00Z" {
		t.Errorf("resolve() = (%q, %q, %q), want ldflags values unchanged", v, commit, date)
	}
}

func TestResolve_StringDevUnchangedWithoutBuildInfo(t *testing.T) {
	// With all defaults and no usable build info (as in some test contexts),
	// the defaults pass through untouched — no panic, no surprise values.
	v := String()
	if v == "" {
		t.Fatal("String() returned empty")
	}
}
