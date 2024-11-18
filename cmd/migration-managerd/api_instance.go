package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/version"
)

var instancesCmd = APIEndpoint{
	Path: "instances",

	Get:  APIEndpointAction{Handler: instancesGet, AllowUntrusted: true},
}

var instanceCmd = APIEndpoint{
	Path: "instances/{uuid}",

	Get:    APIEndpointAction{Handler: instanceGet, AllowUntrusted: true},
	Put:    APIEndpointAction{Handler: instancePut, AllowUntrusted: true},
}

// swagger:operation GET /1.0/instances instances instances_get
//
//	Get the instances
//
//	Returns a list of instances (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API instances
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
//	            $ref: "#/definitions/Instance"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instancesGet(d *Daemon, r *http.Request) response.Response {
	result := []instance.Instance{}
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		instances, err := d.db.GetAllInstances(tx)
		if err != nil {
			return err
		}

		result = instances
		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, result)
}

// swagger:operation GET /1.0/instances/{uuid} instances instance_get
//
//	Get the instance
//
//	Gets a specific instance.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Instance
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
//	          $ref: "#/definitions/Instance"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instanceGet(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(err)
	}

	var i instance.Instance
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbInstance, err := d.db.GetInstance(tx, UUID)
		if err != nil {
			return err
		}

		i = dbInstance
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get instance '%s': %w", UUID, err))
	}

	return response.SyncResponseETag(true, i, i)
}

// swagger:operation PUT /1.0/instances/{uuid} instances instance_put
//
//	Update the instance
//
//	Updates the instance definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: instance
//	    description: Instance definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/Instance"
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
func instancePut(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(err)
	}

	// Get the existing instance.
	var i instance.Instance
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbInstance, err := d.db.GetInstance(tx, UUID)
		if err != nil {
			return err
		}

		i = dbInstance
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get instance '%s': %w", UUID, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, i)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	// Decode into the existing instance.
	err = json.NewDecoder(r.Body).Decode(&i)
	if err != nil {
		return response.BadRequest(err)
	}

	// Update instance in the database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateInstance(tx, i)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating instance '%s': %w", i.GetUUID(), err))
	}

	return response.SyncResponseLocation(true, nil, "/" + version.APIVersion + "/instances/" + i.GetUUID().String())
}
