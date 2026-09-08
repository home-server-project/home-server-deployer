package selinux

import (
	"fmt"
	"os"
	"strings"

	"github.com/home-server-project/home-server-deployer/internal/model"
)

type Backend struct {
	enforcePath string
}

func New() *Backend                           { return &Backend{enforcePath: "/sys/fs/selinux/enforce"} }
func NewWithEnforcePath(path string) *Backend { return &Backend{enforcePath: path} }

func (b *Backend) Capabilities() model.SecurityCapabilities {
	cap := model.SecurityCapabilities{Backend: "selinux"}
	data, err := os.ReadFile(b.enforcePath)
	if err != nil {
		return cap
	}
	cap.Enabled = true
	cap.Enforcing = strings.TrimSpace(string(data)) == "1"
	return cap
}

// MountSuffix translates catalog intent to narrow Podman bind-mount semantics.
// It never accepts arbitrary SELinux labels or commands.
func (b *Backend) MountSuffix(intent string, readOnly bool) (string, error) {
	var opts []string
	if readOnly {
		opts = append(opts, "ro")
	}
	switch intent {
	case "private", "cache", "database":
		opts = append(opts, "Z")
	case "shared":
		opts = append(opts, "z")
	case "readonly-media":
		if !readOnly {
			return "", fmt.Errorf("readonly-media intent requires readOnly=true")
		}
	default:
		return "", fmt.Errorf("unsupported SELinux intent %q", intent)
	}
	return strings.Join(opts, ","), nil
}
