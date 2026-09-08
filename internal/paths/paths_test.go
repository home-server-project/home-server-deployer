package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirContained(t *testing.T) {
	base := t.TempDir()
	host := "/srv/apps"
	r := Root{ID: "apps", AgentPath: base, HostPath: host}
	got, err := EnsureDir(r, "alpha/data", 0750)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(base, "alpha/data") {
		t.Fatalf("got %s", got)
	}
	if st, err := os.Stat(got); err != nil || !st.IsDir() {
		t.Fatalf("directory not created: %v", err)
	}
}

func TestEnsureDirRejectsTraversal(t *testing.T) {
	r := Root{ID: "apps", AgentPath: t.TempDir(), HostPath: "/srv/apps"}
	for _, v := range []string{"../etc", "/etc", "a/../b", "."} {
		if _, err := EnsureDir(r, v, 0750); err == nil {
			t.Fatalf("expected %q to fail", v)
		}
	}
}

func TestEnsureDirRejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	r := Root{ID: "apps", AgentPath: base, HostPath: "/srv/apps"}
	if _, err := EnsureDir(r, "link/child", 0750); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}

func TestEnsureDirRejectsSymlinkApprovedRoot(t *testing.T) {
	outside := t.TempDir()
	parent := t.TempDir()
	link := filepath.Join(parent, "root-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	r := Root{ID: "apps", AgentPath: link, HostPath: "/srv/apps"}
	if _, err := EnsureDir(r, "child", 0750); err == nil {
		t.Fatal("expected symlink approved-root rejection")
	}
}
