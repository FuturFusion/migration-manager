package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slog"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/acme"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/ports"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func LoadConfig(configPath string) (*api.SystemConfig, error) {
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	c := &api.SystemConfig{}
	err = yaml.Unmarshal(contents, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func InitConfig(dir string) (*api.SystemConfig, error) {
	c, err := LoadConfig(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if err != nil {
		return &api.SystemConfig{}, nil
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
				port = ports.HTTPSDefaultPort
			}

			newCfg.Network.Address = net.JoinHostPort(ip.String(), port)
		}
	}

	newCfg.Security.ACME = acme.SetACMEDefaults(newCfg.Security.ACME)

	if newCfg.Settings.SyncInterval == "" {
		newCfg.Settings.SyncInterval = (10 * time.Minute).Truncate(time.Minute).String()
	}

	if newCfg.Settings.LogLevel == "" {
		newCfg.Settings.LogLevel = slog.LevelWarn.String()
	} else {
		newCfg.Settings.LogLevel = strings.ToUpper(s.Settings.LogLevel)
	}

	for i := range newCfg.Settings.LogTargets {
		newCfg.Settings.LogTargets[i] = logger.WebhookDefaultConfig(newCfg.Settings.LogTargets[i])
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

	err := acme.ValidateACMEConfig(newCfg.Security.ACME)
	if err != nil {
		return err
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

		if endpoint.Port() != "" {
			portInt, err := strconv.Atoi(endpoint.Port())
			if err != nil {
				return fmt.Errorf("Worker endpoint port %q is invalid: %w", endpoint.Port(), err)
			}

			if portInt < 1 || portInt > 0xffff {
				return fmt.Errorf("Worker endpoint port %d is invalid", portInt)
			}
		}

		if endpoint.Path != "" {
			return fmt.Errorf("Worker endpoint %q contains path", newCfg.Network.WorkerEndpoint)
		}

		if endpoint.RawQuery != "" {
			return fmt.Errorf("Worker endpoint %q contains query", newCfg.Network.WorkerEndpoint)
		}
	}

	syncInterval, err := time.ParseDuration(newCfg.Settings.SyncInterval)
	if err != nil {
		return fmt.Errorf("Invalid sync interval duration %q: %w", newCfg.Settings.SyncInterval, err)
	}

	if syncInterval <= time.Second {
		return fmt.Errorf("Sync interval %q is too frequent, must be at least 1s", newCfg.Settings.SyncInterval)
	}

	err = logger.ValidateLevel(newCfg.Settings.LogLevel)
	if err != nil {
		return err
	}

	loggerNames := map[string]bool{}
	for _, cfg := range newCfg.Settings.LogTargets {
		err := logger.WebhookValidateConfig(cfg)
		if err != nil {
			return err
		}

		if loggerNames[cfg.Name] {
			return fmt.Errorf("Log target %q is defined more than once", cfg.Name)
		}

		loggerNames[cfg.Name] = true
	}

	return nil
}
