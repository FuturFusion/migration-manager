package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/workerlog"
	"github.com/FuturFusion/migration-manager/shared/api"
)

const workerLogUploadLimit = 2 * api.WorkerLogChunkSize

var workerLogsCmd = APIEndpoint{
	Path: "worker/{uuid}/:logs",
	Post: APIEndpointAction{Handler: workerLogsPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit), Authenticator: workerLogsAuthenticate},
}

var instanceLogsCmd = APIEndpoint{
	Path: "instances/{uuid}/logs",
	Get:  APIEndpointAction{Handler: instanceLogsGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var instanceLogCmd = APIEndpoint{
	Path: "instances/{uuid}/logs/{session}",
	Get:  APIEndpointAction{Handler: instanceLogGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

func workerLogStore(d *Daemon) *workerlog.Store {
	return workerlog.NewStore(filepath.Join(d.os.LogDir, "migration-manager", "workers"))
}

func workerLogsAuthenticate(d *Daemon, w http.ResponseWriter, r *http.Request) (*authenticatorResponse, error) {
	// Bound the body before authentication can parse form data.
	r.Body = http.MaxBytesReader(w, r.Body, workerLogUploadLimit)
	err := r.ParseForm()
	if err != nil {
		return nil, err
	}

	instance, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		return nil, err
	}

	instKey := r.Form.Get("instance")
	if instKey != "" {
		queryUUID, err := uuid.Parse(instKey)
		if err != nil {
			return nil, err
		}

		if instance != queryUUID {
			return nil, fmt.Errorf("Instance query parameter does not match worker path")
		}
	}

	// Bind token lookup to this destination using the existing instance parameter.
	r.Form.Set("instance", instance.String())
	return TokenAuthenticate(d, w, r)
}

func workerLogsPost(d *Daemon, r *http.Request) response.Response {
	instance, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		return response.BadRequest(err)
	}

	// Base64 encoding adds a third to the chunk size; leave room for the JSON envelope.
	content, err := io.ReadAll(io.LimitReader(r.Body, workerLogUploadLimit+1))
	if err != nil {
		return response.BadRequest(err)
	}

	if len(content) > workerLogUploadLimit {
		return response.BadRequest(fmt.Errorf("Log upload is too large"))
	}

	var batch api.WorkerLogBatch
	err = json.Unmarshal(content, &batch)
	if err != nil {
		return response.BadRequest(err)
	}

	session, err := uuid.Parse(batch.Session)
	if err != nil {
		return response.BadRequest(err)
	}

	if batch.Offset < 0 || len(batch.Data) == 0 || len(batch.Data) > api.WorkerLogChunkSize {
		return response.BadRequest(fmt.Errorf("Invalid log offset or chunk size"))
	}

	offset, err := workerLogStore(d).Append(instance, session, batch.Offset, batch.Data)
	if errors.Is(err, workerlog.ErrOffset) {
		return response.BadRequest(err)
	}

	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, offset)
}

// swagger:operation GET /1.0/instances/{uuid}/logs instances instance_logs_get
//
//	List worker log sessions
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: uuid
//	    required: true
//	    type: string
//	responses:
//	  "200":
//	    description: List worker log sessions
//	    schema:
//	      type: object
//	      properties:
//	        metadata:
//	          type: array
//	          items:
//	            $ref: "#/definitions/WorkerLogSession"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instanceLogsGet(d *Daemon, r *http.Request) response.Response {
	instance, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		return response.BadRequest(err)
	}

	sessions, err := workerLogStore(d).Sessions(instance)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, sessions)
}

// swagger:operation GET /1.0/instances/{uuid}/logs/{session} instances instance_log_get
//
//	Read or download a worker log
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: path
//	    name: uuid
//	    required: true
//	    type: string
//	  - in: path
//	    name: session
//	    required: true
//	    type: string
//	  - in: query
//	    name: lines
//	    type: integer
//	    default: 200
//	    minimum: 1
//	    maximum: 10000
//	    description: Maximum lines, also limited to 1 MiB
//	  - in: query
//	    name: oldest_first
//	    type: boolean
//	    default: false
//	    description: Read the earliest lines instead of the latest lines
//	  - in: query
//	    name: download
//	    type: integer
//	    description: Set to 1 to download the complete plain text file
//	responses:
//	  "200":
//	    description: Read or download a worker log
//	    schema:
//	      type: object
//	      properties:
//	        metadata:
//	          $ref: "#/definitions/WorkerLogContent"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "404":
//	    $ref: "#/responses/NotFound"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func instanceLogGet(d *Daemon, r *http.Request) response.Response {
	instance, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		return response.BadRequest(err)
	}

	session, err := uuid.Parse(r.PathValue("session"))
	if err != nil {
		return response.BadRequest(err)
	}

	store := workerLogStore(d)
	if r.URL.Query().Get("download") == "1" {
		_, err = os.Stat(store.Path(instance, session))
		if err != nil {
			return response.SmartError(err)
		}

		return response.FileResponse(r, []response.FileResponseEntry{
			{
				Path:     store.Path(instance, session),
				Filename: instance.String() + "_" + session.String() + ".log",
			},
		}, map[string]string{"Content-Type": "text/plain; charset=utf-8"})
	}

	lines := api.WorkerLogDefaultLines
	value := r.URL.Query().Get("lines")
	if value != "" {
		lines, err = strconv.Atoi(value)
		if err != nil || lines < 1 || lines > api.WorkerLogMaxLines {
			return response.BadRequest(fmt.Errorf("Line count must be between 1 and %d", api.WorkerLogMaxLines))
		}
	}

	oldestFirst := false
	value = r.URL.Query().Get("oldest_first")
	if value != "" {
		oldestFirst, err = strconv.ParseBool(value)
		if err != nil {
			return response.BadRequest(fmt.Errorf("Invalid oldest_first value: %w", err))
		}
	}

	content, err := store.Read(instance, session, lines, oldestFirst)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, content)
}
