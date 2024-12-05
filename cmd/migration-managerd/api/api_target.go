package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"

	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var targetsCmd = APIEndpoint{
	Path: "targets",

	Get:  APIEndpointAction{Handler: targetsGet, AllowUntrusted: true},
	Post: APIEndpointAction{Handler: targetsPost, AllowUntrusted: true},
}

var targetCmd = APIEndpoint{
	Path: "targets/{name}",

	Delete: APIEndpointAction{Handler: targetDelete, AllowUntrusted: true},
	Get:    APIEndpointAction{Handler: targetGet, AllowUntrusted: true},
	Put:    APIEndpointAction{Handler: targetPut, AllowUntrusted: true},
}

// swagger:operation GET /1.0/targets targets targets_get
//
//	Get the targets
//
//	Returns a list of targets (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API targets
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
//	            $ref: "#/definitions/IncusTarget"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func targetsGet(d *Daemon, r *http.Request) response.Response {
	result := []target.Target{}
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		targets, err := d.db.GetAllTargets(tx)
		if err != nil {
			return err
		}

		result = targets
		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, result)
}

// swagger:operation POST /1.0/targets targets targets_post
//
//	Add a target
//
//	Creates a new target.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: target
//	    description: Target configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/IncusTarget"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func targetsPost(d *Daemon, r *http.Request) response.Response {
	var t target.InternalIncusTarget

	// Decode into the new target.
	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		return response.BadRequest(err)
	}

	// Insert into database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.AddTarget(tx, &t)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating target %q: %w", t.GetName(), err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/targets/"+t.GetName())
}

// swagger:operation DELETE /1.0/targets/{name} targets target_delete
//
//	Delete the target
//
//	Removes the target.
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
func targetDelete(d *Daemon, r *http.Request) response.Response {
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	if name == "" {
		// TODO: can this code path even be reached?
		return response.BadRequest(fmt.Errorf("Target name cannot be empty"))
	}

	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.DeleteTarget(tx, name)
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to delete target '%s': %w", name, err))
	}

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/targets/{name} targets target_get
//
//	Get the target
//
//	Gets a specific target.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Target
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
//	          $ref: "#/definitions/IncusTarget"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func targetGet(d *Daemon, r *http.Request) response.Response {
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	if name == "" {
		return response.BadRequest(fmt.Errorf("Target name cannot be empty"))
	}

	var t target.Target
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbTarget, err := d.db.GetTarget(tx, name)
		if err != nil {
			return err
		}

		t = dbTarget
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get target '%s': %w", name, err))
	}

	return response.SyncResponseETag(true, t, t)
}

// swagger:operation PUT /1.0/targets/{name} targets target_put
//
//	Update the target
//
//	Updates the target definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: target
//	    description: Target definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/IncusTarget"
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
func targetPut(d *Daemon, r *http.Request) response.Response {
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	if name == "" {
		return response.BadRequest(fmt.Errorf("Target name cannot be empty"))
	}

	// Get the existing target.
	var t target.Target
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbTarget, err := d.db.GetTarget(tx, name)
		if err != nil {
			return err
		}

		t = dbTarget
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get target '%s': %w", name, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, t)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	// Decode into the existing target.
	err = json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		return response.BadRequest(err)
	}

	// Update target in the database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateTarget(tx, t)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating target %q: %w", t.GetName(), err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/targets/"+t.GetName())
}
