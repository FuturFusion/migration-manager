package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	sharedAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var instancesCmd = APIEndpoint{
	Path: "instances",

	Get: APIEndpointAction{Handler: instancesGet, AllowUntrusted: true},
}

var instanceCmd = APIEndpoint{
	Path: "instances/{uuid}",

	Get: APIEndpointAction{Handler: instanceGet, AllowUntrusted: true},
}

var instanceStateCmd = APIEndpoint{
	Path: "instances/{uuid}/state",

	Put: APIEndpointAction{Handler: instanceStatePut, AllowUntrusted: true},
}

var instanceOverrideCmd = APIEndpoint{
	Path: "instances/{uuid}/override",

	Delete: APIEndpointAction{Handler: instanceOverrideDelete, AllowUntrusted: true},
	Get:    APIEndpointAction{Handler: instanceOverrideGet, AllowUntrusted: true},
	Post:   APIEndpointAction{Handler: instanceOverridePost, AllowUntrusted: true},
	Put:    APIEndpointAction{Handler: instanceOverridePut, AllowUntrusted: true},
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

// swagger:operation GET /1.0/instances/{uuid}/override instances instance_override_get
//
//	Get the instance override
//
//	Gets a specific instance override.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: InstanceOverride
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
//	          $ref: "#/definitions/InstanceOverride"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instanceOverrideGet(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(err)
	}

	var override api.InstanceOverride
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbOverride, err := d.db.GetInstanceOverride(tx, UUID)
		if err != nil {
			return err
		}

		override = dbOverride
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get override for instance '%s': %w", UUID, err))
	}

	return response.SyncResponseETag(true, override, override)
}

// swagger:operation POST /1.0/instances/{uuid}/override instances instance_override_post
//
//	Add an instance override
//
//	Creates a new instance override.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: override
//	    description: Instance override
//	    required: true
//	    schema:
//	      $ref: "#/definitions/InstanceOverride"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instanceOverridePost(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(err)
	}

	var override api.InstanceOverride

	// Decode into the new override.
	err = json.NewDecoder(r.Body).Decode(&override)
	if err != nil {
		return response.BadRequest(err)
	}

	// Insert into database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.AddInstanceOverride(tx, override)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating override for instance %s: %w", UUID, err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/instances/"+UUIDString+"/override")
}

// swagger:operation PUT /1.0/instances/{uuid}/override instances instance_override_put
//
//	Update the instance override
//
//	Updates the instance override definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: instance
//	    description: Instance override definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/InstanceOverride"
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
func instanceOverridePut(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(err)
	}

	// Get the existing instance override.
	var override api.InstanceOverride
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbOverride, err := d.db.GetInstanceOverride(tx, UUID)
		if err != nil {
			return err
		}

		override = dbOverride
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get override for instance '%s': %w", UUID, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, override)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	// Decode into the existing instance override.
	err = json.NewDecoder(r.Body).Decode(&override)
	if err != nil {
		return response.BadRequest(err)
	}

	// Update instance override in the database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateInstanceOverride(tx, override)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating override for instance '%s': %w", UUID, err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/instances/"+UUID.String()+"/override")
}

// swagger:operation DELETE /1.0/instances/{uuid}/override instances instance_override_delete
//
//	Delete an instance override
//
//	Removes the instance override.
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
func instanceOverrideDelete(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(err)
	}

	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.DeleteInstanceOverride(tx, UUID)
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to delete override for instance '%s': %w", UUID, err))
	}

	return response.EmptySyncResponse
}

// swagger:operation PUT /1.0/instances/{uuid}/state instances instance_state_put
//
//	Mark an instance as not eligible for migration
//
//	As long as an instance is not yet assigned to a batch, it can be marked as
//	not eligible for migration.
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - description: State
//	    example: true
//	    in: query
//	    name: migration_user_disabled
//	    type: boolean
//	    required: true
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instanceStatePut(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		// TODO: can this code path even be reached?
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(sharedAPI.StatusErrorf(http.StatusBadRequest, "Invalid instance UUID %q", UUID))
	}

	migrationUserDisabledString := r.URL.Query().Get("migration_user_disabled")
	migrationUserDisabled, err := strconv.ParseBool(migrationUserDisabledString)
	if err != nil {
		return response.SmartError(sharedAPI.StatusErrorf(http.StatusBadRequest, "Invalid value for migration_user_disabled %q", migrationUserDisabledString))
	}

	allowedSourceState := api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION
	targetState := api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
	if migrationUserDisabled {
		allowedSourceState = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
		targetState = api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION
	}

	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		// Get the instance.
		i, err := d.db.GetInstance(tx, UUID)
		if err != nil {
			return sharedAPI.StatusErrorf(http.StatusBadRequest, "Failed to get instance %q: %v", UUID, err)
		}

		internalInstance, ok := i.(*instance.InternalInstance)
		if !ok {
			return fmt.Errorf("Unsupported instance type %T", i)
		}

		if internalInstance.MigrationStatus != allowedSourceState {
			return sharedAPI.StatusErrorf(http.StatusBadRequest, "Set migration disabled for instance %q in state %q not allowed", UUID, internalInstance.MigrationStatus.String())
		}

		internalInstance.MigrationStatus = targetState

		// Update into database.
		return d.db.UpdateInstance(tx, internalInstance)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating migration status for instance %s: %w", UUID, err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/instances/"+UUIDString)
}
