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

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var queueRootCmd = APIEndpoint{
	Path: "queue",

	Get: APIEndpointAction{Handler: queueRootGet, AllowUntrusted: true},
}

var queueCmd = APIEndpoint{
	Path: "queue/{uuid}",

	Get: APIEndpointAction{Handler: queueGet, AllowUntrusted: true},
	Put: APIEndpointAction{Handler: queuePut, AllowUntrusted: true},
}

// swagger:operation GET /1.0/queue queue queueRoot_get
//
//	Get the current migration queue
//
//	Returns a list of all migrations underway (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Migration queue instances
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
//	            $ref: "#/definitions/QueueEntry"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func queueRootGet(d *Daemon, r *http.Request) response.Response {
	result := []api.QueueEntry{}

	// Get all batches.
	var batches []batch.Batch
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbBatches, err := d.db.GetAllBatches(tx)
		if err != nil {
			return err
		}

		batches = dbBatches
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("failed to get batches: %w", err))
	}

	// For each batch that has entered the "queued" state or later, add its instances.
	for _, b := range batches {
		if b.GetStatus() == api.BATCHSTATUS_UNKNOWN || b.GetStatus() == api.BATCHSTATUS_DEFINED || b.GetStatus() == api.BATCHSTATUS_READY {
			continue
		}

		id, err := b.GetDatabaseID()
		if err != nil {
			return response.BadRequest(fmt.Errorf("failed to get database ID for batch '%s': %w", b.GetName(), err))
		}

		var instances []instance.Instance
		err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
			dbInstances, err := d.db.GetAllInstancesForBatchID(tx, id)
			if err != nil {
				return err
			}

			instances = dbInstances
			return nil
		})
		if err != nil {
			return response.BadRequest(fmt.Errorf("failed to get instances for batch '%s': %w", b.GetName(), err))
		}

		for _, i := range instances {
			result = append(result, api.QueueEntry{
				InstanceUUID:          i.GetUUID(),
				InstanceName:          i.GetName(),
				MigrationStatus:       i.GetMigrationStatus(),
				MigrationStatusString: i.GetMigrationStatusString(),
				BatchID:               id,
				BatchName:             b.GetName(),
			})
		}
	}

	return response.SyncResponse(true, result)
}

// swagger:operation GET /1.0/queue/{uuid} queue queue_get
//
//	Get worker command for instance
//
//	Gets a worker command, if any, for this queued instance.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: WorkerCommand
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
//	          $ref: "#/definitions/WorkerCommand"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func queueGet(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(err)
	}

	// Get the instance.
	var i *instance.InternalInstance
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbInstance, err := d.db.GetInstance(tx, UUID)
		if err != nil {
			return err
		}

		internalInstance, ok := dbInstance.(*instance.InternalInstance)
		if !ok {
			return fmt.Errorf("wasn't given an InternalInstance?")
		}
		i = internalInstance
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("failed to get instance '%s': %w", UUID, err))
	}

	// Don't return info for instances that aren't in the migration queue.
	if i.GetBatchID() == internal.INVALID_DATABASE_ID || !i.IsMigrating() {
		return response.BadRequest(fmt.Errorf("instance '%s' isn't in the migration queue", i.GetName()))
	}

	// If the instance is already doing something, don't start something else.
	if i.MigrationStatus != api.MIGRATIONSTATUS_IDLE {
		return response.BadRequest(fmt.Errorf("instance '%s' isn't idle: %s (%s)", i.Name, i.MigrationStatus.String(), i.MigrationStatusString))
	}

	// Setup the default "idle" command
	cmd := api.WorkerCommand{
		Command:   api.WORKERCOMMAND_IDLE,
		Name:      i.Name,
		Source:    api.VMwareSource{},
		OS:        i.OS,
		OSVersion: i.OSVersion,
	}

	// Fetch the source for the instance.
	var s api.VMwareSource
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbSource, err := d.db.GetSourceByID(tx, i.SourceID)
		if err != nil {
			return err
		}

		encodedSource, err := json.Marshal(dbSource)
		if err != nil {
			return err
		}

		return json.Unmarshal(encodedSource, &s)
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("failed to get source '%s': %w", UUID, err))
	}

	// If we can do a background disk sync, kick it off if needed
	if i.NeedsDiskImport && i.Disks[0].DifferentialSyncSupported {
		cmd.Command = api.WORKERCOMMAND_IMPORT_DISKS
		cmd.Source = s

		i.MigrationStatus = api.MIGRATIONSTATUS_BACKGROUND_IMPORT
		i.MigrationStatusString = i.MigrationStatus.String()
	} else if !i.NeedsDiskImport || !i.Disks[0].DifferentialSyncSupported {
		// FIXME need finer-grained logic before kicking off finalization step.
		cmd.Command = api.WORKERCOMMAND_FINALIZE_IMPORT
		cmd.Source = s

		i.MigrationStatus = api.MIGRATIONSTATUS_FINAL_IMPORT
		i.MigrationStatusString = api.MIGRATIONSTATUS_FINAL_IMPORT.String()
	}

	// Update instance in the database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateInstanceStatus(tx, UUID, i.MigrationStatus, i.MigrationStatusString, i.NeedsDiskImport)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("failed updating instance '%s': %w", i.GetUUID(), err))
	}

	return response.SyncResponseETag(true, cmd, cmd)
}

// swagger:operation PUT /1.0/queue/{uuid} queue queue_put
//
//	Sets worker response for instance
//
//	Sets the response from the worker for this queued instance.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: response
//	    description: WorkerResponse definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/WorkerResponse"
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
func queuePut(d *Daemon, r *http.Request) response.Response {
	UUIDString, err := url.PathUnescape(mux.Vars(r)["uuid"])
	if err != nil {
		return response.SmartError(err)
	}

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.SmartError(err)
	}

	// Get the instance.
	var i *instance.InternalInstance
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbInstance, err := d.db.GetInstance(tx, UUID)
		if err != nil {
			return err
		}

		internalInstance, ok := dbInstance.(*instance.InternalInstance)
		if !ok {
			return fmt.Errorf("wasn't given an InternalInstance?")
		}
		i = internalInstance
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("failed to get instance '%s': %w", UUID, err))
	}

	// Don't update instances that aren't in the migration queue.
	if i.GetBatchID() == internal.INVALID_DATABASE_ID || !i.IsMigrating() {
		return response.BadRequest(fmt.Errorf("instance '%s' isn't in the migration queue", i.GetName()))
	}

	// Decode the command response.
	var resp api.WorkerResponse
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		return response.BadRequest(err)
	}

	// Process the response.
	switch resp.Status {
	case api.WORKERRESPONSE_RUNNING:
		i.MigrationStatusString = resp.StatusString
	case api.WORKERRESPONSE_SUCCESS:
		switch i.MigrationStatus {
		case api.MIGRATIONSTATUS_BACKGROUND_IMPORT:
			i.NeedsDiskImport = false
			i.MigrationStatus = api.MIGRATIONSTATUS_IDLE
			i.MigrationStatusString = api.MIGRATIONSTATUS_IDLE.String()
		case api.MIGRATIONSTATUS_FINAL_IMPORT:
			i.MigrationStatus = api.MIGRATIONSTATUS_IMPORT_COMPLETE
			i.MigrationStatusString = api.MIGRATIONSTATUS_IMPORT_COMPLETE.String()
		}
	case api.WORKERRESPONSE_FAILED:
		i.MigrationStatus = api.MIGRATIONSTATUS_ERROR
		i.MigrationStatusString = resp.StatusString
	}

	// Update instance in the database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateInstanceStatus(tx, UUID, i.MigrationStatus, i.MigrationStatusString, i.NeedsDiskImport)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("failed updating instance '%s': %w", i.GetUUID(), err))
	}

	return response.SyncResponse(true, nil)
}
