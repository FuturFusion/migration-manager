package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/ports"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func LoadConfig() (*api.SystemConfig, error) {
	// Set the default port for a fresh config.
	c := &api.SystemConfig{}
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

func SetDefaults(s api.SystemConfig) (*api.SystemConfig, error) {
	newCfg := s
	parseIP := func(addr string) (net.IP, error) {
		if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") && len(addr) > 2 {
			addr = addr[1 : len(addr)-1]
		}

		ip := net.ParseIP(addr)
		if ip == nil {
			return nil, fmt.Errorf("%q is not a valid IP address", addr)
		}

		return ip, nil
	}

	if s.Network.Address != "" {
		host, port, err := net.SplitHostPort(s.Network.Address)
		if err != nil {
			ip, err := parseIP(s.Network.Address)
			if err != nil {
				return nil, err
			}

			newCfg.Network.Address = net.JoinHostPort(ip.String(), ports.HTTPSDefaultPort)
		} else {
			if host == "" {
				host = "::"
			}

			ip, err := parseIP(host)
			if err != nil {
				return nil, err
			}

			if port == "" {
				newCfg.Network.Address = net.JoinHostPort(ip.String(), ports.HTTPSDefaultPort)
			}
		}
	}

	return &newCfg, nil
}

func Validate(newCfg api.SystemConfig, oldCfg api.SystemConfig) error {
	if oldCfg.Network.Address != "" && newCfg.Network.Address == "" {
		return fmt.Errorf("Network address %q cannot be unset", oldCfg.Network.Address)
	}

	if len(oldCfg.Security.TrustedTLSClientCertFingerprints) > 0 && len(newCfg.Security.TrustedTLSClientCertFingerprints) == 0 {
		return fmt.Errorf("Last trusted TLS client certificate fingerprint cannot be removed")
	}

	if newCfg.Network.Address != "" {
		host, port, err := net.SplitHostPort(newCfg.Network.Address)
		if err != nil {
			return fmt.Errorf("Server IP address %q is invalid", newCfg.Network.Address)
		}

		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("%q is not a valid IP address", host)
		}

		portInt, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("Server port %q is invalid: %w", port, err)
		}

		if portInt < 1 || portInt > 0xffff {
			return fmt.Errorf("Server port %d is invalid", portInt)
		}
	}

	if newCfg.Network.WorkerEndpoint != "" {
		endpoint, err := url.ParseRequestURI(newCfg.Network.WorkerEndpoint)
		if err != nil {
			return fmt.Errorf("Failed to parse worker endpoint %q: %w", newCfg.Network.WorkerEndpoint, err)
		}

		if endpoint.Scheme == "" {
			return fmt.Errorf("Failed to determine scheme for worker endpoint %q", newCfg.Network.WorkerEndpoint)
		}

		if endpoint.Hostname() == "" {
			return fmt.Errorf("Failed to determine host for worker endpoint %q", newCfg.Network.WorkerEndpoint)
		}

		if endpoint.Port() == "" {
			return fmt.Errorf("Failed to determine port for worker endpoint %q", newCfg.Network.WorkerEndpoint)
		}

		if endpoint.Path != "" {
			return fmt.Errorf("Worker endpoint %q contains path", newCfg.Network.WorkerEndpoint)
		}

		if endpoint.RawQuery != "" {
			return fmt.Errorf("Worker endpoint %q contains query", newCfg.Network.WorkerEndpoint)
		}
	}

	return nil
}
