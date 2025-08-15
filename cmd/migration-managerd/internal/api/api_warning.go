package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"

	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var warningsCmd = APIEndpoint{
	Path: "warnings",

	Get: APIEndpointAction{Handler: warningsGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var warningCmd = APIEndpoint{
	Path: "warnings/{uuid}",

	Get: APIEndpointAction{Handler: warningGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put: APIEndpointAction{Handler: warningPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

// swagger:operation GET /1.0/warnings warnings warnings_get
//
//	Get the warnings
//
//	Returns a list of warnings (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API warnings
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
//	          description: List of warnings
//	          items:
//	            $ref: "#/definitions/Warning"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func warningsGet(d *Daemon, r *http.Request) response.Response {
	warnings, err := d.warning.GetAll(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	result := make([]api.Warning, 0, len(warnings))
	for _, warning := range warnings {
		result = append(result, warning.ToAPI())
	}

	return response.SyncResponse(true, result)
}

// swagger:operation GET /1.0/warnings/{uuid} warnings warning_get
//
//	Get the warning
//
//	Gets a specific warning.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Warning
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
//	          $ref: "#/definitions/Warning"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func warningGet(d *Daemon, r *http.Request) response.Response {
	wUUIDStr := r.PathValue("uuid")
	wUUID, err := uuid.Parse(wUUIDStr)
	if err != nil {
		return response.SmartError(err)
	}

	warning, err := d.warning.GetByUUID(r.Context(), wUUID)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(
		true,
		warning.ToAPI(),
		warning,
	)
}

// swagger:operation PUT /1.0/warnings/{uuid} warnings warning_put
//
//	Update the warning
//
//	Updates the warning definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: warning
//	    description: Warning definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/Warning"
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
func warningPut(d *Daemon, r *http.Request) response.Response {
	wUUIDStr := r.PathValue("uuid")
	wUUID, err := uuid.Parse(wUUIDStr)
	if err != nil {
		return response.SmartError(err)
	}

	var warning api.WarningPut

	err = json.NewDecoder(r.Body).Decode(&warning)
	if err != nil {
		return response.BadRequest(err)
	}

	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		currentWarning, err := d.warning.GetByUUID(ctx, wUUID)
		if err != nil {
			return fmt.Errorf("Failed to get warning %q: %w", wUUID, err)
		}

		// Validate ETag
		err = util.EtagCheck(r, currentWarning)
		if err != nil {
			return incusAPI.StatusErrorf(http.StatusPreconditionFailed, err.Error())
		}

		_, err = d.warning.UpdateStatusByUUID(ctx, wUUID, warning.Status)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/warnings/"+wUUIDStr)
}
