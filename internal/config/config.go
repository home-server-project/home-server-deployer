package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultPath = "/etc/home-server-deployer/config.json"

type Root struct {
	ID        string `json:"id"`
	HostPath  string `json:"hostPath"`
	AgentPath string `json:"agentPath"`
}

type Config struct {
	ListenSocket      string `json:"listenSocket"`
	PodmanSocket      string `json:"podmanSocket"`
	StateRoot         string `json:"stateRoot"`
	DocumentationRoot string `json:"documentationRoot"`
	CatalogRoot       string `json:"catalogRoot"`
	ApprovedRoots     []Root `json:"approvedRoots"`
	TestVMMarker      string `json:"testVmMarker"`
}

func Default() Config {
	return Config{
		ListenSocket:      "/run/home-server-deployer/agent.sock",
		PodmanSocket:      "/run/podman/podman.sock",
		StateRoot:         "/var/lib/home-server-deployer",
		DocumentationRoot: "/var/lib/home-server-deployer/docs",
		CatalogRoot:       "/usr/share/home-server-deployer/catalog/apps",
		ApprovedRoots: []Root{{
			ID:        "application-data",
			HostPath:  "/var/lib/home-server-apps",
			AgentPath: "/approved/application-data",
		}},
		TestVMMarker: "/etc/home-server-deployer/test-vm",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	for name, p := range map[string]string{
		"listenSocket":      c.ListenSocket,
		"podmanSocket":      c.PodmanSocket,
		"stateRoot":         c.StateRoot,
		"documentationRoot": c.DocumentationRoot,
		"catalogRoot":       c.CatalogRoot,
	} {
		if p == "" || !filepath.IsAbs(p) {
			return fmt.Errorf("%s must be an absolute path", name)
		}
	}
	seen := map[string]bool{}
	for _, r := range c.ApprovedRoots {
		if r.ID == "" || seen[r.ID] {
			return fmt.Errorf("approved root IDs must be unique and non-empty")
		}
		seen[r.ID] = true
		if !filepath.IsAbs(r.HostPath) || !filepath.IsAbs(r.AgentPath) {
			return fmt.Errorf("approved root %q paths must be absolute", r.ID)
		}
	}
	return nil
}
