package podman

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapabilityContract582584And61(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fixture string
		native  bool
	}{
		{"podman-5.8.2", "podman-5.8.2-version.json", false},
		{"podman-5.8.4", "podman-5.8.4-version.json", false},
		{"podman-6.1.0", "podman-6.1.0-version.json", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			versionBody, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			infoBody, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "podman-info.json"))
			if err != nil {
				t.Fatal(err)
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/version":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(versionBody)
				case strings.HasSuffix(r.URL.Path, "/libpod/quadlets/json"):
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte("[]"))
				case strings.HasSuffix(r.URL.Path, "/libpod/info"):
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(infoBody)
				default:
					http.NotFound(w, r)
				}
			}))
			defer s.Close()

			c := NewWithHTTPClient(s.Client(), s.URL)
			caps, err := c.DiscoverCapabilities(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !caps.QuadletInstall || !caps.QuadletRemove || !caps.QuadletList || !caps.QuadletPrint {
				t.Fatalf("missing common capabilities: %+v", caps)
			}
			if caps.NativeApplicationInstall != tc.native {
				t.Fatalf("native application capability=%v want %v", caps.NativeApplicationInstall, tc.native)
			}
			if !IsSupportedVersion(caps.Version) {
				t.Fatalf("fixture version %s unexpectedly below supported floor %s", caps.Version, SupportedMinimumVersion)
			}
			if caps.CgroupManager != "systemd" || caps.CgroupVersion != "v2" || caps.NetworkBackend != "netavark" {
				t.Fatalf("host info not decoded: %+v", caps)
			}
		})
	}
}
