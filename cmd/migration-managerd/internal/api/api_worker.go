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
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var workerUpdateCmd = APIEndpoint{
	Path: "worker/{uuid}/:update",

	Post: APIEndpointAction{Handler: workerUpdatePost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit), Authenticator: TokenAuthenticate},
}

var workerCommandCmd = APIEndpoint{
	Path: "worker/{uuid}/:command",

	Post: APIEndpointAction{Handler: workerCommandPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit), Authenticator: TokenAuthenticate},
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

func workerCommandPost(d *Daemon, r *http.Request) response.Response {
	err := d.WaitForSchemaUpdate(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	// Share this lock with running worker tasks.
	workerLock.RLock()
	defer workerLock.RUnlock()
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

func workerUpdatePost(d *Daemon, r *http.Request) response.Response {
	err := d.WaitForSchemaUpdate(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	// Share this lock with running worker tasks.
	workerLock.RLock()
	defer workerLock.RUnlock()
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
