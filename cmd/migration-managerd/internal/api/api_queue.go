package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var queueRootCmd = APIEndpoint{
	Path: "queue",

	Get: APIEndpointAction{Handler: queueRootGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var queueCmd = APIEndpoint{
	Path: "queue/{uuid}",

	Get:    APIEndpointAction{Handler: queueGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Delete: APIEndpointAction{Handler: queueDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var queueWorkerCmd = APIEndpoint{
	Path: "queue/{uuid}/worker",

	// Endpoints used by the migration worker which authenticates via a randomly-generated UUID unique to each instance.
	Post: APIEndpointAction{Handler: queueWorkerPost, AccessHandler: allowAuthenticated},
}

var queueWorkerCommandCmd = APIEndpoint{
	Path: "queue/{uuid}/worker/command",

	Post: APIEndpointAction{Handler: queueWorkerCommandPost, AccessHandler: allowAuthenticated},
}

// Authenticate a migration worker. Allow a GET for an existing instance so the worker can get its instructions,
// and for POST require the secret token to be valid when the worker reports back.
func (d *Daemon) workerAccessTokenValid(r *http.Request) bool {
	// Only allow GET and POST methods.
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return false
	}

	// Limit to just queue status updates
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		return false
	}

	if pathParts[2] != "queue" {
		return false
	}

	// Ensure we got a valid instance UUID.
	instanceUUID, err := uuid.Parse(pathParts[3])
	if err != nil {
		return false
	}

	// Get the instance.
	i, err := d.queue.GetByInstanceUUID(r.Context(), instanceUUID)
	if err != nil {
		return false
	}

	if r.Method == http.MethodPost {
		// Get the secret token.
		err = r.ParseForm()
		if err != nil {
			return false
		}

		secretUUID, err := uuid.Parse(r.Form.Get("secret"))
		if err != nil {
			return false
		}

		return secretUUID == i.SecretToken
	}

	// Allow a GET for a valid instance.
	return r.Method == http.MethodGet
}

// swagger:operation GET /1.0/queue queue queueRoot_get
//
//	Get the current migration queue
//
//	Returns a list of all migrations underway (URLs).
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
//	          description: List of migration items in the queue
//                items:
//                  type: string
//                example: |-
//                  [
//                    "/1.0/queue/26fa4eb7-8d4f-4bf8-9a6a-dd95d166dfad",
//                    "/1.0/queue/9aad7f16-0d2e-440e-872f-4e9df2d53367"
//                  ]
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/queue?recursion=1 queue queueRoot_get_recursion
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
	// Parse the recursion field.
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	var result []api.QueueEntry
	var paths []string
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		queueItems, err := d.queue.GetAll(r.Context())
		if err != nil {
			return err
		}

		if recursion == 1 {
			result = make([]api.QueueEntry, 0, len(queueItems))
			for _, queueItem := range queueItems {
				instance, err := d.instance.GetByUUID(ctx, queueItem.InstanceUUID)
				if err != nil {
					return err
				}

				result = append(result, queueItem.ToAPI(instance.Properties.Name, d.queueHandler.LastWorkerUpdate(queueItem.InstanceUUID)))
			}

			return nil
		}

		paths = make([]string, 0, len(queueItems))
		for _, queueItem := range queueItems {
			paths = append(paths, fmt.Sprintf("/%s/queue/%s", api.APIVersion, queueItem.InstanceUUID))
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	if recursion == 1 {
		return response.SyncResponse(true, result)
	}

	return response.SyncResponse(true, paths)
}

// swagger:operation GET /1.0/queue/{uuid} queue queue_get
//
//	Get migration entry from queue
//
//	Returns details about the specified queue entry.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Queue entry
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
//	          $ref: "#/definitions/QueueEntry"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func queueGet(d *Daemon, r *http.Request) response.Response {
	UUIDString := r.PathValue("uuid")

	UUID, err := uuid.Parse(UUIDString)
	if err != nil {
		return response.BadRequest(err)
	}

	var queueItem *migration.QueueEntry
	var instanceName string
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		instance, err := d.instance.GetByUUID(ctx, UUID)
		if err != nil {
			return err
		}

		queueItem, err = d.queue.GetByInstanceUUID(r.Context(), UUID)
		if err != nil {
			return err
		}

		instanceName = instance.Properties.Name

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(true, queueItem.ToAPI(instanceName, d.queueHandler.LastWorkerUpdate(queueItem.InstanceUUID)), queueItem)
}

// swagger:operation DELETE /1.0/queues/{name} queues queue_delete
//
//	Delete the queue
//
//	Removes the queue.
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
func queueDelete(d *Daemon, r *http.Request) response.Response {
	uuidStr := r.PathValue("uuid")
	queueUUID, err := uuid.Parse(uuidStr)
	if err != nil {
		return response.BadRequest(err)
	}

	err = d.queue.DeleteByUUID(r.Context(), queueUUID)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

// swagger:operation POST /1.0/queue/{uuid}/worker/command queue queue_worker_command_post
//
//	Generate next worker command for instance
//
//	Generates the next worker command, if any, for this queued instance.
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
func queueWorkerCommandPost(d *Daemon, r *http.Request) response.Response {
	uuidString := r.PathValue("uuid")

	instanceUUID, err := uuid.Parse(uuidString)
	if err != nil {
		return response.BadRequest(err)
	}

	workerCommand, err := d.queue.NewWorkerCommandByInstanceUUID(r.Context(), instanceUUID)
	if err != nil {
		return response.SmartError(err)
	}

	apiSourceJSON, err := json.Marshal(workerCommand.Source.ToAPI())
	if err != nil {
		return response.SmartError(err)
	}

	d.queueHandler.RecordWorkerUpdate(instanceUUID)
	return response.SyncResponseETag(true, api.WorkerCommand{
		Command:    workerCommand.Command,
		Location:   workerCommand.Location,
		SourceType: workerCommand.SourceType,
		Source:     apiSourceJSON,
		OS:         workerCommand.OS,
		OSVersion:  workerCommand.OSVersion,
	}, workerCommand)
}

// swagger:operation POST /1.0/queue/{uuid}/worker queue queue_worker_post
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
func queueWorkerPost(d *Daemon, r *http.Request) response.Response {
	uuidString := r.PathValue("uuid")

	instanceUUID, err := uuid.Parse(uuidString)
	if err != nil {
		return response.BadRequest(err)
	}

	// Decode the command response.
	var resp api.WorkerResponse
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		return response.BadRequest(err)
	}

	_, err = d.queue.ProcessWorkerUpdate(r.Context(), instanceUUID, resp.Status, resp.StatusMessage)
	if err != nil {
		return response.SmartError(err)
	}

	d.queueHandler.RecordWorkerUpdate(instanceUUID)
	return response.SyncResponse(true, nil)
}
