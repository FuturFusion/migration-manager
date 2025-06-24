package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

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
	var err error

	// Parse the recursion field.
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	if recursion == 1 {
		ctx, trans := transaction.Begin(r.Context())
		defer func() {
			rollbackErr := trans.Rollback()
			if rollbackErr != nil {
				response.SmartError(fmt.Errorf("Transaction rollback failed: %v, reason: %w", rollbackErr, err))
			}
		}()

		batches, err := d.batch.GetAll(ctx)
		if err != nil {
			return response.SmartError(err)
		}

		result := make([]api.Batch, 0, len(batches))

		for _, batch := range batches {
			windows, err := d.batch.GetMigrationWindows(ctx, batch.Name)
			if err != nil {
				return response.SmartError(err)
			}

			result = append(result, batch.ToAPI(windows))
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

	ctx, trans := transaction.Begin(r.Context())
	defer func() {
		rollbackErr := trans.Rollback()
		if rollbackErr != nil {
			response.SmartError(fmt.Errorf("Transaction rollback failed: %v, reason: %w", rollbackErr, err))
		}
	}()

	constraints := make([]migration.BatchConstraint, len(apiBatch.Constraints))
	for i, c := range apiBatch.Constraints {
		var duration time.Duration
		if c.MinInstanceBootTime != "" {
			duration, err = time.ParseDuration(c.MinInstanceBootTime)
			if err != nil {
				return response.SmartError(fmt.Errorf("Failed to parse min migration time for batch %q: %w", apiBatch.Name, err))
			}
		}

		constraints[i] = migration.BatchConstraint{
			Name:                   c.Name,
			Description:            c.Description,
			IncludeExpression:      c.IncludeExpression,
			MaxConcurrentInstances: c.MaxConcurrentInstances,
			MinInstanceBootTime:    duration,
		}
	}

	batch := migration.Batch{
		Name:              apiBatch.Name,
		Target:            apiBatch.Target,
		TargetProject:     apiBatch.TargetProject,
		Status:            api.BATCHSTATUS_DEFINED,
		StatusMessage:     string(api.BATCHSTATUS_DEFINED),
		StoragePool:       apiBatch.StoragePool,
		IncludeExpression: apiBatch.IncludeExpression,
		Constraints:       constraints,
	}

	_, err = d.batch.Create(ctx, batch)
	if err != nil {
		return response.SmartError(err)
	}

	windows := make(migration.MigrationWindows, len(apiBatch.MigrationWindows))
	for i, w := range apiBatch.MigrationWindows {
		windows[i] = migration.MigrationWindow{Start: w.Start, End: w.End, Lockout: w.Lockout}
	}

	err = d.batch.AssignMigrationWindows(ctx, batch.Name, windows)
	if err != nil {
		return response.SmartError(err)
	}

	err = trans.Commit()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed commit transaction: %w", err))
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
	var err error

	name := r.PathValue("name")

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

	windows, err := d.batch.GetMigrationWindows(ctx, name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(
		true,
		batch.ToAPI(windows),
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

	var batch api.BatchPut

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

	constraints := make([]migration.BatchConstraint, len(batch.Constraints))
	for i, c := range batch.Constraints {
		var duration time.Duration
		if c.MinInstanceBootTime != "" {
			duration, err = time.ParseDuration(c.MinInstanceBootTime)
			if err != nil {
				return response.SmartError(fmt.Errorf("Failed to parse min migration time for batch %q: %w", batch.Name, err))
			}
		}

		constraints[i] = migration.BatchConstraint{
			Name:                   c.Name,
			Description:            c.Description,
			IncludeExpression:      c.IncludeExpression,
			MaxConcurrentInstances: c.MaxConcurrentInstances,
			MinInstanceBootTime:    duration,
		}
	}

	// Get the existing batch.
	currentBatch, err := d.batch.GetByName(ctx, name)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get batch %q: %w", name, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, currentBatch)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	dbWindows, err := d.batch.GetMigrationWindows(ctx, batch.Name)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get current migration windows for batch %q: %w", batch.Name, err))
	}

	var changedWindows migration.MigrationWindows
	if len(dbWindows) != len(batch.MigrationWindows) {
		changedWindows = make(migration.MigrationWindows, len(batch.MigrationWindows))
		for i, w := range batch.MigrationWindows {
			changedWindows[i] = migration.MigrationWindow{Start: w.Start, End: w.End, Lockout: w.Lockout}
		}
	} else {
		windowMap := map[string]bool{}
		for _, w := range dbWindows {
			windowMap[w.Key()] = true
		}

		changed := false
		for _, w := range batch.MigrationWindows {
			newWindow := migration.MigrationWindow{Start: w.Start, End: w.End, Lockout: w.Lockout}
			if !windowMap[newWindow.Key()] {
				changed = true
				break
			}
		}

		if changed {
			changedWindows = migration.MigrationWindows{}
			for _, w := range batch.MigrationWindows {
				newWindow := migration.MigrationWindow{Start: w.Start, End: w.End, Lockout: w.Lockout}
				changedWindows = append(changedWindows, newWindow)
			}
		}
	}

	err = d.batch.Update(ctx, name, &migration.Batch{
		ID:                currentBatch.ID,
		Name:              batch.Name,
		Target:            batch.Target,
		TargetProject:     batch.TargetProject,
		Status:            currentBatch.Status,
		StatusMessage:     currentBatch.StatusMessage,
		StoragePool:       batch.StoragePool,
		IncludeExpression: batch.IncludeExpression,
		Constraints:       constraints,
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating batch %q: %w", batch.Name, err))
	}

	if changedWindows != nil {
		err := d.batch.ChangeMigrationWindows(ctx, batch.Name, changedWindows)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to update migration windows for batch %q: %w", batch.Name, err))
		}
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

	instances, err := d.instance.GetAllByBatch(ctx, batch.Name)
	if err != nil {
		return response.SmartError(err)
	}

	if recursion == 1 {
		result := make([]api.Instance, 0, len(instances))

		for _, instance := range instances {
			result = append(result, instance.ToAPI())
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
var batchStartLock sync.Mutex

func batchStartPost(d *Daemon, r *http.Request) response.Response {
	batchStartLock.Lock()
	defer batchStartLock.Unlock()
	batchName := r.PathValue("name")

	instances := map[uuid.UUID]migration.Instance{}
	var batch *migration.Batch
	var target *migration.Target
	var windows migration.MigrationWindows
	err := transaction.Do(r.Context(), func(ctx context.Context) error {
		var err error
		batch, err = d.batch.GetByName(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get batch %q: %w", batchName, err)
		}

		windows, err = d.batch.GetMigrationWindows(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get migration windows for batch %q: %w", batchName, err)
		}

		target, err = d.target.GetByName(ctx, batch.Target)
		if err != nil {
			return fmt.Errorf("Failed to get target for batch %q: %w", batch.Name, err)
		}

		queueEntries, err := d.queue.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get queue entries: %w", err)
		}

		queueMap := make(map[uuid.UUID]bool, len(queueEntries))
		for _, entry := range queueEntries {
			queueMap[entry.InstanceUUID] = true
		}

		batchInstances, err := d.instance.GetAllByBatch(ctx, batch.Name)
		if err != nil {
			return fmt.Errorf("Failed to get instances for batch %q: %w", batch.Name, err)
		}

		for _, inst := range batchInstances {
			if queueMap[inst.UUID] {
				slog.Warn("Instance is already queued in a different batch, ignoring", slog.String("batch", batchName), slog.String("instance", inst.Properties.Location))
				continue
			}

			instances[inst.UUID] = inst
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Validate that the batch can be queued.
	_, err = d.validateForQueue(r.Context(), *batch, windows, *target, instances)
	if err != nil {
		return response.SmartError(err)
	}

	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		err := d.batch.StartBatchByName(ctx, batchName)
		if err != nil {
			return err
		}

		for _, inst := range instances {
			secret, err := uuid.NewRandom()
			if err != nil {
				return err
			}

			status := api.MIGRATIONSTATUS_CREATING
			message := "Creating target instance definition"
			err = inst.DisabledReason()
			if err != nil {
				status = api.MIGRATIONSTATUS_BLOCKED
				message = err.Error()
			}

			_, err = d.queue.CreateEntry(ctx, migration.QueueEntry{
				InstanceUUID:           inst.UUID,
				BatchName:              batchName,
				NeedsDiskImport:        true,
				SecretToken:            secret,
				MigrationStatus:        status,
				MigrationStatusMessage: message,
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
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
