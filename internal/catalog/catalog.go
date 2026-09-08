package catalog

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/home-server-project/home-server-deployer/internal/backup"
	"github.com/home-server-project/home-server-deployer/internal/model"
	"gopkg.in/yaml.v3"
)

var safeID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type Store struct {
	root string
	apps map[string]model.Catalog
}

func NewStore(root string, apps []model.Catalog) (*Store, error) {
	s := &Store{root: root, apps: map[string]model.Catalog{}}
	for _, app := range apps {
		if err := Validate(app); err != nil {
			return nil, err
		}
		if _, exists := s.apps[app.Metadata.ID]; exists {
			return nil, fmt.Errorf("duplicate catalog ID %q", app.Metadata.ID)
		}
		s.apps[app.Metadata.ID] = app
	}
	return s, nil
}

func Load(root string) (*Store, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read catalog root: %w", err)
	}
	s := &Store{root: root, apps: map[string]model.Catalog{}}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest := filepath.Join(root, entry.Name(), "app.yaml")
		b, err := os.ReadFile(manifest)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var app model.Catalog
		dec := yaml.NewDecoder(bytes.NewReader(b))
		dec.KnownFields(true)
		if err := dec.Decode(&app); err != nil {
			return nil, fmt.Errorf("decode %s: %w", manifest, err)
		}
		if err := Validate(app); err != nil {
			return nil, fmt.Errorf("validate %s: %w", manifest, err)
		}
		if _, exists := s.apps[app.Metadata.ID]; exists {
			return nil, fmt.Errorf("duplicate catalog ID %q", app.Metadata.ID)
		}
		s.apps[app.Metadata.ID] = app
	}
	return s, nil
}

func Validate(app model.Catalog) error {
	if app.APIVersion != model.CatalogAPIVersion {
		return fmt.Errorf("unsupported apiVersion %q", app.APIVersion)
	}
	if app.Kind != "Application" {
		return fmt.Errorf("kind must be Application")
	}
	if !safeID.MatchString(app.Metadata.ID) {
		return fmt.Errorf("invalid metadata.id %q", app.Metadata.ID)
	}
	if app.Metadata.Name == "" || app.Metadata.Version == "" {
		return fmt.Errorf("metadata.name and metadata.version are required")
	}
	inputNames := map[string]bool{}
	for _, in := range app.Spec.Inputs {
		if !safeID.MatchString(in.Name) {
			return fmt.Errorf("invalid input name %q", in.Name)
		}
		if inputNames[in.Name] {
			return fmt.Errorf("duplicate input %q", in.Name)
		}
		inputNames[in.Name] = true
		switch in.Type {
		case "string", "path", "port", "integer":
		default:
			return fmt.Errorf("unsupported input type %q", in.Type)
		}
	}
	for _, d := range app.Spec.Directories {
		if d.Name == "" || d.RootID == "" || !inputNames[d.SubpathInput] || d.ContainerPath == "" {
			return fmt.Errorf("invalid directory %q", d.Name)
		}
		switch d.SecurityIntent {
		case "private", "shared", "readonly-media", "cache", "database":
		default:
			return fmt.Errorf("unsupported securityIntent %q", d.SecurityIntent)
		}
	}
	if len(app.Spec.Resources) == 0 {
		return fmt.Errorf("at least one resource is required")
	}
	seenResources := map[string]bool{}
	for _, r := range app.Spec.Resources {
		if !validQuadletName(r.Name) {
			return fmt.Errorf("invalid Quadlet resource name %q", r.Name)
		}
		if r.Template == "" || filepath.Base(r.Template) != r.Template {
			return fmt.Errorf("invalid resource template %q", r.Template)
		}
		if seenResources[r.Name] {
			return fmt.Errorf("duplicate resource %q", r.Name)
		}
		seenResources[r.Name] = true
	}
	if app.Spec.MinPodmanVersion == "" {
		return fmt.Errorf("spec.minPodmanVersion is required")
	}
	switch app.Spec.Update.Mode {
	case "manual-only", "track-latest", "track-lts", "track-stable", "pinned-version", "pinned-digest", "catalog-recommended":
	default:
		return fmt.Errorf("unsupported update mode %q", app.Spec.Update.Mode)
	}
	switch app.Spec.Health.Strategy {
	case "runtime", "podman-health", "application-provider":
	default:
		return fmt.Errorf("unsupported health strategy %q", app.Spec.Health.Strategy)
	}
	if app.Spec.Backup.Strategy == "" {
		return fmt.Errorf("backup.strategy is required")
	}
	if err := backup.ValidateContract(app.Spec.Backup); err != nil {
		return err
	}
	return nil
}

func validQuadletName(name string) bool {
	if filepath.Base(name) != name || strings.ContainsAny(name, "\r\n\x00") {
		return false
	}
	for _, ext := range []string{".container", ".volume", ".network", ".image", ".build", ".pod", ".artifact"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func (s *Store) List() []model.Catalog {
	out := make([]model.Catalog, 0, len(s.apps))
	for _, app := range s.apps {
		out = append(out, app)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Metadata.ID < out[j].Metadata.ID })
	return out
}

func (s *Store) Get(id string) (model.Catalog, bool) { a, ok := s.apps[id]; return a, ok }

func (s *Store) Template(appID, name string) (string, error) {
	app, ok := s.apps[appID]
	if !ok {
		return "", fmt.Errorf("unknown application %q", appID)
	}
	allowed := false
	for _, r := range app.Spec.Resources {
		if r.Template == name {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("template %q is not declared by %s", name, appID)
	}
	b, err := os.ReadFile(filepath.Join(s.root, appID, name))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ResolveParameters(app model.Catalog, supplied map[string]string) (map[string]string, error) {
	out := map[string]string{}
	known := map[string]model.InputSpec{}
	for _, in := range app.Spec.Inputs {
		known[in.Name] = in
		out[in.Name] = in.Default
	}
	for k, v := range supplied {
		in, ok := known[k]
		if !ok {
			return nil, fmt.Errorf("unknown parameter %q", k)
		}
		if strings.ContainsAny(v, "\r\n\x00") {
			return nil, fmt.Errorf("parameter %q contains forbidden control characters", k)
		}
		if err := validateInput(in, v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	for _, in := range app.Spec.Inputs {
		v := out[in.Name]
		if in.Required && v == "" {
			return nil, fmt.Errorf("parameter %q is required", in.Name)
		}
		if v != "" {
			if err := validateInput(in, v); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func validateInput(in model.InputSpec, v string) error {
	switch in.Type {
	case "path":
		if v == "" || filepath.IsAbs(v) || filepath.Clean(v) != v || v == "." || strings.HasPrefix(v, "../") || strings.Contains(v, "/../") {
			return fmt.Errorf("parameter %q must be a clean relative path", in.Name)
		}
	case "port":
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("parameter %q must be a TCP/UDP port", in.Name)
		}
	case "integer":
		if _, err := strconv.Atoi(v); err != nil {
			return fmt.Errorf("parameter %q must be an integer", in.Name)
		}
	case "string":
		if len(v) > 4096 {
			return fmt.Errorf("parameter %q is too long", in.Name)
		}
	}
	return nil
}
