package podman

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompatibilityProfilesSeparateCapabilitiesFromSupportPolicy(t *testing.T) {
	apiIntroduced := ProfileForVersion("5.8.0")
	if !apiIntroduced.QuadletInstall || !apiIntroduced.QuadletPrint || !apiIntroduced.QuadletRemove {
		t.Fatalf("5.8.0 Quadlet API profile missing capabilities: %+v", apiIntroduced)
	}
	if IsSupportedVersion("5.8.0") {
		t.Fatal("5.8.0 API capability introduction must not imply product support")
	}

	for _, version := range []string{"5.8.2", "5.8.4"} {
		profile := ProfileForVersion(version)
		if !profile.QuadletInstall || profile.NativeApplicationInstall {
			t.Fatalf("bad %s profile: %+v", version, profile)
		}
		if !IsSupportedVersion(version) {
			t.Fatalf("%s should satisfy the product support floor", version)
		}
	}

	v61 := ProfileForVersion("6.1.0")
	if !v61.QuadletInstall || !v61.NativeApplicationInstall || !IsSupportedVersion("6.1.0") {
		t.Fatalf("bad 6.1.0 profile: %+v", v61)
	}

	old := ProfileForVersion("5.7.2")
	if old.QuadletInstall {
		t.Fatalf("pre-5.8 Quadlet REST capability unexpectedly enabled: %+v", old)
	}
	if IsSupportedVersion("5.8.1") {
		t.Fatal("5.8.1 must remain below the Home Server Deployer supported floor")
	}
}

func TestInstallQuadletNeverRequestsNativeReplace(t *testing.T) {
	var installQuery string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Version":"5.8.4","Components":[{"Name":"Podman Engine","Version":"5.8.4","Details":{"APIVersion":"5.8.0","MinAPIVersion":"4.0.0"}}]}`))
		case strings.HasSuffix(r.URL.Path, "/libpod/quadlets") && r.Method == http.MethodPost:
			installQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()

	client := NewWithHTTPClient(s.Client(), s.URL)
	if err := client.InstallQuadlet(context.Background(), "test.container", "[Container]\nImage=alpine\n"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(installQuery, "replace=") {
		t.Fatalf("Alpha 0 must not request native Quadlet replace: %q", installQuery)
	}
	if !strings.Contains(installQuery, "reload-systemd=false") {
		t.Fatalf("Quadlet install must defer systemd reload: %q", installQuery)
	}
}
