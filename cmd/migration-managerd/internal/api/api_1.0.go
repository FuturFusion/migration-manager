package api

import (
	"net/http"

	"github.com/FuturFusion/migration-manager/internal/server/request"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var api10Cmd = APIEndpoint{
	Get: APIEndpointAction{Handler: api10Get, AllowUntrusted: true},
}

var api10 = []APIEndpoint{
	api10Cmd,
	artifactCmd,
	artifactFilesCmd,
	artifactsCmd,
	artifactFileCmd,
	batchCmd,
	batchInstancesCmd,
	batchResetCmd,
	batchStartCmd,
	batchStopCmd,
	batchesCmd,
	instanceCmd,
	instanceOverrideCmd,
	instancesCmd,
	networkCmd,
	networkInstancesCmd,
	networksCmd,
	queueRootCmd,
	queueCmd,
	queueWorkerCmd,
	queueWorkerCommandCmd,
	sourceCmd,
	sourcesCmd,
	systemCertificateCmd,
	systemNetworkCmd,
	systemSecurityCmd,
	targetCmd,
	targetsCmd,
	warningCmd,
	warningsCmd,
}

// swagger:operation GET /1.0 server server_get_untrusted
//
//	Get the server environment
//
//	Shows a small subset of the server environment and configuration
//	which is required by untrusted clients to reach a server.
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
		AuthMethods: []string{"oidc", "tls"},
	}

	// Return the authentication method, if any, that the client is using.
	ctx := r.Context()
	auth := ctx.Value(request.CtxProtocol)
	if auth != nil {
		v, ok := auth.(string)
		if ok {
			srv.Auth = v
		}
	}

	return response.SyncResponseETag(true, srv, nil)
}
