package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var batchesCmd = APIEndpoint{
	Path: "batches",

	Get:  APIEndpointAction{Handler: batchesGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Post: APIEndpointAction{Handler: batchesPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanCreate)},
}

var batchCmd = APIEndpoint{
	Path: "batches/{name}",

	Delete: APIEndpointAction{Handler: batchDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
	Get:    APIEndpointAction{Handler: batchGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put:    APIEndpointAction{Handler: batchPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

var batchInstancesCmd = APIEndpoint{
	Path: "batches/{name}/instances",

	Get: APIEndpointAction{Handler: batchInstancesGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var batchStartCmd = APIEndpoint{
	Path: "batches/{name}/start",

	Post: APIEndpointAction{Handler: batchStartPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanCreate)},
}

var batchStopCmd = APIEndpoint{
	Path: "batches/{name}/stop",

	Post: APIEndpointAction{Handler: batchStopPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
}

// swagger:operation GET /1.0/batches batches batches_get
//
//	Get the batches
//
//	Returns a list of batches (URLs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API batches
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
//	          description: List of batches
//                items:
//                  type: string
//                example: |-
//                  [
//                    "/1.0/batches/foo",
//                    "/1.0/batches/bar"
//                  ]
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/batches?recursion=1 batches batches_get_recursion
//
//	Get the batches
//
//	Returns a list of batches (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API batches
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
//	          description: List of batches
//	          items:
//	            $ref: "#/definitions/Batch"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func batchesGet(d *Daemon, r *http.Request) response.Response {
	// Parse the recursion field.
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	if recursion == 1 {
		batches, err := d.batch.GetAll(r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		result := make([]api.Batch, 0, len(batches))
		for _, batch := range batches {
			result = append(result, api.Batch{
				DatabaseID:           batch.ID,
				Name:                 batch.Name,
				TargetID:             batch.TargetID,
				TargetProject:        batch.TargetProject,
				Status:               batch.Status,
				StatusString:         batch.StatusString,
				StoragePool:          batch.StoragePool,
				IncludeExpression:    batch.IncludeExpression,
				MigrationWindowStart: batch.MigrationWindowStart,
				MigrationWindowEnd:   batch.MigrationWindowEnd,
			})
		}

		return response.SyncResponse(true, result)
	}

	batchNames, err := d.batch.GetAllNames(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	result := make([]string, 0, len(batchNames))
	for _, name := range batchNames {
		result = append(result, fmt.Sprintf("/%s/batches/%s", api.APIVersion, name))
	}

	return response.SyncResponse(true, result)
}

// swagger:operation POST /1.0/batches batches batches_post
//
//	Add a batch
//
//	Creates a new batch.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: batch
//	    description: Batch configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/Batch"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func batchesPost(d *Daemon, r *http.Request) response.Response {
	var apiBatch api.Batch

	// Decode into the new batch.
	err := json.NewDecoder(r.Body).Decode(&apiBatch)
	if err != nil {
		return response.BadRequest(err)
	}

	batch := migration.Batch{
		ID:                   apiBatch.DatabaseID,
		Name:                 apiBatch.Name,
		TargetID:             apiBatch.TargetID,
		TargetProject:        apiBatch.TargetProject,
		Status:               api.BATCHSTATUS_DEFINED,
		StatusString:         api.BATCHSTATUS_DEFINED.String(),
		StoragePool:          apiBatch.StoragePool,
		IncludeExpression:    apiBatch.IncludeExpression,
		MigrationWindowStart: apiBatch.MigrationWindowStart,
		MigrationWindowEnd:   apiBatch.MigrationWindowEnd,
	}

	_, err = d.batch.Create(r.Context(), batch)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/batches/"+batch.Name)
}

// swagger:operation DELETE /1.0/batches/{name} batches batch_delete
//
//	Delete the batch
//
//	Removes the batch.
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
func batchDelete(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	err := d.batch.DeleteByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/batches/{name} batches batch_get
//
//	Get the batch
//
//	Gets a specific batch.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Batch
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
//	          $ref: "#/definitions/Batch"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func batchGet(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	batch, err := d.batch.GetByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(
		true,
		api.Batch{
			DatabaseID:           batch.ID,
			Name:                 batch.Name,
			TargetID:             batch.TargetID,
			TargetProject:        batch.TargetProject,
			Status:               batch.Status,
			StatusString:         batch.StatusString,
			StoragePool:          batch.StoragePool,
			IncludeExpression:    batch.IncludeExpression,
			MigrationWindowStart: batch.MigrationWindowStart,
			MigrationWindowEnd:   batch.MigrationWindowEnd,
		},
		batch,
	)
}

// swagger:operation PUT /1.0/batches/{name} batches batch_put
//
//	Update the batch
//
//	Updates the batch definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: batch
//	    description: Batch definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/Batch"
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
func batchPut(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	var batch api.Batch

	// Decode into the existing batch.
	err := json.NewDecoder(r.Body).Decode(&batch)
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

	// Get the existing batch.
	currentBatch, err := d.batch.GetByName(ctx, name)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get batch %q: %w", name, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, currentBatch)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	_, err = d.batch.UpdateByID(ctx, migration.Batch{
		ID:                   currentBatch.ID,
		Name:                 batch.Name,
		TargetID:             batch.TargetID,
		TargetProject:        batch.TargetProject,
		Status:               batch.Status,
		StatusString:         batch.StatusString,
		StoragePool:          batch.StoragePool,
		IncludeExpression:    batch.IncludeExpression,
		MigrationWindowStart: batch.MigrationWindowStart,
		MigrationWindowEnd:   batch.MigrationWindowEnd,
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating batch %q: %w", batch.Name, err))
	}

	err = trans.Commit()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed commit transaction: %w", err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/batches/"+batch.Name)
}

// swagger:operation GET /1.0/batches/{name}/instances batches batches_instances_get
//
//	Get instances for the batch
//
//	Returns a list of instances assigned to this batch (URLs).
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
//                items:
//                  type: string
//                example: |-
//                  [
//                    "/1.0/instances/foo",
//                    "/1.0/instances/bar"
//                  ]
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/batches/{name}/instances?recursion=1 batches batches_instances_get_recursion
//
//	Get instances for the batch
//
//	Returns a list of instances assigned to this batch (structs).
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
//	            $ref: "#/definitions/Instance"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func batchInstancesGet(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	// Parse the recursion field.
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	ctx, trans := transaction.Begin(r.Context())
	defer func() {
		rollbackErr := trans.Rollback()
		if rollbackErr != nil {
			response.SmartError(fmt.Errorf("Transaction rollback failed: %v, reason: %w", rollbackErr, err))
		}
	}()

	batch, err := d.batch.GetByName(ctx, name)
	if err != nil {
		return response.SmartError(err)
	}

	instances, err := d.instance.GetAllByBatchID(ctx, batch.ID)
	if err != nil {
		return response.SmartError(err)
	}

	if recursion == 1 {
		result := make([]api.Instance, 0, len(instances))
		for _, instance := range instances {
			apiInstance := api.Instance{
				UUID:                  instance.UUID,
				InventoryPath:         instance.InventoryPath,
				Annotation:            instance.Annotation,
				MigrationStatus:       instance.MigrationStatus,
				MigrationStatusString: instance.MigrationStatusString,
				LastUpdateFromSource:  instance.LastUpdateFromSource,
				SourceID:              instance.SourceID,
				TargetID:              instance.TargetID,
				BatchID:               instance.BatchID,
				GuestToolsVersion:     instance.GuestToolsVersion,
				Architecture:          instance.Architecture,
				HardwareVersion:       instance.HardwareVersion,
				OS:                    instance.OS,
				OSVersion:             instance.OSVersion,
				Devices:               instance.Devices,
				Disks:                 instance.Disks,
				NICs:                  instance.NICs,
				Snapshots:             instance.Snapshots,
				CPU:                   instance.CPU,
				Memory:                instance.Memory,
				UseLegacyBios:         instance.UseLegacyBios,
				SecureBootEnabled:     instance.SecureBootEnabled,
				TPMPresent:            instance.TPMPresent,
			}

			if instance.Overrides != nil {
				apiInstance.Overrides = api.InstanceOverride(*instance.Overrides)
			}

			result = append(result, apiInstance)
		}

		return response.SyncResponse(true, result)
	}

	result := make([]string, 0, len(instances))
	for _, instance := range instances {
		result = append(result, fmt.Sprintf("/%s/instances/%s", api.APIVersion, instance.UUID))
	}

	return response.SyncResponse(true, result)
}

// swagger:operation POST /1.0/batches/{name}/start batches batches_start_post
//
//	Start a batch
//
//	Starts a batch and begins the migration process for its instances.
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
func batchStartPost(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	err := d.batch.StartBatchByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, nil)
}

// swagger:operation POST /1.0/batches/{name}/stop batches batches_stop_post
//
//	Stop a batch
//
//	Stops a batch and suspends the migration process for its instances.
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
func batchStopPost(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	err := d.batch.StopBatchByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, nil)
}
