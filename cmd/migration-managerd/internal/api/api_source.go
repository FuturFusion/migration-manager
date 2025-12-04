package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	incusTLS "github.com/lxc/incus/v6/shared/tls"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
	"github.com/FuturFusion/migration-manager/shared/api/event"
)

var sourcesCmd = APIEndpoint{
	Path: "sources",

	Get:  APIEndpointAction{Handler: sourcesGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Post: APIEndpointAction{Handler: sourcesPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanCreate)},
}

var sourceCmd = APIEndpoint{
	Path: "sources/{name}",

	Delete: APIEndpointAction{Handler: sourceDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
	Get:    APIEndpointAction{Handler: sourceGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put:    APIEndpointAction{Handler: sourcePut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

var sourceSyncCmd = APIEndpoint{
	Path: "sources/{name}/:sync",

	Post: APIEndpointAction{Handler: sourceSyncPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
}

// swagger:operation GET /1.0/sources sources sources_get
//
//	Get the sources
//
//	Returns a list of sources (URLs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API sources
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
//	          description: List of sources
//	          items:
//	            type: string
//	          example: |-
//	            [
//	              "/1.0/sources/foo",
//	              "/1.0/sources/bar"
// 	            ]
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/sources?recursion=1 sources sources_get_recursion
//
//	Get the sources
//
//	Returns a list of sources (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API sources
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
//	          description: List of sources
//	          items:
//	            $ref: "#/definitions/Source"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func sourcesGet(d *Daemon, r *http.Request) response.Response {
	// Parse the recursion field.
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	if recursion > 0 {
		sources, err := d.source.GetAll(r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		result := make([]api.Source, 0, len(sources))
		for _, src := range sources {
			if src.SourceType == api.SOURCETYPE_NSX && recursion > 1 {
				nsxSource, err := source.NewInternalNSXSourceFrom(src.ToAPI())
				if err != nil {
					return response.SmartError(err)
				}

				err = nsxSource.Connect(r.Context())
				if err != nil {
					return response.SmartError(fmt.Errorf("Failed to connect to source %q: %w", src.Name, err))
				}

				err = nsxSource.FetchSourceData(r.Context())
				if err != nil {
					return response.SmartError(err)
				}

				b, err := json.Marshal(nsxSource.NSXSourceProperties)
				if err != nil {
					return response.SmartError(err)
				}

				src.Properties = b
			}

			result = append(result, src.ToAPI())
		}

		return response.SyncResponse(true, result)
	}

	sourceNames, err := d.source.GetAllNames(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	result := make([]string, 0, len(sourceNames))
	for _, name := range sourceNames {
		result = append(result, fmt.Sprintf("/%s/sources/%s", api.APIVersion, name))
	}

	return response.SyncResponse(true, result)
}

// swagger:operation POST /1.0/sources sources sources_post
//
//	Add a source
//
//	Creates a new source.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: source
//	    description: Source configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/Source"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func sourcesPost(d *Daemon, r *http.Request) response.Response {
	var apiSrc api.Source

	err := json.NewDecoder(r.Body).Decode(&apiSrc)
	if err != nil {
		return response.BadRequest(err)
	}

	src, err := d.source.Create(r.Context(), migration.Source{
		Name:       apiSrc.Name,
		SourceType: apiSrc.SourceType,
		Properties: apiSrc.Properties,
		EndpointFunc: func(s api.Source) (migration.SourceEndpoint, error) {
			switch s.SourceType {
			case api.SOURCETYPE_VMWARE:
				return source.NewInternalVMwareSourceFrom(s)
			case api.SOURCETYPE_NSX:
				return source.NewInternalNSXSourceFrom(s)
			}

			return nil, fmt.Errorf("Unknown source type: %q", s.SourceType)
		},
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating source %q: %w", apiSrc.Name, err))
	}

	// Trigger a scan of this new source for instances.
	if src.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_OK && src.SourceType == api.SOURCETYPE_VMWARE {
		err = d.syncOneSource(r.Context(), src)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to initiate sync from source %q: %w", src.Name, err))
		}
	}

	metadata := make(map[string]string)
	metadata["ConnectivityStatus"] = string(src.GetExternalConnectivityStatus())

	// If waiting on fingerprint confirmation, return it to the user.
	if src.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT {
		metadata["certFingerprint"] = incusTLS.CertFingerprint(src.GetServerCertificate())
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewSourceEvent(event.SourceCreated, r, src.ToAPI(), src.Name))

	return response.SyncResponseLocation(true, metadata, "/"+api.APIVersion+"/sources/"+apiSrc.Name)
}

// swagger:operation DELETE /1.0/sources/{name} sources source_delete
//
//	Delete the source
//
//	Removes the source.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func sourceDelete(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	var apiSrc api.Source
	err := transaction.Do(r.Context(), func(ctx context.Context) error {
		src, err := d.source.GetByName(ctx, name)
		if err != nil {
			return err
		}

		apiSrc = src.ToAPI()

		return d.source.DeleteByName(ctx, name, d.instance)
	})
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewSourceEvent(event.SourceRemoved, r, apiSrc, apiSrc.Name))

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/sources/{name} sources source_get
//
//	Get the source
//
//	Gets a specific source.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Source
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
//	          $ref: "#/definitions/Source"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func sourceGet(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	src, err := d.source.GetByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	if src.SourceType == api.SOURCETYPE_NSX && recursion > 0 {
		nsxSource, err := source.NewInternalNSXSourceFrom(src.ToAPI())
		if err != nil {
			return response.SmartError(err)
		}

		err = nsxSource.Connect(r.Context())
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to connect to source %q: %w", src.Name, err))
		}

		err = nsxSource.FetchSourceData(r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		b, err := json.Marshal(nsxSource.NSXSourceProperties)
		if err != nil {
			return response.SmartError(err)
		}

		src.Properties = b
	}

	return response.SyncResponseETag(
		true,
		src.ToAPI(),
		src,
	)
}

// swagger:operation PUT /1.0/sources/{name} sources source_put
//
//	Update the source
//
//	Updates the source definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: source
//	    description: Source definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/Source"
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
func sourcePut(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	var apiSrc api.SourcePut

	err := json.NewDecoder(r.Body).Decode(&apiSrc)
	if err != nil {
		return response.BadRequest(err)
	}

	ctx, trans := transaction.Begin(r.Context())
	defer func() {
		rollbackErr := trans.Rollback()
		if rollbackErr != nil {
			response.SmartError(fmt.Errorf("Transaction rollback failed: %v, reason: %w", rollbackErr, err))
		}
	}()

	currentSource, err := d.source.GetByName(ctx, name)
	if err != nil {
		return response.SmartError(err)
	}

	// Validate ETag
	err = util.EtagCheck(r, currentSource)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	src := &migration.Source{
		ID:         currentSource.ID,
		Name:       apiSrc.Name,
		SourceType: currentSource.SourceType,
		Properties: apiSrc.Properties,
		EndpointFunc: func(s api.Source) (migration.SourceEndpoint, error) {
			switch s.SourceType {
			case api.SOURCETYPE_VMWARE:
				return source.NewInternalVMwareSourceFrom(s)
			case api.SOURCETYPE_NSX:
				return source.NewInternalNSXSourceFrom(s)
			}

			return nil, fmt.Errorf("Unknown source type: %q", s.SourceType)
		},
	}

	err = d.source.Update(ctx, name, src, d.instance)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating source %q: %w", apiSrc.Name, err))
	}

	err = trans.Commit()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed commit transaction: %w", err))
	}

	// Trigger a scan of this new source for instances.
	if src.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_OK && src.SourceType == api.SOURCETYPE_VMWARE {
		err = d.syncOneSource(r.Context(), *src)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to initiate sync from source %q: %w", src.Name, err))
		}
	}

	metadata := make(map[string]string)
	metadata["ConnectivityStatus"] = string(src.GetExternalConnectivityStatus())

	// If waiting on fingerprint confirmation, return it to the user.
	if src.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT {
		metadata["certFingerprint"] = incusTLS.CertFingerprint(src.GetServerCertificate())
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewSourceEvent(event.SourceModified, r, src.ToAPI(), src.Name))

	return response.SyncResponseLocation(true, metadata, "/"+api.APIVersion+"/sources/"+apiSrc.Name)
}

// swagger:operation POST /1.0/sources/{name}/:sync sources source_sync
//
//	Sync source data
//
//	Perform a sync to fetch new source data from a specified source.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func sourceSyncPost(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	src, err := d.source.GetByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	err = d.syncOneSource(r.Context(), *src)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewSourceEvent(event.SourceSynced, r, src.ToAPI(), src.Name))

	return response.EmptySyncResponse
}
