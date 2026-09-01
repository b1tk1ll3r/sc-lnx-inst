package main

import "testing"

func TestDistroFamily(t *testing.T) {
	cases := []struct {
		id, like, want string
	}{
		{"debian", "", "debian"},
		{"ubuntu", "debian", "debian"},
		{"linuxmint", "ubuntu debian", "debian"},
		{"fedora", "", "fedora"},
		{"nobara", "fedora", "fedora"},
		{"rocky", "rhel centos fedora", "fedora"},
		{"arch", "", "arch"},
		{"manjaro", "arch", "arch"},
		{"omarchy", "arch", "arch"},
		{"opensuse-tumbleweed", "suse opensuse", "suse"},
		{"opensuse-leap", "suse opensuse", "suse"},
		{"custom", "debian", "debian"},
		{"gentoo", "", "other"},
	}
	for _, tc := range cases {
		if got := distroFamily(tc.id, tc.like); got != tc.want {
			t.Fatalf("distroFamily(%q,%q)=%q want %q", tc.id, tc.like, got, tc.want)
		}
	}
}

func TestImmutableDistro(t *testing.T) {
	for _, tc := range []struct {
		id, variant string
	}{
		{"fedora", "silverblue"},
		{"fedora", "kinoite"},
		{"steamos", "steamdeck"},
		{"opensuse-microos", ""},
	} {
		if !immutableDistro(tc.id, tc.variant) {
			t.Fatalf("expected immutable: %#v", tc)
		}
	}
	if immutableDistro("fedora", "workstation") {
		t.Fatal("Fedora Workstation must not be treated as immutable")
	}
}
