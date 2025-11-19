package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/target"
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
	Path: "batches/{name}/:start",

	Post: APIEndpointAction{Handler: batchStartPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanCreate)},
}

var batchStopCmd = APIEndpoint{
	Path: "batches/{name}/:stop",

	Post: APIEndpointAction{Handler: batchStopPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
}

var batchResetCmd = APIEndpoint{
	Path: "batches/{name}/:reset",

	Post: APIEndpointAction{Handler: batchResetPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
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
			windows, err := d.window.GetAllByBatch(ctx, batch.Name)
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

	if apiBatch.Defaults.Placement.Target == "" {
		apiBatch.Defaults.Placement.Target = api.DefaultTarget
	}

	if apiBatch.Defaults.Placement.TargetProject == "" {
		apiBatch.Defaults.Placement.TargetProject = api.DefaultTargetProject
	}

	if apiBatch.Defaults.Placement.StoragePool == "" {
		apiBatch.Defaults.Placement.StoragePool = api.DefaultStoragePool
	}

	if apiBatch.Config.BackgroundSyncInterval == "" {
		apiBatch.Config.BackgroundSyncInterval = (time.Minute * 10).String()
	}

	if apiBatch.Config.FinalBackgroundSyncLimit == "" {
		apiBatch.Config.FinalBackgroundSyncLimit = (time.Minute * 10).String()
	}

	batch := migration.Batch{
		Name:              apiBatch.Name,
		Status:            api.BATCHSTATUS_DEFINED,
		StatusMessage:     string(api.BATCHSTATUS_DEFINED),
		IncludeExpression: apiBatch.IncludeExpression,
		Defaults:          apiBatch.Defaults,
		Constraints:       apiBatch.Constraints,
		Config:            apiBatch.Config,
	}

	_, err = d.batch.Create(ctx, batch)
	if err != nil {
		return response.SmartError(err)
	}

	for _, w := range apiBatch.MigrationWindows {
		window := migration.Window{
			Name:    w.Name,
			Start:   w.Start,
			End:     w.End,
			Lockout: w.Lockout,
			Batch:   apiBatch.Name,
			Config:  w.Config,
		}

		_, err = d.window.Create(ctx, window)
		if err != nil {
			return response.SmartError(err)
		}
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

	windows, err := d.window.GetAllByBatch(ctx, name)
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

	err = d.batch.Update(ctx, d.queue, name, &migration.Batch{
		ID:                currentBatch.ID,
		Name:              batch.Name,
		Status:            currentBatch.Status,
		StatusMessage:     currentBatch.StatusMessage,
		IncludeExpression: batch.IncludeExpression,
		StartDate:         currentBatch.StartDate,
		Constraints:       batch.Constraints,
		Config:            batch.Config,
		Defaults:          batch.Defaults,
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating batch %q: %w", batch.Name, err))
	}

	windows := make(migration.Windows, 0, len(batch.MigrationWindows))
	for _, w := range batch.MigrationWindows {
		windows = append(windows, migration.Window{
			Name:    w.Name,
			Start:   w.Start,
			End:     w.End,
			Lockout: w.Lockout,
			Batch:   batch.Name,
			Config:  w.Config,
		})
	}

	err = d.window.ReplaceByBatch(ctx, d.queue, batch.Name, windows)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to update migration windows for batch %q: %w", batch.Name, err))
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

	err := d.WaitForSchemaUpdate(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	batchName := r.PathValue("name")

	// Check the worker endpoint is valid before starting the batch.
	workerURL, err := url.ParseRequestURI(d.getWorkerEndpoint())
	if err != nil {
		return response.SmartError(fmt.Errorf("Cannot start batch, worker endpoint %q is invalid: %w", d.getWorkerEndpoint(), err))
	}

	if workerURL.Hostname() == "" || workerURL.Hostname() == "0.0.0.0" || workerURL.Hostname() == "::" {
		return response.SmartError(fmt.Errorf("Worker endpoint cannot use a wildcard address: %q", d.getWorkerEndpoint()))
	}

	instances := map[uuid.UUID]migration.Instance{}
	var batch *migration.Batch
	var windows migration.Windows
	var networks migration.Networks
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		var err error
		batch, err = d.batch.GetByName(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get batch %q: %w", batchName, err)
		}

		windows, err = d.window.GetAllByBatch(ctx, batchName)
		if err != nil {
			return fmt.Errorf("Failed to get migration windows for batch %q: %w", batchName, err)
		}

		networks, err = d.network.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("Failed to get networks for batch %q: %w", batch.Name, err)
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

	// Validate that the batch can even start before doing more work.
	if batch.Status != api.BATCHSTATUS_DEFINED {
		return response.SmartError(fmt.Errorf("Batch %q in state %q cannot be started", batch.Name, string(batch.Status)))
	}

	if len(instances) == 0 {
		return response.SmartError(fmt.Errorf("Cannot start batch %q with no instances", batch.Name))
	}

	err = windows.HasValidWindow()
	if err != nil {
		return response.SmartError(fmt.Errorf("Cannot start batch %q with invalid migration windows: %w", batch.Name, err))
	}

	placementsByUUID := map[uuid.UUID]api.Placement{}
	for _, inst := range instances {
		usedNetworks := migration.FilterUsedNetworks(networks, migration.Instances{inst})
		placement, err := d.batch.DeterminePlacement(r.Context(), inst, usedNetworks, *batch, windows)
		if err != nil {
			return response.SmartError(fmt.Errorf("Failed to run scriptlet for instance %q: %w", inst.Properties.Location, err))
		}

		placementsByUUID[inst.UUID] = *placement
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

			status := api.MIGRATIONSTATUS_WAITING
			message := "Preparing for migration"
			err = inst.DisabledReason(batch.Config.RestrictionOverrides)
			if err != nil {
				status = api.MIGRATIONSTATUS_BLOCKED
				message = err.Error()
			}

			_, err = d.queue.CreateEntry(ctx, migration.QueueEntry{
				InstanceUUID:           inst.UUID,
				BatchName:              batchName,
				ImportStage:            migration.IMPORTSTAGE_BACKGROUND,
				SecretToken:            secret,
				MigrationStatus:        status,
				MigrationStatusMessage: message,
				Placement:              placementsByUUID[inst.UUID],
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
	// Exclusively grab the worker lock so migration actions don't interfere.
	workerLock.Lock()
	defer workerLock.Unlock()

	name := r.PathValue("name")

	err := d.batch.StopBatchByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, nil)
}

// swagger:operation POST /1.0/batches/{name}/reset batches batches_reset_post
//
//	Reset a batch
//
//	Resets a batch, removes all queue entries, and cleans up incomplete target VMs and volumes.
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
func batchResetPost(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")
	err := transaction.Do(r.Context(), func(ctx context.Context) error {
		// Get a record of all queue entries before we wipe the records.
		entries, err := d.queue.GetAllByBatch(ctx, name)
		if err != nil {
			return err
		}

		err = d.batch.ResetBatchByName(ctx, name, d.queue, d.source, d.target)
		if err != nil {
			return err
		}

		// Get the list of VMs to to clean up.
		instances, err := d.instance.GetAllQueued(ctx, entries)
		if err != nil {
			return err
		}

		instMap := make(map[uuid.UUID]migration.Instance, len(instances))
		for _, inst := range instances {
			instMap[inst.UUID] = inst
		}

		// Get the list of targets for the VMs to clean up according to the queue entry placement.
		targets, err := d.target.GetAll(ctx)
		if err != nil {
			return err
		}

		targetMap := make(map[string]migration.Target, len(targets))
		for _, t := range targets {
			targetMap[t.Name] = t
		}

		for _, q := range entries {
			inst := instMap[q.InstanceUUID]
			t, ok := targetMap[q.Placement.TargetName]
			if !ok {
				continue
			}

			it, err := target.NewTarget(t.ToAPI())
			if err != nil {
				return err
			}

			err = it.Connect(ctx)
			if err != nil {
				return err
			}

			err = it.SetProject(q.Placement.TargetProject)
			if err != nil {
				return err
			}

			// Only remove VMs with a worker volume,
			// in case we are resetting a completed or errored batch where some VMs have already completed migration.
			err = it.CleanupVM(ctx, inst.GetName(), true)
			if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
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
