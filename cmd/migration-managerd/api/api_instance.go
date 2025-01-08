package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

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

var instanceOverrideCmd = APIEndpoint{
	Path: "instances/{uuid}/override",

	Delete: APIEndpointAction{Handler: instanceOverrideDelete, AccessHandler: allowAuthenticated},
	Get:    APIEndpointAction{Handler: instanceOverrideGet, AllowUntrusted: true},
	Post:   APIEndpointAction{Handler: instanceOverridePost, AccessHandler: allowAuthenticated},
	Put:    APIEndpointAction{Handler: instanceOverridePut, AccessHandler: allowAuthenticated},
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
	UUIDString := r.PathValue("uuid")

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
	UUIDString := r.PathValue("uuid")

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
	UUIDString := r.PathValue("uuid")

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

	// If migration is disabled, need to update the actual instance status.
	if override.DisableMigration {
		err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
			return d.db.UpdateInstanceStatus(tx, UUID, api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION, api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION.String(), true)
		})
		if err != nil {
			return response.BadRequest(fmt.Errorf("Failed to update status for instance '%s': %w", UUID, err))
		}
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
	UUIDString := r.PathValue("uuid")

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

	currentMigrationStatus := override.DisableMigration

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

	// If migration status has changed, need to update the actual instance status.
	if currentMigrationStatus != override.DisableMigration {
		newStatus := api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION
		if !override.DisableMigration {
			newStatus = api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH
		}

		err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
			return d.db.UpdateInstanceStatus(tx, UUID, newStatus, newStatus.String(), true)
		})
		if err != nil {
			return response.BadRequest(fmt.Errorf("Failed to update status for instance '%s': %w", UUID, err))
		}
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
	UUIDString := r.PathValue("uuid")

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

	// When deleting an override, be sure to reset migration status if needed.
	if override.DisableMigration {
		err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
			return d.db.UpdateInstanceStatus(tx, UUID, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH, api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH.String(), true)
		})
		if err != nil {
			return response.BadRequest(fmt.Errorf("Failed to update status for instance '%s': %w", UUID, err))
		}
	}

	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.DeleteInstanceOverride(tx, UUID)
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to delete override for instance '%s': %w", UUID, err))
	}

	return response.EmptySyncResponse
}
