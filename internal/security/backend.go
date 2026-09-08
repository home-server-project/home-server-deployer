package security

import "github.com/home-server-project/home-server-deployer/internal/model"

// Backend translates catalog storage/security intent into platform-specific
// behavior. Catalogs never contain arbitrary SELinux/AppArmor commands or labels.
type Backend interface {
	Capabilities() model.SecurityCapabilities
	MountSuffix(intent string, readOnly bool) (string, error)
}
