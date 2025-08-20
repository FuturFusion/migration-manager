package config

import (
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func LoadConfig() (*api.SystemConfig, error) {
	contents, err := os.ReadFile(util.VarPath("config.yml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &api.SystemConfig{}, nil
		}

		return nil, err
	}

	c := &api.SystemConfig{}
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

	err = os.WriteFile(util.VarPath("server.crt"), []byte(c.ServerCertificate.Cert), 0o664)
	if err != nil {
		return err
	}

	if c.ServerCertificate.CA != "" {
		err = os.WriteFile(util.VarPath("server.ca"), []byte(c.ServerCertificate.CA), 0o664)
		if err != nil {
			return err
		}
	}

	err = os.WriteFile(util.VarPath("server.key"), []byte(c.ServerCertificate.Key), 0o600)
	if err != nil {
		return err
	}

	return os.WriteFile(util.VarPath("config.yml"), contents, 0o644)
}

func Validate(s api.SystemConfig) error {
	if s.RestServerPort < 1 || s.RestServerPort > 0xffff {
		return fmt.Errorf("Server port %q is invalid", s.RestServerPort)
	}

	if s.RestServerIPAddr != "" {
		ip := net.ParseIP(s.RestServerIPAddr)
		if ip == nil {
			return fmt.Errorf("Server IP address %q is invalid", s.RestServerIPAddr)
		}
	}

	if s.RestWorkerEndpoint != "" {
		endpoint, err := url.ParseRequestURI(s.RestWorkerEndpoint)
		if err != nil {
			return fmt.Errorf("Failed to parse worker endpoint %q: %w", s.RestWorkerEndpoint, err)
		}

		if endpoint.Scheme == "" {
			return fmt.Errorf("Failed to determine scheme for worker endpoint %q", s.RestWorkerEndpoint)
		}

		if endpoint.Hostname() == "" {
			return fmt.Errorf("Failed to determine host for worker endpoint %q", s.RestWorkerEndpoint)
		}

		if endpoint.Port() == "" {
			return fmt.Errorf("Failed to determine port for worker endpoint %q", s.RestWorkerEndpoint)
		}

		if endpoint.Path != "" {
			return fmt.Errorf("Worker endpoint %q contains path", s.RestWorkerEndpoint)
		}

		if endpoint.RawQuery != "" {
			return fmt.Errorf("Worker endpoint %q contains query", s.RestWorkerEndpoint)
		}
	}

	certBlock, _ := pem.Decode([]byte(s.ServerCertificate.Cert))
	if certBlock == nil {
		return fmt.Errorf("Certificate must be base64 encoded PEM certificate")
	}

	keyBlock, _ := pem.Decode([]byte(s.ServerCertificate.Key))
	if keyBlock == nil {
		return fmt.Errorf("Key must be base64 encoded PEM key")
	}

	if s.ServerCertificate.CA != "" {
		caBlock, _ := pem.Decode([]byte(s.ServerCertificate.CA))
		if caBlock == nil {
			return fmt.Errorf("CA must be base64 encoded PEM key")
		}
	}

	return nil
}
