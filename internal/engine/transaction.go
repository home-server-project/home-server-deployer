package engine

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/home-server-project/home-server-deployer/internal/model"
	"github.com/home-server-project/home-server-deployer/internal/paths"
)

func (e *Engine) ExecutePlan(ctx context.Context, id string) (model.OperationResult, error) {
	p, err := e.state.GetPlan(id)
	if err != nil {
		return model.OperationResult{}, err
	}
	if time.Now().After(p.ExpiresAt) {
		return model.OperationResult{}, fmt.Errorf("plan expired")
	}
	if p.Digest != planDigest(p) {
		return model.OperationResult{}, fmt.Errorf("plan digest mismatch")
	}
	if len(p.Drift) > 0 {
		return model.OperationResult{}, fmt.Errorf("administrator drift detected; refusing %s", p.Operation)
	}
	if p.Operation != "install" {
		in, err := e.state.GetInstance(p.InstanceID)
		if err != nil {
			return model.OperationResult{}, err
		}
		if len(e.detectDrift(ctx, in)) > 0 {
			return model.OperationResult{}, fmt.Errorf("administrator drift appeared after planning; refusing %s", p.Operation)
		}
	}

	switch p.Operation {
	case "install", "update":
		err = e.upsert(ctx, p)
	case "remove":
		err = e.remove(ctx, p)
	}
	r := model.OperationResult{PlanID: p.ID, InstanceID: p.InstanceID, Operation: p.Operation, At: time.Now().UTC()}
	if err != nil {
		r.Status = "failed"
		r.Message = err.Error()
		return r, err
	}
	r.Status = "succeeded"
	_ = e.state.DeletePlan(p.ID)
	return r, nil
}

func (e *Engine) upsert(ctx context.Context, p model.Plan) error {
	app, _ := e.catalog.Get(p.AppID)
	for _, d := range p.Directories {
		if _, err := paths.EnsureDir(e.roots[d.RootID], d.RelativePath, 0o750); err != nil {
			return err
		}
	}

	if p.Operation == "update" {
		previous, err := e.state.GetInstance(p.InstanceID)
		if err != nil {
			return err
		}
		old := make(map[string]string, len(previous.Resources))
		for _, n := range sortedResourceNames(previous.Resources) {
			s, err := e.podman.GetQuadlet(ctx, n)
			if err != nil {
				return err
			}
			old[n] = s
		}
		return e.update(ctx, p, app, previous, old)
	}
	return e.install(ctx, p, app)
}

func (e *Engine) install(ctx context.Context, p model.Plan, app model.Catalog) error {
	installed := []string{}
	fail := func(cause error) error {
		if err := e.rollbackInstall(ctx, installed); err != nil {
			return fmt.Errorf("%v; rollback failed: %w", cause, err)
		}
		return cause
	}

	for _, r := range p.Resources {
		if err := e.podman.InstallQuadlet(ctx, r.Name, r.Content); err != nil {
			return fail(err)
		}
		installed = append(installed, r.Name)
	}
	if err := e.systemd.Reload(ctx); err != nil {
		return fail(err)
	}
	qm, err := e.quadletMap(ctx)
	if err != nil {
		return fail(err)
	}
	for _, r := range p.Resources {
		if !r.Start {
			continue
		}
		q, ok := qm[r.Name]
		if !ok || q.UnitName == "" {
			return fail(fmt.Errorf("generated unit for %s not found", r.Name))
		}
		if err := e.systemd.Start(ctx, q.UnitName); err != nil {
			return fail(err)
		}
		if err := e.verifyUnitActive(ctx, q.UnitName); err != nil {
			return fail(err)
		}
	}

	resources, err := e.captureResources(ctx, p.Resources)
	if err != nil {
		return fail(err)
	}
	now := time.Now().UTC()
	in := model.Instance{
		ID:             p.InstanceID,
		CatalogID:      p.AppID,
		CatalogVersion: p.CatalogVersion,
		Parameters:     p.Parameters,
		Resources:      resources,
		Directories:    p.Directories,
		InstalledAt:    now,
		UpdatedAt:      now,
		History:        []model.HistoryEntry{{At: now, Operation: "install", Result: "succeeded"}},
	}
	if err := e.state.SaveInstance(in); err != nil {
		return err
	}
	return e.docs.Write(in, app)
}

// update implements the Alpha 0 safe common-denominator Quadlet transaction.
// Native Podman replace is deliberately not used, including on Podman 6.x.
func (e *Engine) update(ctx context.Context, p model.Plan, app model.Catalog, previous model.Instance, old map[string]string) error {
	for _, unit := range sortedUnits(previous.Resources) {
		if err := e.systemd.Stop(ctx, unit); err != nil {
			return err
		}
	}

	removedOld := []string{}
	for _, name := range sortedResourceNames(previous.Resources) {
		if err := e.podman.RemoveQuadlet(ctx, name, false); err != nil {
			return e.rollbackRemovedOld(ctx, previous, old, removedOld, err)
		}
		removedOld = append(removedOld, name)
	}

	installedNew := []string{}
	for _, r := range p.Resources {
		if err := e.podman.InstallQuadlet(ctx, r.Name, r.Content); err != nil {
			return e.rollbackUpdatedSources(ctx, previous, old, installedNew, nil, err)
		}
		installedNew = append(installedNew, r.Name)
	}

	if err := e.systemd.Reload(ctx); err != nil {
		return e.rollbackUpdatedSources(ctx, previous, old, installedNew, nil, err)
	}
	qm, err := e.quadletMap(ctx)
	if err != nil {
		return e.rollbackUpdatedSources(ctx, previous, old, installedNew, nil, err)
	}

	started := []string{}
	for _, r := range p.Resources {
		if !r.Start {
			continue
		}
		q, ok := qm[r.Name]
		if !ok || q.UnitName == "" {
			return e.rollbackUpdatedSources(ctx, previous, old, installedNew, started, fmt.Errorf("generated unit for %s not found", r.Name))
		}
		// Track the unit before restart so rollback will stop it even if systemd
		// reports a failure after partially starting the new definition.
		started = append(started, q.UnitName)
		if err := e.systemd.Restart(ctx, q.UnitName); err != nil {
			return e.rollbackUpdatedSources(ctx, previous, old, installedNew, started, err)
		}
		if err := e.verifyUnitActive(ctx, q.UnitName); err != nil {
			return e.rollbackUpdatedSources(ctx, previous, old, installedNew, started, err)
		}
	}

	resources, err := e.captureResources(ctx, p.Resources)
	if err != nil {
		return e.rollbackUpdatedSources(ctx, previous, old, installedNew, started, err)
	}
	now := time.Now().UTC()
	in := model.Instance{
		ID:             p.InstanceID,
		CatalogID:      p.AppID,
		CatalogVersion: p.CatalogVersion,
		Parameters:     p.Parameters,
		Resources:      resources,
		Directories:    p.Directories,
		InstalledAt:    previous.InstalledAt,
		UpdatedAt:      now,
		History:        append(previous.History, model.HistoryEntry{At: now, Operation: "update", Result: "succeeded"}),
	}
	if err := e.state.SaveInstance(in); err != nil {
		return err
	}
	return e.docs.Write(in, app)
}

func (e *Engine) rollbackInstall(ctx context.Context, names []string) error {
	var errs []error
	for i := len(names) - 1; i >= 0; i-- {
		if err := e.podman.RemoveQuadlet(ctx, names[i], true); err != nil {
			errs = append(errs, err)
		}
	}
	if err := e.systemd.Reload(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// rollbackRemovedOld handles failure while removing the previous source set.
// Only files already removed are restored, so still-present old files are never
// overwritten and native replace is never needed.
func (e *Engine) rollbackRemovedOld(ctx context.Context, previous model.Instance, old map[string]string, removed []string, cause error) error {
	var errs []error
	for _, name := range removed {
		if err := e.podman.InstallQuadlet(ctx, name, old[name]); err != nil {
			errs = append(errs, err)
		}
	}
	if err := e.systemd.Reload(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := e.restartPrevious(ctx, previous); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v; rollback failed: %w", cause, errors.Join(errs...))
	}
	return cause
}

// rollbackUpdatedSources removes only sources successfully installed by this
// transaction, reinstalls the snapshotted previous sources normally, reloads
// systemd once, and verifies the previous service state.
func (e *Engine) rollbackUpdatedSources(ctx context.Context, previous model.Instance, old map[string]string, installed, started []string, cause error) error {
	var errs []error
	for i := len(started) - 1; i >= 0; i-- {
		if err := e.systemd.Stop(ctx, started[i]); err != nil {
			errs = append(errs, err)
		}
	}
	for i := len(installed) - 1; i >= 0; i-- {
		if err := e.podman.RemoveQuadlet(ctx, installed[i], false); err != nil {
			errs = append(errs, err)
		}
	}
	for _, name := range sortedStringMapKeys(old) {
		if err := e.podman.InstallQuadlet(ctx, name, old[name]); err != nil {
			errs = append(errs, err)
		}
	}
	if err := e.systemd.Reload(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := e.restartPrevious(ctx, previous); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v; rollback failed: %w", cause, errors.Join(errs...))
	}
	return cause
}

func (e *Engine) restartPrevious(ctx context.Context, previous model.Instance) error {
	var errs []error
	for _, unit := range sortedUnits(previous.Resources) {
		if err := e.systemd.Restart(ctx, unit); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := e.verifyUnitActive(ctx, unit); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (e *Engine) verifyUnitActive(ctx context.Context, unit string) error {
	status, err := e.systemd.Status(ctx, unit)
	if err != nil {
		return err
	}
	if !strings.Contains(status, "/active/") {
		return fmt.Errorf("systemd unit %s failed verification: %s", unit, status)
	}
	return nil
}

func (e *Engine) captureResources(ctx context.Context, rendered []model.RenderedResource) (map[string]model.ResourceState, error) {
	qm, err := e.quadletMap(ctx)
	if err != nil {
		return nil, err
	}
	resources := map[string]model.ResourceState{}
	for _, r := range rendered {
		s, err := e.podman.GetQuadlet(ctx, r.Name)
		if err != nil {
			return nil, err
		}
		resources[r.Name] = model.ResourceState{Name: r.Name, UnitName: qm[r.Name].UnitName, SHA256: hash(s), Source: s}
	}
	return resources, nil
}

func (e *Engine) remove(ctx context.Context, p model.Plan) error {
	in, err := e.state.GetInstance(p.InstanceID)
	if err != nil {
		return err
	}
	app, _ := e.catalog.Get(in.CatalogID)
	old := map[string]string{}
	for n := range in.Resources {
		s, err := e.podman.GetQuadlet(ctx, n)
		if err != nil {
			return err
		}
		old[n] = s
	}
	for _, u := range sortedUnits(in.Resources) {
		if err := e.systemd.Stop(ctx, u); err != nil {
			return err
		}
	}
	removed := []string{}
	rollback := func(c error) error {
		x := e.rollbackRemove(ctx, in, old, removed)
		if x != nil {
			return fmt.Errorf("%v; rollback failed: %w", c, x)
		}
		return c
	}
	for _, n := range sortedResourceNames(in.Resources) {
		if err := e.podman.RemoveQuadlet(ctx, n, false); err != nil {
			return rollback(err)
		}
		removed = append(removed, n)
	}
	if err := e.systemd.Reload(ctx); err != nil {
		return rollback(err)
	}
	now := time.Now().UTC()
	in.RemovedAt = &now
	in.UpdatedAt = now
	in.History = append(in.History, model.HistoryEntry{At: now, Operation: "remove", Result: "succeeded", Note: "application data preserved"})
	if err := e.state.SaveInstance(in); err != nil {
		return err
	}
	return e.docs.Write(in, app)
}

func (e *Engine) rollbackRemove(ctx context.Context, in model.Instance, old map[string]string, removed []string) error {
	var errs []error
	for _, n := range removed {
		if err := e.podman.InstallQuadlet(ctx, n, old[n]); err != nil {
			errs = append(errs, err)
		}
	}
	if err := e.systemd.Reload(ctx); err != nil {
		errs = append(errs, err)
	}
	for _, u := range sortedUnits(in.Resources) {
		if err := e.systemd.Start(ctx, u); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := e.verifyUnitActive(ctx, u); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (e *Engine) Status(ctx context.Context, id string) (model.InstanceStatus, error) {
	in, err := e.state.GetInstance(id)
	if err != nil {
		return model.InstanceStatus{}, err
	}
	qs, err := e.podman.ListQuadlets(ctx)
	if err != nil {
		return model.InstanceStatus{}, err
	}
	cs, err := e.podman.ListContainers(ctx)
	if err != nil {
		return model.InstanceStatus{}, err
	}
	qm := map[string]model.Quadlet{}
	for _, q := range qs {
		qm[q.Name] = q
	}
	cm := map[string]model.RuntimeContainer{}
	for _, c := range cs {
		if c.SystemdUnit != "" {
			cm[c.SystemdUnit] = c
		}
	}
	o := model.InstanceStatus{Instance: in}
	for _, n := range sortedResourceNames(in.Resources) {
		x := in.Resources[n]
		r := model.ResourceStatus{Name: n, UnitName: x.UnitName}
		if q, ok := qm[n]; ok {
			r.QuadletStatus = q.Status
			if r.UnitName == "" {
				r.UnitName = q.UnitName
			}
		}
		if r.UnitName != "" {
			r.SystemdStatus, _ = e.systemd.Status(ctx, r.UnitName)
			if c, ok := cm[r.UnitName]; ok {
				cc := c
				r.Runtime = &cc
			}
		}
		o.Resources = append(o.Resources, r)
	}
	return o, nil
}

func (e *Engine) Lifecycle(ctx context.Context, id, action string) error {
	in, err := e.state.GetInstance(id)
	if err != nil {
		return err
	}
	if in.RemovedAt != nil {
		return fmt.Errorf("instance is removed")
	}
	for _, u := range sortedUnits(in.Resources) {
		switch action {
		case "start":
			err = e.systemd.Start(ctx, u)
		case "stop":
			err = e.systemd.Stop(ctx, u)
		case "restart":
			err = e.systemd.Restart(ctx, u)
		default:
			return fmt.Errorf("unsupported lifecycle action")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) Logs(ctx context.Context, id string, tail int) (map[string]string, error) {
	in, err := e.state.GetInstance(id)
	if err != nil {
		return nil, err
	}
	units := map[string]bool{}
	for _, r := range in.Resources {
		units[r.UnitName] = true
	}
	cs, err := e.podman.ListContainers(ctx)
	if err != nil {
		return nil, err
	}
	o := map[string]string{}
	for _, c := range cs {
		if units[c.SystemdUnit] {
			s, err := e.podman.Logs(ctx, c.ID, tail)
			if err != nil {
				return nil, err
			}
			o[c.Name] = s
		}
	}
	return o, nil
}

func (e *Engine) Documentation(id, name string) ([]byte, error) {
	if _, err := e.state.GetInstance(id); err != nil {
		return nil, err
	}
	return e.docs.Read(id, name)
}

func (e *Engine) BackupContract(id string) (model.BackupContract, error) {
	in, err := e.state.GetInstance(id)
	if err != nil {
		return model.BackupContract{}, err
	}
	app, ok := e.catalog.Get(in.CatalogID)
	if !ok {
		return model.BackupContract{}, fmt.Errorf("catalog missing")
	}
	return app.Spec.Backup, nil
}

func (e *Engine) quadletMap(ctx context.Context) (map[string]model.Quadlet, error) {
	qs, err := e.podman.ListQuadlets(ctx)
	if err != nil {
		return nil, err
	}
	m := map[string]model.Quadlet{}
	for _, q := range qs {
		m[q.Name] = q
	}
	return m, nil
}

func sortedResourceNames(m map[string]model.ResourceState) []string {
	o := make([]string, 0, len(m))
	for n := range m {
		o = append(o, n)
	}
	sort.Strings(o)
	return o
}

func sortedStringMapKeys(m map[string]string) []string {
	o := make([]string, 0, len(m))
	for n := range m {
		o = append(o, n)
	}
	sort.Strings(o)
	return o
}

func sortedUnits(m map[string]model.ResourceState) []string {
	o := []string{}
	for _, r := range m {
		if r.UnitName != "" {
			o = append(o, r.UnitName)
		}
	}
	sort.Strings(o)
	return o
}

func planDigest(p model.Plan) string {
	p.Digest = ""
	b, _ := json.Marshal(p)
	return hash(string(b))
}

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func newID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func safeID(s string) bool {
	if s == "" || len(s) > 63 || s[0] == '-' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
