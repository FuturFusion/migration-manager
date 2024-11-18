package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var queueRootCmd = APIEndpoint{
	Path: "queue",

	Get:  APIEndpointAction{Handler: queueRootGet, AllowUntrusted: true},
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
		return response.BadRequest(fmt.Errorf("Failed to get batches: %w", err))
	}

	// For each batch that has entered the "queued" state or later, add its instances.
	for _, b := range batches {
		if b.GetStatus() == api.BATCHSTATUS_UNKNOWN || b.GetStatus() == api.BATCHSTATUS_DEFINED || b.GetStatus() == api.BATCHSTATUS_READY {
			continue
		}

		id, err := b.GetDatabaseID()
		if err != nil {
			return response.BadRequest(fmt.Errorf("Failed to get database ID for batch '%s': %w", b.GetName(), err))
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
			return response.BadRequest(fmt.Errorf("Failed to get instances for batch '%s': %w", b.GetName(), err))
		}

		for _, i := range instances {
			result = append(result, api.QueueEntry{
				InstanceUUID: i.GetUUID(),
				InstanceName: i.GetName(),
				MigrationStatus: i.GetMigrationStatus(),
				MigrationStatusString: i.GetMigrationStatusString(),
				BatchID: id,
				BatchName: b.GetName(),
			})
		}
	}

	return response.SyncResponse(true, result)
}
