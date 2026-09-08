package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/home-server-project/home-server-deployer/internal/catalog"
	"github.com/home-server-project/home-server-deployer/internal/config"
	"github.com/home-server-project/home-server-deployer/internal/docsgen"
	"github.com/home-server-project/home-server-deployer/internal/model"
	"github.com/home-server-project/home-server-deployer/internal/podman"
	"github.com/home-server-project/home-server-deployer/internal/state"
)

type fp struct {
	caps model.PodmanCapabilities
	q    map[string]string
	ops  []string
}

func (f *fp) DiscoverCapabilities(context.Context) (model.PodmanCapabilities, error) {
	return f.caps, nil
}

func (f *fp) ListQuadlets(context.Context) ([]model.Quadlet, error) {
	o := []model.Quadlet{}
	for n := range f.q {
		o = append(o, model.Quadlet{Name: n, UnitName: strings.TrimSuffix(n, filepath.Ext(n)) + ".service", Status: "active/running"})
	}
	return o, nil
}

func (f *fp) GetQuadlet(_ context.Context, n string) (string, error) {
	v, ok := f.q[n]
	if !ok {
		return "", os.ErrNotExist
	}
	return v, nil
}

func (f *fp) InstallQuadlet(_ context.Context, n, c string) error {
	if _, ok := f.q[n]; ok {
		return errors.New("exists")
	}
	f.q[n] = c
	f.ops = append(f.ops, "install:"+n)
	return nil
}

func (f *fp) RemoveQuadlet(_ context.Context, n string, _ bool) error {
	delete(f.q, n)
	f.ops = append(f.ops, "remove:"+n)
	return nil
}

func (f *fp) ListContainers(context.Context) ([]model.RuntimeContainer, error) { return nil, nil }
func (f *fp) Logs(context.Context, string, int) (string, error)                { return "", nil }

type fsd struct {
	reloads     int
	starts      int
	stops       int
	restarts    int
	failRestart bool
}

func (f *fsd) Capabilities(context.Context) model.SystemdCapabilities {
	return model.SystemdCapabilities{Available: true, Version: "259"}
}
func (f *fsd) Reload(context.Context) error { f.reloads++; return nil }
func (f *fsd) Start(context.Context, string) error {
	f.starts++
	return nil
}
func (f *fsd) Stop(context.Context, string) error {
	f.stops++
	return nil
}
func (f *fsd) Restart(context.Context, string) error {
	f.restarts++
	if f.failRestart {
		f.failRestart = false
		return errors.New("injected")
	}
	return nil
}
func (f *fsd) Status(context.Context, string) (string, error) { return "loaded/active/running", nil }
func (f *fsd) Close()                                         {}

type fsec struct{}

func (fsec) Capabilities() model.SecurityCapabilities {
	return model.SecurityCapabilities{Backend: "selinux", Enabled: true, Enforcing: true}
}
func (fsec) MountSuffix(string, bool) (string, error) { return "Z", nil }

type fix struct {
	e          *Engine
	p          *fp
	s          *fsd
	st         *state.Store
	root, docs string
	tpl        string
}

func fixture(t *testing.T) *fix { return fixtureForVersion(t, "5.8.4") }

func fixtureForVersion(t *testing.T, version string) *fix {
	t.Helper()
	r := t.TempDir()
	cr := filepath.Join(r, "catalog")
	ad := filepath.Join(cr, "alpha-smoke")
	if err := os.MkdirAll(ad, 0o755); err != nil {
		t.Fatal(err)
	}
	app := model.Catalog{
		APIVersion: model.CatalogAPIVersion,
		Kind:       "Application",
		Metadata:   model.AppMetadata{ID: "alpha-smoke", Name: "Alpha", Version: "0.1.0"},
		Spec: model.AppSpec{
			MinPodmanVersion: "5.8.2",
			Inputs:           []model.InputSpec{{Name: "data-subpath", Type: "path", Required: true, Default: "alpha-smoke"}},
			Directories:      []model.DirectorySpec{{Name: "data", RootID: "application-data", SubpathInput: "data-subpath", ContainerPath: "/data", SecurityIntent: "private"}},
			Resources:        []model.ResourceSpec{{Name: "alpha-smoke.container", Template: "a.tmpl", Start: true}},
			Update:           model.UpdatePolicy{Mode: "manual-only"},
			Health:           model.HealthContract{Strategy: "runtime"},
			Backup:           model.BackupContract{Strategy: "filesystem", Consistency: "stop-required", Includes: []string{"data"}},
		},
	}
	tp := filepath.Join(ad, "a.tmpl")
	if err := os.WriteFile(tp, []byte("[Container]\nImage=alpine:3.22\nExec=sleep infinity\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := catalog.NewStore(cr, []model.Catalog{app})
	if err != nil {
		t.Fatal(err)
	}
	st := state.New(filepath.Join(r, "state"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	ar := filepath.Join(r, "apps")
	if err := os.MkdirAll(ar, 0o755); err != nil {
		t.Fatal(err)
	}
	dr := filepath.Join(r, "docs")
	cfg := config.Default()
	cfg.ApprovedRoots = []config.Root{{ID: "application-data", HostPath: "/var/lib/home-server-apps", AgentPath: ar}}
	p := &fp{caps: model.PodmanCapabilities{Version: version, QuadletList: true, QuadletInstall: true, QuadletRemove: true}, q: map[string]string{}}
	sd := &fsd{}
	return &fix{New(p, sd, c, st, docsgen.New(dr), fsec{}, cfg), p, sd, st, ar, dr, tp}
}

func install(t *testing.T, f *fix) model.Instance {
	t.Helper()
	p, err := f.e.CreatePlan(context.Background(), model.PlanRequest{Operation: "install", AppID: "alpha-smoke", InstanceID: "alpha-smoke", Parameters: map[string]string{"data-subpath": "alpha-smoke"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.e.ExecutePlan(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	x, err := f.st.GetInstance("alpha-smoke")
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func update(t *testing.T, f *fix) {
	t.Helper()
	p, err := f.e.CreatePlan(context.Background(), model.PlanRequest{Operation: "update", AppID: "alpha-smoke", InstanceID: "alpha-smoke"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.e.ExecutePlan(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
}

func TestInstallAndDocs(t *testing.T) {
	f := fixture(t)
	x := install(t, f)
	if f.s.reloads != 1 || f.s.starts != 1 {
		t.Fatal("lifecycle")
	}
	if x.Resources["alpha-smoke.container"].SHA256 == "" {
		t.Fatal("hash")
	}
	if _, err := os.Stat(filepath.Join(f.docs, "alpha-smoke", "README.md")); err != nil {
		t.Fatal(err)
	}
}

func TestProductSupportFloorIs582(t *testing.T) {
	f := fixtureForVersion(t, "5.8.1")
	_, err := f.e.CreatePlan(context.Background(), model.PlanRequest{Operation: "install", AppID: "alpha-smoke", InstanceID: "alpha-smoke", Parameters: map[string]string{"data-subpath": "alpha-smoke"}})
	if err == nil || !strings.Contains(err.Error(), podman.SupportedMinimumVersion) {
		t.Fatalf("expected supported-floor rejection, got %v", err)
	}
}

func TestDriftRefusesUpdate(t *testing.T) {
	f := fixture(t)
	install(t, f)
	f.p.q["alpha-smoke.container"] += "\n# admin\n"
	p, err := f.e.CreatePlan(context.Background(), model.PlanRequest{Operation: "update", AppID: "alpha-smoke", InstanceID: "alpha-smoke"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Drift) != 1 {
		t.Fatal("drift not found")
	}
	if _, err = f.e.ExecutePlan(context.Background(), p.ID); err == nil {
		t.Fatal("drift overwrite allowed")
	}
}

func TestBrokenUpdateRollsBack(t *testing.T) {
	f := fixture(t)
	install(t, f)
	old := f.p.q["alpha-smoke.container"]
	if err := os.WriteFile(f.tpl, []byte("[Container]\nImage=broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := f.e.CreatePlan(context.Background(), model.PlanRequest{Operation: "update", AppID: "alpha-smoke", InstanceID: "alpha-smoke"})
	if err != nil {
		t.Fatal(err)
	}
	f.s.failRestart = true
	if _, err = f.e.ExecutePlan(context.Background(), p.ID); err == nil {
		t.Fatal("expected failure")
	}
	if f.p.q["alpha-smoke.container"] != old {
		t.Fatal("rollback mismatch")
	}
}

func TestShorterQuadletUpdateRemovesTrailingDirectiveOnAllSupportedTargets(t *testing.T) {
	for _, version := range []string{"5.8.2", "5.8.4", "6.1.0"} {
		t.Run(version, func(t *testing.T) {
			f := fixtureForVersion(t, version)
			longSource := "[Container]\nImage=alpine:3.22\nExec=sleep infinity\nEnvironment=HOME_SERVER_DEPLOYER_REPLACE_REGRESSION=1\n"
			if err := os.WriteFile(f.tpl, []byte(longSource), 0o644); err != nil {
				t.Fatal(err)
			}
			install(t, f)
			if !strings.Contains(f.p.q["alpha-smoke.container"], "HOME_SERVER_DEPLOYER_REPLACE_REGRESSION") {
				t.Fatal("regression fixture did not install the longer source")
			}

			shortSource := "[Container]\nImage=alpine:3.22\nExec=sleep infinity\n"
			if err := os.WriteFile(f.tpl, []byte(shortSource), 0o644); err != nil {
				t.Fatal(err)
			}
			update(t, f)
			got := f.p.q["alpha-smoke.container"]
			if strings.Contains(got, "HOME_SERVER_DEPLOYER_REPLACE_REGRESSION") {
				t.Fatalf("removed trailing directive survived safe update on Podman %s: %q", version, got)
			}
			if len(f.p.ops) < 3 || f.p.ops[len(f.p.ops)-2] != "remove:alpha-smoke.container" || f.p.ops[len(f.p.ops)-1] != "install:alpha-smoke.container" {
				t.Fatalf("update did not use remove+install transaction on Podman %s: %v", version, f.p.ops)
			}
		})
	}
}

func TestRemoveKeepsDataAndDocs(t *testing.T) {
	f := fixture(t)
	install(t, f)
	d := filepath.Join(f.root, "alpha-smoke", "keep")
	if err := os.WriteFile(d, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := f.e.CreatePlan(context.Background(), model.PlanRequest{Operation: "remove", AppID: "alpha-smoke", InstanceID: "alpha-smoke"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.e.ExecutePlan(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(d); err != nil {
		t.Fatal("data removed")
	}
	if _, err = os.Stat(filepath.Join(f.docs, "alpha-smoke", "README.md")); err != nil {
		t.Fatal("docs removed")
	}
}
