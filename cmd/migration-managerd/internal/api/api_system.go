package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
	"github.com/FuturFusion/migration-manager/shared/api/event"
)

var systemBackupCmd = APIEndpoint{
	Path: "system/:backup",

	Post: APIEndpointAction{Handler: systemBackupPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemRestoreCmd = APIEndpoint{
	Path: "system/:restore",

	Post: APIEndpointAction{Handler: systemRestorePost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemNetworkCmd = APIEndpoint{
	Path: "system/network",

	Get: APIEndpointAction{Handler: systemNetworkGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put: APIEndpointAction{Handler: systemNetworkPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemSecurityCmd = APIEndpoint{
	Path: "system/security",

	Get: APIEndpointAction{Handler: systemSecurityGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put: APIEndpointAction{Handler: systemSecurityPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemSettingsCmd = APIEndpoint{
	Path: "system/settings",

	Get: APIEndpointAction{Handler: systemSettingsGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put: APIEndpointAction{Handler: systemSettingsPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemCertificateCmd = APIEndpoint{
	Path: "system/certificate",

	Post: APIEndpointAction{Handler: systemCertificateUpdate, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var restoreLock sync.Mutex

// swagger:operation POST /1.0/system/:backup system system_backup_post
//
//	Generate a system backup
//
//	Generate and return a `gzip` compressed tar archive backup of the system state and configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	  - application/gzip
//	parameters:
//	  - in: body
//	    name: system
//	    description: Backup configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/SystemBackupPost"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func systemBackupPost(d *Daemon, r *http.Request) response.Response {
	var cfg api.SystemBackupPost
	err := json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		return response.BadRequest(err)
	}

	artifacts, err := d.artifact.GetAll(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	includeMap := map[uuid.UUID]bool{}
	for _, artUUID := range cfg.IncludeArtifacts {
		includeMap[artUUID] = true
	}

	exclude := []string{}
	for _, a := range artifacts {
		if !includeMap[a.UUID] {
			exclude = append(exclude, filepath.Join(filepath.Base(d.os.ArtifactDir), a.UUID.String()))
		}
	}

	return response.ManualResponse(func(w http.ResponseWriter) error {
		w.Header().Set("Content-Type", "application/gzip")
		err = util.CreateTarballWriter(r.Context(), w, d.os.VarDir, exclude...)
		if err != nil {
			return response.SmartError(err).Render(w)
		}

		return nil
	})
}

// swagger:operation POST /1.0/system/:restore system system_restore_post
//
//	Restore a system backup
//
//	Restore a `gzip` compressed tar backup of the system state and configuration. Upon completion Migration Manager will immediately restart.
//
//	Remember to properly set the `Content-Type: application/gzip` HTTP header.
//
//	---
//	consumes:
//	  - application/gzip
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: gzip tar archive
//	    description: Application backup to restore
//	    required: true
//	    schema:
//	      type: file
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func systemRestorePost(d *Daemon, r *http.Request) response.Response {
	queue, err := d.queue.GetAll(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	for _, q := range queue {
		if q.MigrationStatus != api.MIGRATIONSTATUS_FINISHED && q.MigrationStatus != api.MIGRATIONSTATUS_ERROR && q.MigrationStatus != api.MIGRATIONSTATUS_CANCELED {
			return response.SmartError(fmt.Errorf("Unable to perform backup restore, queue entries are still migrating"))
		}
	}

	err = d.os.WriteFile(filepath.Join(d.os.CacheDir, "backup.tar.gz"), r.Body)
	if err != nil {
		return response.SmartError(err)
	}

	go func() {
		<-r.Context().Done() // Wait until request has finished.

		restoreLock.Lock()
		defer restoreLock.Unlock()
		slog.Info("Restarting daemon to initiate restore from backup")
		err := sys.ReplaceDaemon()
		if err != nil {
			slog.Error("Failed restarting daemon", slog.Any("error", err))
		}
	}()

	return response.ManualResponse(func(w http.ResponseWriter) error {
		err := response.EmptySyncResponse.Render(w)
		if err != nil {
			return err
		}

		f, ok := w.(http.Flusher)
		if ok {
			f.Flush()
			return nil
		}

		return fmt.Errorf("Unable to flush response writer %T", f)
	})
}

// swagger:operation GET /1.0/system/settings system_settings system_settings_get
//
//	Get the system settings configuration
//
//	Returns the system settings configuration.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API system settings
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: System settings configuration
//	          items:
//	            $ref: "#/definitions/SystemSettings"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func systemSettingsGet(d *Daemon, r *http.Request) response.Response {
	return response.SyncResponse(true, d.config.Settings)
}

// swagger:operation PUT /1.0/system/settings system_settings system_settings_put
//
//	Update the system settings configuration
//
//	Updates the system settings configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: system_settings
//	    description: System settings configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/SystemSettings"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func systemSettingsPut(d *Daemon, r *http.Request) response.Response {
	var cfg api.SystemSettings
	err := json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		return response.BadRequest(err)
	}

	newConfig := d.config
	newConfig.Settings = cfg

	err = d.ReloadConfig(false, newConfig)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewSystemSettingsEvent(event.SystemSettingsModified, r, d.config.Settings))

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/system/network system_network system_network_get
//
//	Get the system network configuration
//
//	Returns the system network configuration.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API system network
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: System network configuration
//	          items:
//	            $ref: "#/definitions/ConfigNetwork"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func systemNetworkGet(d *Daemon, r *http.Request) response.Response {
	return response.SyncResponse(true, d.config.Network)
}

// swagger:operation PUT /1.0/system/network system_network system_network_put
//
//	Update the system network configuration
//
//	Updates the system network configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: system_network
//	    description: System network configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/ConfigNetwork"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func systemNetworkPut(d *Daemon, r *http.Request) response.Response {
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

	var cfg api.SystemNetwork
	err = json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		return response.BadRequest(err)
	}

	newConfig := d.config
	newConfig.Network = cfg

	err = d.ReloadConfig(false, newConfig)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewSystemNetworkEvent(event.SystemNetworkModified, r, d.config.Network))

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/system/security system_security system_security_get
//
//	Get the system security configuration
//
//	Returns the system security configuration
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API system security
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: System security configuration
//	          items:
//	            $ref: "#/definitions/ConfigSecurity"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func systemSecurityGet(d *Daemon, r *http.Request) response.Response {
	return response.SyncResponse(true, d.config.Security)
}

// swagger:operation PUT /1.0/system/security system_security system_security_put
//
//	Update the system security configuration
//
//	Updates the system security configuration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: system_security
//	    description: System security configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/ConfigSecurity"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func systemSecurityPut(d *Daemon, r *http.Request) response.Response {
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

	var cfg api.SystemSecurity
	err = json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		return response.BadRequest(err)
	}

	newConfig := d.config
	newConfig.Security = cfg

	err = d.ReloadConfig(false, newConfig)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewSystemSecurityEvent(event.SystemSecurityModified, r, d.config.Security))

	return response.EmptySyncResponse
}

// swagger:operation POST /1.0/system/certificate system_certificate system_certificate_post
//
//	Update system certificate
//
//	Updates the system certificate.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: system_certificate
//	    description: Certificate configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/CertificatePost"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
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

	var cfg api.SystemCertificatePost
	err = json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		return response.BadRequest(err)
	}

	err = d.updateServerCert(cfg)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewSystemCertificatePostEvent(event.SystemCertificateModified, r, api.SystemCertificatePost{}))

	return response.EmptySyncResponse
}
