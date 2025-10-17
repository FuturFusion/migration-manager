package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var workerUpdateCmd = APIEndpoint{
	Path: "worker/{uuid}/:update",

	Post: APIEndpointAction{Handler: workerUpdatePost, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
}

var workerCommandCmd = APIEndpoint{
	Path: "worker/{uuid}/:command",

	Post: APIEndpointAction{Handler: workerCommandPost, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
}

func instanceUUIDFromRequestURL(r *http.Request) (uuid.UUID, error) {
	// Only allow GET and POST methods.
	if r.Method != http.MethodPost {
		return uuid.Nil, fmt.Errorf("Expected method %q, but received method %q", http.MethodPost, r.Method)
	}

	// Limit to just worker status updates
	// /internal/worker/{uuid}/{:action}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) != 5 {
		return uuid.Nil, fmt.Errorf("Invalid request URL path: %q", r.URL.Path)
	}

	if pathParts[1] != "internal" && pathParts[2] != "worker" && !slices.Contains([]string{":command", ":update"}, pathParts[4]) {
		return uuid.Nil, fmt.Errorf("Request to API path %q is not valid", r.URL.Path)
	}

	queueUUID, err := uuid.Parse(pathParts[3])
	if err != nil {
		return uuid.Nil, fmt.Errorf("Invalid UUID in request URL %q: %w", r.URL.Path, err)
	}

	return queueUUID, nil
}

// swagger:operation POST /internal/worker/{uuid}/:command worker worker_command_post
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
func workerCommandPost(d *Daemon, r *http.Request) response.Response {
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
		Command:      workerCommand.Command,
		Location:     workerCommand.Location,
		SourceType:   workerCommand.SourceType,
		Source:       apiSourceJSON,
		OS:           workerCommand.OS,
		OSVersion:    workerCommand.OSVersion,
		OSType:       workerCommand.OSType,
		Architecture: workerCommand.Architecture,
	}, workerCommand)
}

// swagger:operation POST /internal/worker/{uuid}/:update worker worker_update_post
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
func workerUpdatePost(d *Daemon, r *http.Request) response.Response {
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

	updatedEntry, err := d.queue.ProcessWorkerUpdate(r.Context(), instanceUUID, resp.Status, resp.StatusMessage)
	if err != nil {
		return response.SmartError(err)
	}

	if updatedEntry.MigrationStatus == api.MIGRATIONSTATUS_ERROR {
		var src *migration.Source
		var inst *migration.Instance
		err := transaction.Do(r.Context(), func(ctx context.Context) error {
			var err error
			inst, err = d.instance.GetByUUID(ctx, instanceUUID)
			if err != nil {
				return err
			}

			src, err = d.source.GetByName(ctx, inst.Source)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}

		// Power on the source VM if it was initially running.
		if inst.Properties.Running {
			is, err := source.NewInternalVMwareSourceFrom(src.ToAPI())
			if err != nil {
				return response.SmartError(err)
			}

			err = is.Connect(r.Context())
			if err != nil {
				return response.SmartError(err)
			}

			err = is.PowerOnVM(r.Context(), inst.Properties.Location)
			if err != nil {
				return response.SmartError(err)
			}
		}
	}

	d.queueHandler.RecordWorkerUpdate(instanceUUID)
	return response.SyncResponse(true, nil)
}
