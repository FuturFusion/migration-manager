package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/lxc/incus/v6/shared/revert"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/config"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/auth/oidc"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var systemSecurityCmd = APIEndpoint{
	Path: "system/security",

	Get:   APIEndpointAction{Handler: systemSecurityGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put:   APIEndpointAction{Handler: systemSecurityUpdate(http.MethodPut), AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Patch: APIEndpointAction{Handler: systemSecurityUpdate(http.MethodPatch), AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

func systemSecurityGet(d *Daemon, r *http.Request) response.Response {
	apiCfg := d.config

	// Don't display the private key over the API.
	apiCfg.ServerCertificate.Key = ""

	return response.SyncResponse(true, apiCfg)
}

func systemSecurityUpdate(method string) func(*Daemon, *http.Request) response.Response {
	replace := method == http.MethodPut
	return func(d *Daemon, r *http.Request) response.Response {
		entries, err := d.queue.GetAll(r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		if len(entries) > 0 {
			return response.PreconditionFailed(fmt.Errorf("Unable to update server config, active migration in progress"))
		}

		var cfg api.SystemConfigPut
		err = json.NewDecoder(r.Body).Decode(&cfg)
		if err != nil {
			return response.BadRequest(err)
		}

		oldCfg := d.config
		var cfgIsValid bool
		reverter := revert.New()
		defer reverter.Fail()
		reverter.Add(func() {
			slog.Error("Reverting system update after error")
			changedCert := d.config.ServerCertificate != oldCfg.ServerCertificate && cfgIsValid
			changedOIDC := d.config.OIDC != oldCfg.OIDC && cfgIsValid
			changedOpenFGA := d.config.OpenFGA != oldCfg.OpenFGA && cfgIsValid

			// Revert to the old config
			d.config = oldCfg
			err := config.SaveConfig(d.config)
			if err != nil {
				slog.Error("Failed to revert server config", slog.Any("error", err))
			}

			if changedCert {
				err = d.updateServerCert()
				if err != nil {
					slog.Error("Failed to revert server certificate change", slog.Any("error", err))
				}
			}

			if changedOIDC {
				d.oidcVerifier, err = oidc.NewVerifier(d.config.OIDC.Issuer, d.config.OIDC.ClientID, d.config.OIDC.Scope, d.config.OIDC.Audience, d.config.OIDC.Claim)
				if err != nil {
					slog.Error("Failed to revert OIDC config change", slog.Any("error", err))
				}
			}

			if changedOpenFGA {
				err = d.setupOpenFGA(d.config.OpenFGA.APIURL, d.config.OpenFGA.APIToken, d.config.OpenFGA.StoreID)
				if err != nil {
					slog.Error("Failed to revert OpenFGA config change", slog.Any("error", err))
				}
			}
		})

		if replace {
			d.config.SystemConfigPut = cfg
		} else {
			if cfg.RestWorkerEndpoint != "" {
				d.config.RestWorkerEndpoint = cfg.RestWorkerEndpoint
			}

			if cfg.RestServerIPAddr != "" {
				d.config.RestServerIPAddr = cfg.RestServerIPAddr
			}

			if cfg.RestServerPort != 0 {
				d.config.RestServerPort = cfg.RestServerPort
			}

			if len(cfg.TrustedTLSClientCertFingerprints) > 0 {
				d.config.TrustedTLSClientCertFingerprints = cfg.TrustedTLSClientCertFingerprints
			}

			if cfg.ServerCertificate.Cert != "" {
				d.config.ServerCertificate.Cert = cfg.ServerCertificate.Cert
			}

			if cfg.ServerCertificate.Key != "" {
				d.config.ServerCertificate.Key = cfg.ServerCertificate.Key
			}

			if cfg.ServerCertificate.CA != "" {
				d.config.ServerCertificate.CA = cfg.ServerCertificate.CA
			}

			if cfg.OIDC.Issuer != "" {
				d.config.OIDC.Issuer = cfg.OIDC.Issuer
			}

			if cfg.OIDC.ClientID != "" {
				d.config.OIDC.ClientID = cfg.OIDC.ClientID
			}

			if cfg.OIDC.Scope != "" {
				d.config.OIDC.Scope = cfg.OIDC.Scope
			}

			if cfg.OIDC.Audience != "" {
				d.config.OIDC.Audience = cfg.OIDC.Audience
			}

			if cfg.OIDC.Claim != "" {
				d.config.OIDC.Claim = cfg.OIDC.Claim
			}

			if cfg.OpenFGA.APIToken != "" {
				d.config.OpenFGA.APIToken = cfg.OpenFGA.APIToken
			}

			if cfg.OpenFGA.APIURL != "" {
				d.config.OpenFGA.APIURL = cfg.OpenFGA.APIURL
			}

			if cfg.OpenFGA.StoreID != "" {
				d.config.OpenFGA.StoreID = cfg.OpenFGA.StoreID
			}
		}

		err = config.Validate(d.config)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to update server config: %w", err))
		}

		// If we got here, at least the new config was deemed valid so begin updating the daemon.
		cfgIsValid = true
		err = config.SaveConfig(d.config)
		if err != nil {
			return response.SmartError(err)
		}

		if d.config.OIDC != oldCfg.OIDC {
			d.oidcVerifier, err = oidc.NewVerifier(d.config.OIDC.Issuer, d.config.OIDC.ClientID, d.config.OIDC.Scope, d.config.OIDC.Audience, d.config.OIDC.Claim)
			if err != nil {
				return response.SmartError(err)
			}
		}

		// Setup OpenFGA authorization.
		if d.config.OpenFGA != oldCfg.OpenFGA {
			err = d.setupOpenFGA(d.config.OpenFGA.APIURL, d.config.OpenFGA.APIToken, d.config.OpenFGA.StoreID)
			if err != nil {
				return response.SmartError(fmt.Errorf("Failed to configure OpenFGA: %w", err))
			}
		}

		if d.config.ServerCertificate != oldCfg.ServerCertificate {
			err = d.updateServerCert()
			if err != nil {
				return response.SmartError(err)
			}
		}

		if d.config.RestServerIPAddr != oldCfg.RestServerIPAddr || d.config.RestServerPort != oldCfg.RestServerPort {
			d.updateHTTPListener(net.JoinHostPort(d.config.RestServerIPAddr, strconv.Itoa(d.config.RestServerPort)))
		}

		reverter.Success()

		return response.EmptySyncResponse
	}
}
