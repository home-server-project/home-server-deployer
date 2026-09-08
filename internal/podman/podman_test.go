package podman

import "testing"

func TestCompatibilityProfiles(t *testing.T) {
	v58 := ProfileForVersion("5.8.4"); if !v58.QuadletInstall || v58.NativeApplicationInstall { t.Fatalf("bad 5.8.4 profile: %+v", v58) }
	v61 := ProfileForVersion("6.1.0"); if !v61.QuadletInstall || !v61.NativeApplicationInstall { t.Fatalf("bad 6.1.0 profile: %+v", v61) }
	old := ProfileForVersion("5.7.2"); if old.QuadletInstall { t.Fatalf("unsupported baseline unexpectedly enabled: %+v", old) }
}
