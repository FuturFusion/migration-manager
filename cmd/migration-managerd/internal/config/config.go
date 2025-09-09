package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/ports"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func LoadConfig() (*api.SystemConfig, error) {
	// Set the default port for a fresh config.
	c := &api.SystemConfig{
		Network: api.ConfigNetwork{
			Port: ports.HTTPSDefaultPort,
		},
	}

	contents, err := os.ReadFile(util.VarPath("config.yml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}

		return nil, err
	}

	err = yaml.Unmarshal(contents, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func SaveConfig(c api.SystemConfig) error {
	contents, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(util.VarPath("config.yml"), contents, 0o644)
}

func Validate(s api.SystemConfig) error {
	if s.Network.Port < 1 || s.Network.Port > 0xffff {
		return fmt.Errorf("Server port %d is invalid", s.Network.Port)
	}

	if s.Network.Address != "" {
		ip := net.ParseIP(s.Network.Address)
		if ip == nil {
			return fmt.Errorf("Server IP address %q is invalid", s.Network.Address)
		}
	}

	if s.Network.WorkerEndpoint != "" {
		endpoint, err := url.ParseRequestURI(s.Network.WorkerEndpoint)
		if err != nil {
			return fmt.Errorf("Failed to parse worker endpoint %q: %w", s.Network.WorkerEndpoint, err)
		}

		if endpoint.Scheme == "" {
			return fmt.Errorf("Failed to determine scheme for worker endpoint %q", s.Network.WorkerEndpoint)
		}

		if endpoint.Hostname() == "" {
			return fmt.Errorf("Failed to determine host for worker endpoint %q", s.Network.WorkerEndpoint)
		}

		if endpoint.Port() == "" {
			return fmt.Errorf("Failed to determine port for worker endpoint %q", s.Network.WorkerEndpoint)
		}

		if endpoint.Path != "" {
			return fmt.Errorf("Worker endpoint %q contains path", s.Network.WorkerEndpoint)
		}

		if endpoint.RawQuery != "" {
			return fmt.Errorf("Worker endpoint %q contains query", s.Network.WorkerEndpoint)
		}
	}

	return nil
}
