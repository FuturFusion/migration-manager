package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/FuturFusion/migration-manager/internal/batch"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var batchesCmd = APIEndpoint{
	Path: "batches",

	Get:  APIEndpointAction{Handler: batchesGet, AllowUntrusted: true},
	Post: APIEndpointAction{Handler: batchesPost, AccessHandler: allowAuthenticated},
}

var batchCmd = APIEndpoint{
	Path: "batches/{name}",

	Delete: APIEndpointAction{Handler: batchDelete, AccessHandler: allowAuthenticated},
	Get:    APIEndpointAction{Handler: batchGet, AllowUntrusted: true},
	Put:    APIEndpointAction{Handler: batchPut, AccessHandler: allowAuthenticated},
}

var batchInstancesCmd = APIEndpoint{
	Path: "batches/{name}/instances",

	Get: APIEndpointAction{Handler: batchInstancesGet, AllowUntrusted: true},
}

var batchStartCmd = APIEndpoint{
	Path: "batches/{name}/start",

	Post: APIEndpointAction{Handler: batchStartPost, AccessHandler: allowAuthenticated},
}

var batchStopCmd = APIEndpoint{
	Path: "batches/{name}/stop",

	Post: APIEndpointAction{Handler: batchStopPost, AccessHandler: allowAuthenticated},
}

// swagger:operation GET /1.0/batches batches batches_get
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
//	          description: List of sources
//	          items:
//	            $ref: "#/definitions/Batch"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func batchesGet(d *Daemon, r *http.Request) response.Response {
	result := []batch.Batch{}
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		batches, err := d.db.GetAllBatches(tx)
		if err != nil {
			return err
		}

		result = batches
		return nil
	})
	if err != nil {
		return response.SmartError(err)
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
	var b batch.InternalBatch

	// Decode into the new batch.
	err := json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		return response.BadRequest(err)
	}

	// Insert into database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.AddBatch(tx, &b)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating batch %q: %w", b.GetName(), err))
	}

	// Add any instances to this batch that match selection criteria.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateInstancesAssignedToBatch(tx, &b)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to assign instances to batch %q: %w", b.GetName(), err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/batches/"+b.GetName())
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

	if name == "" {
		return response.BadRequest(fmt.Errorf("Batch name cannot be empty"))
	}

	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.DeleteBatch(tx, name)
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to delete batch '%s': %w", name, err))
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

	if name == "" {
		return response.BadRequest(fmt.Errorf("Batch name cannot be empty"))
	}

	var b batch.Batch
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbBatch, err := d.db.GetBatch(tx, name)
		if err != nil {
			return err
		}

		b = dbBatch
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get batch '%s': %w", name, err))
	}

	return response.SyncResponseETag(true, b, b)
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

	if name == "" {
		return response.BadRequest(fmt.Errorf("Batch name cannot be empty"))
	}

	// Get the existing batch.
	var b batch.Batch
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbBatch, err := d.db.GetBatch(tx, name)
		if err != nil {
			return err
		}

		b = dbBatch
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get batch '%s': %w", name, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, b)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	// Decode into the existing batch.
	err = json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		return response.BadRequest(err)
	}

	// Update batch in the database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateBatch(tx, b)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating batch %q: %w", b.GetName(), err))
	}

	// Update any instances for this batch that match selection criteria.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateInstancesAssignedToBatch(tx, b)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to update instances for batch %q: %w", b.GetName(), err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/batches/"+b.GetName())
}

// swagger:operation GET /1.0/batches/{name}/instances batches batches_instances_get
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
//	          description: List of sources
//	          items:
//	            $ref: "#/definitions/Instance"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func batchInstancesGet(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	if name == "" {
		return response.BadRequest(fmt.Errorf("Batch name cannot be empty"))
	}

	var b batch.Batch
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbBatch, err := d.db.GetBatch(tx, name)
		if err != nil {
			return err
		}

		b = dbBatch
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get batch '%s': %w", name, err))
	}

	result := []instance.Instance{}
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		id, err := b.GetDatabaseID()
		if err != nil {
			return err
		}

		result, err = d.db.GetAllInstancesForBatchID(tx, id)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
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

	if name == "" {
		return response.BadRequest(fmt.Errorf("Batch name cannot be empty"))
	}

	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		err := d.db.StartBatch(tx, name)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to start batch '%s': %w", name, err))
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

	if name == "" {
		return response.BadRequest(fmt.Errorf("Batch name cannot be empty"))
	}

	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		err := d.db.StopBatch(tx, name)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to stop batch '%s': %w", name, err))
	}

	return response.SyncResponse(true, nil)
}
