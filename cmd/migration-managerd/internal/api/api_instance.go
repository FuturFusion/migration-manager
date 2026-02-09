package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
	"github.com/FuturFusion/migration-manager/shared/api/event"
)

var instancesCmd = APIEndpoint{
	Path: "instances",

	Get: APIEndpointAction{Handler: instancesGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var instanceCmd = APIEndpoint{
	Path: "instances/{uuid}",

	Get: APIEndpointAction{Handler: instanceGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView), Authenticator: TokenAuthenticate},
}

var instanceOverrideCmd = APIEndpoint{
	Path: "instances/{uuid}/override",

	Delete: APIEndpointAction{Handler: instanceOverrideDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
	Get:    APIEndpointAction{Handler: instanceOverrideGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put:    APIEndpointAction{Handler: instanceOverridePut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

var instanceResetBackgroundImportCmd = APIEndpoint{
	Path: "instances/{uuid}/:reset-background-import",

	Post: APIEndpointAction{Handler: instanceResetBackgroundImport, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

// swagger:operation GET /1.0/instances instances instances_get
//
//	Get the instances
//
//	Returns a list of instances (URLs).
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
//	          description: List of instances
//	          items:
//	            type: string
//	          example: |-
//	            [
//	              "/1.0/instances/26fa4eb7-8d4f-4bf8-9a6a-dd95d166dfad",
//	              "/1.0/instances/9aad7f16-0d2e-440e-872f-4e9df2d53367"
// 	            ]
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/instances?recursion=1 instances instances_get_recursion
//
//	Get the instances
//
//	Returns a list of instances (structs).
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: include_expression
//	    description: An expression used to select instances to add to the result
//	    type: string
//	    example: name matches 'centos'
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
	// Parse the recursion field.
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	includeExpression := r.FormValue("include_expression")

	// Set recursion to 1 whenever 'include_expression' is used.
	if includeExpression != "" {
		recursion = 1
	}

	if recursion == 1 {
		ctx, trans := transaction.Begin(r.Context())
		defer func() {
			rollbackErr := trans.Rollback()
			if rollbackErr != nil {
				response.SmartError(fmt.Errorf("Transaction rollback failed: %v, reason: %w", rollbackErr, err))
			}
		}()

		instances, err := d.instance.GetAll(ctx)
		if err != nil {
			return response.SmartError(err)
		}

		result := make([]api.Instance, 0, len(instances))
		for _, instance := range instances {
			if includeExpression == "" {
				result = append(result, instance.ToAPI())
				continue
			}

			match, err := instance.MatchesCriteria(includeExpression)
			if err != nil {
				return response.SmartError(err)
			}

			if match {
				result = append(result, instance.ToAPI())
			}
		}

		// Sort the list by source, then location.
		sort.Slice(result, func(i, j int) bool {
			if result[i].Source == result[j].Source {
				return result[i].Location < result[j].Location
			}

			return result[i].Source < result[j].Source
		})

		return response.SyncResponse(true, result)
	}

	instanceUUIDs, err := d.instance.GetAllUUIDs(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	result := make([]string, 0, len(instanceUUIDs))
	for _, UUID := range instanceUUIDs {
		result = append(result, fmt.Sprintf("/%s/instances/%s", api.APIVersion, UUID))
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
		return response.BadRequest(err)
	}

	ctx, trans := transaction.Begin(r.Context())
	defer func() {
		rollbackErr := trans.Rollback()
		if rollbackErr != nil {
			response.SmartError(fmt.Errorf("Transaction rollback failed: %v, reason: %w", rollbackErr, err))
		}
	}()

	instance, err := d.instance.GetByUUID(ctx, UUID)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get instance %q: %w", UUID, err))
	}

	return response.SyncResponseETag(
		true,
		instance.ToAPI(),
		instance,
	)
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
		return response.BadRequest(err)
	}

	instance, err := d.instance.GetByUUID(r.Context(), UUID)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get override for instance %q: %w", UUID, err))
	}

	return response.SyncResponseETag(
		true,
		instance.Overrides,
		instance.Overrides,
	)
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
		return response.BadRequest(err)
	}

	// Decode into the existing instance override.
	var override api.InstanceOverride
	err = json.NewDecoder(r.Body).Decode(&override)
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

	// Get the existing instance override.
	currentInstance, err := d.instance.GetByUUID(ctx, UUID)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get instance %q: %w", UUID, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, currentInstance)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	override.LastUpdate = time.Now().UTC()
	currentInstance.Overrides = override

	err = d.instance.Update(ctx, currentInstance)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating override for instance %q: %w", UUID, err))
	}

	err = trans.Commit()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed commit transaction: %w", err))
	}

	apiInstance := currentInstance.ToAPI()
	d.logHandler.SendLifecycle(r.Context(), event.NewInstanceEvent(event.InstanceOverrideModified, r, apiInstance, apiInstance.UUID))

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/instances/"+UUIDString+"/override")
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
	uuidString := r.PathValue("uuid")

	instanceUUID, err := uuid.Parse(uuidString)
	if err != nil {
		return response.BadRequest(err)
	}

	var apiInstance api.Instance
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		inst, err := d.instance.GetByUUID(ctx, instanceUUID)
		if err != nil {
			return err
		}

		inst.Overrides = api.InstanceOverride{}

		apiInstance = inst.ToAPI()

		return d.instance.Update(ctx, inst)
	})
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewInstanceEvent(event.InstanceOverrideModified, r, apiInstance, apiInstance.UUID))

	return response.EmptySyncResponse
}

// swagger:operation POST /1.0/instances/{uuid}/:reset-background-import instances instance_reset_background_import
//
//	Reactivates instance background import support
//
//	Resets background import verification for an instance whose source reports background import support, but could not be verified..
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
func instanceResetBackgroundImport(d *Daemon, r *http.Request) response.Response {
	uuidString := r.PathValue("uuid")

	instanceUUID, err := uuid.Parse(uuidString)
	if err != nil {
		return response.BadRequest(err)
	}

	var apiInstance api.Instance
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		inst, err := d.instance.GetByUUID(ctx, instanceUUID)
		if err != nil {
			return err
		}

		src, err := d.source.GetByName(ctx, inst.Source)
		if err != nil {
			return err
		}

		is, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
		if err != nil {
			return err
		}

		err = is.Connect(ctx)
		if err != nil {
			return err
		}

		supported, err := is.GetBackgroundImport(ctx, inst.UUID)
		if err != nil {
			return err
		}

		if !supported {
			return fmt.Errorf("Instance %q (%q) on source %q does not have background import support", inst.UUID, inst.Properties.Location, src.Name)
		}

		err = d.instance.ResetBackgroundImport(ctx, inst)
		if err != nil {
			return err
		}

		apiInstance = inst.ToAPI()
		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewInstanceEvent(event.InstanceModified, r, apiInstance, apiInstance.UUID))

	return response.EmptySyncResponse
}
