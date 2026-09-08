package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/home-server-project/home-server-deployer/internal/model"
)

func TestLoadRejectsUnknownFields(t *testing.T) {
	d := t.TempDir()
	appDir := filepath.Join(d, "x")
	_ = os.Mkdir(appDir, 0755)
	content := `apiVersion: deployer.home-server-project.io/v1alpha1
kind: Application
metadata:
  id: x
  name: X
  version: "1"
  nope: true
spec:
  minPodmanVersion: 5.8.4
  inputs: []
  directories: []
  resources:
    - name: x.container
      template: x.container.tmpl
      start: true
  update:
    mode: manual-only
  health:
    strategy: runtime
  backup:
    strategy: none
    consistency: none
    includes: []
`
	if err := os.WriteFile(filepath.Join(appDir, "app.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("expected strict YAML decode error")
	}
}

func TestResolveParametersRejectsTraversal(t *testing.T) {
	app := model.Catalog{Spec: model.AppSpec{Inputs: []model.InputSpec{{Name: "data-subpath", Type: "path", Required: true}}}}
	if _, err := ResolveParameters(app, map[string]string{"data-subpath": "../etc"}); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
