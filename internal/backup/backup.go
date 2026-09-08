package backup

import (
	"errors"
	"fmt"

	"github.com/home-server-project/home-server-deployer/internal/model"
)

var ErrExecutionNotImplemented = errors.New("backup execution is not implemented in Alpha 0")

// Provider is the future application-aware backup execution boundary. Catalogs
// select named providers; they never provide arbitrary pre/post shell commands.
type Provider interface {
	Name() string
	Validate(model.BackupContract) error
}

func ValidateContract(c model.BackupContract) error {
	switch c.Strategy {
	case "none", "filesystem", "provider":
	default:
		return fmt.Errorf("unsupported backup strategy %q", c.Strategy)
	}
	switch c.Consistency {
	case "none", "online", "stop-required", "provider-managed":
	default:
		return fmt.Errorf("unsupported backup consistency %q", c.Consistency)
	}
	if c.Strategy == "provider" && c.Provider == "" {
		return fmt.Errorf("provider backup strategy requires a named provider")
	}
	if c.Strategy != "provider" && c.Provider != "" {
		return fmt.Errorf("backup provider is only valid with provider strategy")
	}
	return nil
}
