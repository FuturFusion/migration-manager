package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var api10Cmd = APIEndpoint{
	Get:  APIEndpointAction{Handler: api10Get, AllowUntrusted: true},
	Post: APIEndpointAction{Handler: api10Post, AllowUntrusted: true},
}

var api10 = []APIEndpoint{
	api10Cmd,
	batchCmd,
	batchInstancesCmd,
	batchStartCmd,
	batchStopCmd,
	batchesCmd,
	instanceCmd,
	instancesCmd,
	networkCmd,
	networksCmd,
	queueRootCmd,
	queueCmd,
	sourceCmd,
	sourcesCmd,
	targetCmd,
	targetsCmd,
}

// swagger:operation GET /1.0?public server server_get_untrusted
//
//	Get the server environment
//
//	Shows a small subset of the server environment and configuration
//	which is required by untrusted clients to reach a server.
//
//	The `?public` part of the URL isn't required, it's simply used to
//	separate the two behaviors of this endpoint.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Server environment and configuration
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
//	          $ref: "#/definitions/ServerUntrusted"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func api10Get(d *Daemon, r *http.Request) response.Response {
	srv := api.ServerUntrusted{
		APIStatus:   api.APIStatus,
		APIVersion:  api.APIVersion,
		Auth:        "untrusted",
		AuthMethods: []string{},
	}

	// Get the global config, if any.
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		var err error
		srv.Config, err = d.db.ReadGlobalConfig(tx)
		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(true, srv, nil)
}

// swagger:operation POST /1.0 server server_post
//
//	Update server config
//
//	Replaces an existing config with the provided one.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: config
//	    description: Map of config key value pairs
//	    required: true
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func api10Post(d *Daemon, r *http.Request) response.Response {
	config := make(map[string]string)

	// Decode the config.
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		return response.BadRequest(err)
	}

	// Insert into database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.WriteGlobalConfig(tx, config)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating/updating config: %w", err))
	}

	// Update the in-memory map.
	d.globalConfig = config

	return response.SyncResponse(true, nil)
}
