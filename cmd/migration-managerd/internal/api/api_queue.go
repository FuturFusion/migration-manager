package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"

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
		queueItems, err := d.queue.GetAll(ctx)
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

				var migrationWindow *migration.MigrationWindow
				windowID := queueItem.GetWindowID()
				if windowID != nil {
					migrationWindow, err = d.batch.GetMigrationWindow(ctx, *windowID)
					if err != nil {
						return err
					}
				} else if queueItem.StatusBeforeMigrationWindow() {
					// If the queue entry hasn't reached the point of being assigned a migration window, assume it will use the next available window according to its batch constraints.
					migrationWindow, err = d.queue.GetNextWindow(ctx, queueItem)
					if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
						return err
					}
				}

				if migrationWindow == nil {
					migrationWindow = &migration.MigrationWindow{}
				}

				result = append(result, queueItem.ToAPI(instance.GetName(), d.queueHandler.LastWorkerUpdate(queueItem.InstanceUUID), *migrationWindow))
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
		// Sort the queue list by batch, then instance name.
		sort.Slice(result, func(i, j int) bool {
			if result[i].BatchName == result[j].BatchName {
				return result[i].InstanceName < result[j].InstanceName
			}

			return result[i].BatchName < result[j].BatchName
		})

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
	var migrationWindow *migration.MigrationWindow
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		instance, err := d.instance.GetByUUID(ctx, UUID)
		if err != nil {
			return err
		}

		queueItem, err = d.queue.GetByInstanceUUID(ctx, UUID)
		if err != nil {
			return err
		}

		windowID := queueItem.GetWindowID()
		if windowID != nil {
			migrationWindow, err = d.batch.GetMigrationWindow(ctx, *windowID)
			if err != nil {
				return err
			}
		} else if queueItem.StatusBeforeMigrationWindow() {
			// If the queue entry hasn't reached the point of being assigned a migration window, assume it will use the next available window according to its batch constraints.
			migrationWindow, err = d.queue.GetNextWindow(ctx, *queueItem)
			if err != nil && !incusAPI.StatusErrorCheck(err, http.StatusNotFound) {
				return err
			}
		}

		instanceName = instance.GetName()

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	if migrationWindow == nil {
		migrationWindow = &migration.MigrationWindow{}
	}

	return response.SyncResponseETag(true, queueItem.ToAPI(instanceName, d.queueHandler.LastWorkerUpdate(queueItem.InstanceUUID), *migrationWindow), queueItem)
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
