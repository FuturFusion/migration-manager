package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var systemNetworkCmd = APIEndpoint{
	Path: "system/network",

	Get:   APIEndpointAction{Handler: systemNetworkGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put:   APIEndpointAction{Handler: systemNetworkUpdate(http.MethodPut), AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Patch: APIEndpointAction{Handler: systemNetworkUpdate(http.MethodPatch), AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemSecurityCmd = APIEndpoint{
	Path: "system/security",

	Get:   APIEndpointAction{Handler: systemSecurityGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put:   APIEndpointAction{Handler: systemSecurityUpdate(http.MethodPut), AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Patch: APIEndpointAction{Handler: systemSecurityUpdate(http.MethodPatch), AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemCertificateCmd = APIEndpoint{
	Path: "system/certificate",

	Post: APIEndpointAction{Handler: systemCertificateUpdate, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemSdkCmd = APIEndpoint{
	Path: "system/sdks/{sourceType}",

	Get:  APIEndpointAction{Handler: systemSdkGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Post: APIEndpointAction{Handler: systemSdkPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

func systemNetworkGet(d *Daemon, r *http.Request) response.Response {
	return response.SyncResponse(true, d.config.Network)
}

func systemNetworkUpdate(method string) func(*Daemon, *http.Request) response.Response {
	replace := method == http.MethodPut
	return func(d *Daemon, r *http.Request) response.Response {
		entries, err := d.queue.GetAll(r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		runningEntries := migration.QueueEntries{}
		for _, e := range entries {
			if e.MigrationStatus != api.MIGRATIONSTATUS_FINISHED && e.MigrationStatus != api.MIGRATIONSTATUS_ERROR {
				runningEntries = append(runningEntries, e)
			}
		}

		if len(runningEntries) > 0 {
			return response.PreconditionFailed(fmt.Errorf("Unable to update server config, active migration in progress"))
		}

		var cfg api.ConfigNetwork
		err = json.NewDecoder(r.Body).Decode(&cfg)
		if err != nil {
			return response.BadRequest(err)
		}

		newConfig := d.config
		if replace {
			newConfig.Network = cfg
		} else {
			if cfg.WorkerEndpoint != "" {
				newConfig.Network.WorkerEndpoint = cfg.WorkerEndpoint
			}

			if cfg.Address != "" {
				newConfig.Network.Address = cfg.Address
			}

			if cfg.Port != 0 {
				newConfig.Network.Port = cfg.Port
			}
		}

		err = d.ReloadConfig(newConfig)
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}
}

func systemSecurityGet(d *Daemon, r *http.Request) response.Response {
	return response.SyncResponse(true, d.config.Security)
}

func systemSecurityUpdate(method string) func(*Daemon, *http.Request) response.Response {
	replace := method == http.MethodPut
	return func(d *Daemon, r *http.Request) response.Response {
		entries, err := d.queue.GetAll(r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		runningEntries := migration.QueueEntries{}
		for _, e := range entries {
			if e.MigrationStatus != api.MIGRATIONSTATUS_FINISHED && e.MigrationStatus != api.MIGRATIONSTATUS_ERROR {
				runningEntries = append(runningEntries, e)
			}
		}

		if len(runningEntries) > 0 {
			return response.PreconditionFailed(fmt.Errorf("Unable to update server config, active migration in progress"))
		}

		var cfg api.ConfigSecurity
		err = json.NewDecoder(r.Body).Decode(&cfg)
		if err != nil {
			return response.BadRequest(err)
		}

		newConfig := d.config
		if replace {
			newConfig.Security = cfg
		} else {
			if len(cfg.TrustedTLSClientCertFingerprints) > 0 {
				newConfig.Security.TrustedTLSClientCertFingerprints = cfg.TrustedTLSClientCertFingerprints
			}

			if cfg.OIDC.Issuer != "" {
				newConfig.Security.OIDC.Issuer = cfg.OIDC.Issuer
			}

			if cfg.OIDC.ClientID != "" {
				newConfig.Security.OIDC.ClientID = cfg.OIDC.ClientID
			}

			if cfg.OIDC.Scope != "" {
				newConfig.Security.OIDC.Scope = cfg.OIDC.Scope
			}

			if cfg.OIDC.Audience != "" {
				newConfig.Security.OIDC.Audience = cfg.OIDC.Audience
			}

			if cfg.OIDC.Claim != "" {
				newConfig.Security.OIDC.Claim = cfg.OIDC.Claim
			}

			if cfg.OpenFGA.APIToken != "" {
				newConfig.Security.OpenFGA.APIToken = cfg.OpenFGA.APIToken
			}

			if cfg.OpenFGA.APIURL != "" {
				newConfig.Security.OpenFGA.APIURL = cfg.OpenFGA.APIURL
			}

			if cfg.OpenFGA.StoreID != "" {
				newConfig.Security.OpenFGA.StoreID = cfg.OpenFGA.StoreID
			}
		}

		err = d.ReloadConfig(newConfig)
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}
}

func systemCertificateUpdate(d *Daemon, r *http.Request) response.Response {
	entries, err := d.queue.GetAll(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	runningEntries := migration.QueueEntries{}
	for _, e := range entries {
		if e.MigrationStatus != api.MIGRATIONSTATUS_FINISHED && e.MigrationStatus != api.MIGRATIONSTATUS_ERROR {
			runningEntries = append(runningEntries, e)
		}
	}

	if len(runningEntries) > 0 {
		return response.PreconditionFailed(fmt.Errorf("Unable to update server config, active migration in progress"))
	}

	var cfg api.Certificate
	err = json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		return response.BadRequest(err)
	}

	err = d.updateServerCert(cfg)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

func systemSdkGet(d *Daemon, r *http.Request) response.Response {
	srcType := r.PathValue("sourceType")
	if api.SourceType(srcType) != api.SOURCETYPE_VMWARE {
		return response.BadRequest(fmt.Errorf("SDK upload for source type %q is not supported", srcType))
	}

	sdkName, err := d.os.GetSDKName(api.SourceType(srcType))
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, sdkName)
}

func systemSdkPost(d *Daemon, r *http.Request) response.Response {
	srcType := r.PathValue("sourceType")
	if api.SourceType(srcType) != api.SOURCETYPE_VMWARE {
		return response.BadRequest(fmt.Errorf("SDK upload for source type %q is not supported", srcType))
	}

	err := d.os.WriteSDK(api.SourceType(srcType), r.Body)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to write SDK for source type %q: %w", srcType, err))
	}

	return response.EmptySyncResponse
}
