package selinux

import "testing"

func TestMountSuffix(t *testing.T) {
	b := New()
	cases := map[string]string{"private": "Z", "cache": "Z", "database": "Z", "shared": "z"}
	for intent, want := range cases {
		got, err := b.MountSuffix(intent, false)
		if err != nil || got != want {
			t.Fatalf("%s got %q err %v", intent, got, err)
		}
	}
	if got, err := b.MountSuffix("readonly-media", true); err != nil || got != "ro" {
		t.Fatalf("readonly got %q err %v", got, err)
	}
	if _, err := b.MountSuffix("readonly-media", false); err == nil {
		t.Fatal("expected readonly enforcement")
	}
}
