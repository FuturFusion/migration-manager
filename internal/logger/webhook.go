package logger

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	incustls "github.com/lxc/incus/v6/shared/tls"
	"github.com/lxc/incus/v6/shared/validate"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type webhookLog struct {
	name   string
	level  slog.Level
	scopes []api.LogScope

	client   *http.Client
	address  string
	username string
	password string
	retry    int

	retryTimeout time.Duration
}

func WebhookDefaultConfig(cfg api.SystemSettingsLog) api.SystemSettingsLog {
	newCfg := cfg

	if cfg.Level == "" {
		newCfg.Level = slog.LevelWarn.String()
	} else {
		newCfg.Level = strings.ToUpper(cfg.Level)
	}

	if cfg.RetryCount == 0 {
		newCfg.RetryCount = 3
	}

	if cfg.RetryTimeout == "" {
		newCfg.RetryTimeout = (time.Second * 10).String()
	}

	if len(cfg.Scopes) == 0 {
		newCfg.Scopes = []api.LogScope{api.LogScopeLifecycle, api.LogScopeLogging}
	}

	return newCfg
}

func WebhookValidateConfig(cfg api.SystemSettingsLog) error {
	if cfg.Type != api.LogTypeWebhook {
		return fmt.Errorf("Log type (%q) is not %q", cfg.Type, api.LogTypeWebhook)
	}

	if cfg.RetryCount <= 0 {
		return fmt.Errorf("Log retry count (%d) must be greater than 0", cfg.RetryCount)
	}

	if len(cfg.Scopes) == 0 {
		return fmt.Errorf("Log scopes cannot be empty")
	}

	for _, scope := range cfg.Scopes {
		switch scope {
		case api.LogScopeLifecycle:
		case api.LogScopeLogging:
		default:
			return fmt.Errorf("Unknown log scope %q", scope)
		}
	}

	err := validate.IsAPIName(cfg.Name, false)
	if err != nil {
		return fmt.Errorf("Logger name %q is invalid: %w", cfg.Name, err)
	}

	err = ValidateLevel(cfg.Level)
	if err != nil {
		return fmt.Errorf("Logger %q level %q is invalid: %w", cfg.Name, cfg.Level, err)
	}

	endpoint, err := url.ParseRequestURI(cfg.Address)
	if err != nil {
		return fmt.Errorf("Failed to parse %q %q logger address %q: %w", cfg.Name, cfg.Type, cfg.Address, err)
	}

	if endpoint.Scheme == "" {
		return fmt.Errorf("Failed to determine scheme for %q %q logger address %q", cfg.Name, cfg.Type, cfg.Address)
	}

	if endpoint.Hostname() == "" {
		return fmt.Errorf("Failed to determine host for %q %q logger address %q", cfg.Name, cfg.Type, cfg.Address)
	}

	if endpoint.Port() != "" {
		portInt, err := strconv.Atoi(endpoint.Port())
		if err != nil {
			return fmt.Errorf("Port is invalid for %q %q logger address %q: %w", cfg.Name, cfg.Type, cfg.Address, err)
		}

		if portInt < 1 || portInt > 0xffff {
			return fmt.Errorf("Port %d is invalid for %q %q logger address %q", portInt, cfg.Name, cfg.Type, cfg.Address)
		}
	}

	_, err = time.ParseDuration(cfg.RetryTimeout)
	if err != nil {
		return fmt.Errorf("Logger retry timeout %q is invalid: %w", cfg.RetryTimeout, err)
	}

	return nil
}

func WebhookConfigChanged(oldCfgs, newCfgs []api.SystemSettingsLog) bool {
	if len(oldCfgs) != len(newCfgs) {
		return true
	}

	for i := range oldCfgs {
		if oldCfgs[i].Name != newCfgs[i].Name ||
			oldCfgs[i].Type != newCfgs[i].Type ||
			oldCfgs[i].Level != newCfgs[i].Level ||
			oldCfgs[i].Address != newCfgs[i].Address ||
			oldCfgs[i].Username != newCfgs[i].Username ||
			oldCfgs[i].Password != newCfgs[i].Password ||
			oldCfgs[i].CACert != newCfgs[i].CACert ||
			oldCfgs[i].RetryCount != newCfgs[i].RetryCount ||
			oldCfgs[i].RetryTimeout != newCfgs[i].RetryTimeout ||
			!slices.Equal(oldCfgs[i].Scopes, newCfgs[i].Scopes) {
			return true
		}
	}

	return false
}

func NewWebhookLogger(cfg api.SystemSettingsLog) (slog.Handler, error) {
	w := &webhookLog{
		name:  cfg.Name,
		level: ParseLevel(cfg.Level),

		address:  cfg.Address,
		username: cfg.Username,
		password: cfg.Password,
		retry:    cfg.RetryCount,
		scopes:   cfg.Scopes,

		client: &http.Client{},
	}

	var err error
	w.retryTimeout, err = time.ParseDuration(cfg.RetryTimeout)
	if err != nil {
		return nil, err
	}

	if cfg.CACert != "" {
		// Prepare the TLS config.
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS13,
		}

		// Parse the provided certificate.
		certBlock, _ := pem.Decode([]byte(cfg.CACert))
		if certBlock == nil {
			return nil, errors.New("Invalid remote certificate")
		}

		serverCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Invalid remote certificate: %w", err)
		}

		// Add the certificate to the TLS config.
		incustls.TLSConfigWithTrustedCert(tlsConfig, serverCert)

		// Configure the HTTP client with our TLS config.
		w.client.Transport = &http.Transport{TLSClientConfig: tlsConfig}
	}

	return w, nil
}

// Enabled implements slog.Handler.
func (w *webhookLog) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= w.level
}

// Handle implements slog.Handler.
func (w *webhookLog) Handle(ctx context.Context, r slog.Record) error {
	event := api.Event{Time: r.Time.UTC()}

	if r.Message != string(api.LogScopeLifecycle) {
		ctxMap := map[string]string{}
		r.Attrs(func(a slog.Attr) bool {
			ctxMap[a.Key] = a.Value.String()
			return true
		})

		b, err := json.Marshal(api.EventLogging{
			Message: r.Message,
			Level:   r.Level.String(),
			Context: ctxMap,
		})
		if err != nil {
			return err
		}

		event.Type = api.LogScopeLogging
		event.Metadata = b
	} else {
		var b []byte
		var err error
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "event" {
				b, err = json.Marshal(a.Value.Any())
				if err != nil {
					return false
				}
			}

			return true
		})
		if err != nil {
			return err
		}

		event.Type = api.LogScopeLifecycle
		event.Metadata = b
	}

	if !slices.Contains(w.scopes, event.Type) {
		return nil
	}

	// Prepare the request.
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, w.address, bytes.NewReader(b))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	go func() {
		for i := range w.retry {
			resp, err := w.client.Do(req)
			if err != nil {
				if i < w.retry-1 {
					// Wait 10s and try again.
					time.Sleep(w.retryTimeout)
				}

				continue
			}

			_ = resp.Body.Close()
			return
		}
	}()

	return nil
}

// WithAttrs implements slog.Handler.
func (w *webhookLog) WithAttrs(attrs []slog.Attr) slog.Handler {
	return w
}

// WithGroup implements slog.Handler.
func (w *webhookLog) WithGroup(name string) slog.Handler {
	return w
}
