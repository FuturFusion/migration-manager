package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/shared/api"
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

// swagger:operation GET /1.0/sources sources sources_get
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
	result := []api.Source{}
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		sources, err := d.db.GetAllSources(tx)
		if err != nil {
			return err
		}

		result = sources

		return nil
	})
	if err != nil {
		return response.SmartError(err)
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
	var s api.Source

	// Decode into the new source.
	err := json.NewDecoder(r.Body).Decode(&s)
	if err != nil {
		return response.BadRequest(err)
	}

	// Insert into database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		s, err = d.db.AddSource(tx, s)
		return err
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating source %q: %w", s.Name, err))
	}

	// Trigger a scan of this new source for instances.
	_ = d.syncInstancesFromSources()

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/sources/"+s.Name)
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

	if name == "" {
		return response.BadRequest(fmt.Errorf("Source name cannot be empty"))
	}

	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.DeleteSource(tx, name)
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to delete source '%s': %w", name, err))
	}

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

	if name == "" {
		return response.BadRequest(fmt.Errorf("Source name cannot be empty"))
	}

	var s api.Source
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbSource, err := d.db.GetSource(tx, name)
		if err != nil {
			return err
		}

		s = dbSource
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get source '%s': %w", name, err))
	}

	return response.SyncResponseETag(true, s, s)
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

	if name == "" {
		return response.BadRequest(fmt.Errorf("Source name cannot be empty"))
	}

	// Get the existing source.
	var s api.Source
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbSource, err := d.db.GetSource(tx, name)
		if err != nil {
			return err
		}

		s = dbSource
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get source '%s': %w", name, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, s)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	// Decode into the existing source.
	err = json.NewDecoder(r.Body).Decode(&s)
	if err != nil {
		return response.BadRequest(err)
	}

	// Update source in the database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		s, err = d.db.UpdateSource(tx, s)
		return err
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating source %q: %w", s.Name, err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/sources/"+s.Name)
}
