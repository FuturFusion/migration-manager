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

	Get: APIEndpointAction{Handler: systemNetworkGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put: APIEndpointAction{Handler: systemNetworkPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemSecurityCmd = APIEndpoint{
	Path: "system/security",

	Get: APIEndpointAction{Handler: systemSecurityGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put: APIEndpointAction{Handler: systemSecurityPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var systemCertificateCmd = APIEndpoint{
	Path: "system/certificate",

	Post: APIEndpointAction{Handler: systemCertificateUpdate, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
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

	var cfg api.ConfigNetwork
	err = json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		return response.BadRequest(err)
	}

	newConfig := d.config
	newConfig.Network = cfg

	err = d.ReloadConfig(newConfig)
	if err != nil {
		return response.SmartError(err)
	}

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

	var cfg api.ConfigSecurity
	err = json.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		return response.BadRequest(err)
	}

	newConfig := d.config
	newConfig.Security = cfg

	err = d.ReloadConfig(newConfig)
	if err != nil {
		return response.SmartError(err)
	}

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

	var cfg api.CertificatePost
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
