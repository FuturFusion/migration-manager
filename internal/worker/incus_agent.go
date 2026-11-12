package worker

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// configureIncusAgent writes the configured agent settings into the guest.
func configureIncusAgent(guestPath string, path string, config *api.IncusAgentConfig) error {
	if config == nil || config.Features == nil {
		return nil
	}

	// Confine guest-controlled symlinks to the mounted guest filesystem.
	guest, err := os.OpenRoot(guestPath)
	if err != nil {
		return fmt.Errorf("Failed to open guest filesystem: %w", err)
	}

	defer func() { _ = guest.Close() }()

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("Failed to encode Incus agent configuration: %w", err)
	}

	err = guest.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return fmt.Errorf("Failed to create Incus agent configuration directory: %w", err)
	}

	err = guest.WriteFile(path, data, 0o644)
	if err != nil {
		return fmt.Errorf("Failed to write Incus agent configuration: %w", err)
	}

	return nil
}
