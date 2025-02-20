package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	incusTLS "github.com/lxc/incus/v6/shared/tls"

	"github.com/FuturFusion/migration-manager/internal/client/oidc"
	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var targetsCmd = APIEndpoint{
	Path: "targets",

	Get:  APIEndpointAction{Handler: targetsGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Post: APIEndpointAction{Handler: targetsPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanCreate)},
}

var targetCmd = APIEndpoint{
	Path: "targets/{name}",

	Delete: APIEndpointAction{Handler: targetDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
	Get:    APIEndpointAction{Handler: targetGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put:    APIEndpointAction{Handler: targetPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

// swagger:operation GET /1.0/targets targets targets_get
//
//	Get the targets
//
//	Returns a list of targets (URLs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API targets
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
//	          description: List of targets
//                items:
//                  type: string
//                example: |-
//                  [
//                    "/1.0/targets/foo",
//                    "/1.0/targets/bar"
//                  ]
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/targets?recursion=1 targets targets_get_recursion
//
//	Get the targets
//
//	Returns a list of targets (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API targets
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
//	          description: List of targets
//	          items:
//	            $ref: "#/definitions/IncusTarget"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func targetsGet(d *Daemon, r *http.Request) response.Response {
	// Parse the recursion field.
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	if recursion == 1 {
		targets, err := d.target.GetAll(r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		result := make([]api.Target, 0, len(targets))
		for _, target := range targets {
			result = append(result, api.Target{
				DatabaseID: target.ID,
				Name:       target.Name,
				TargetType: target.TargetType,
				Properties: target.Properties,
			})
		}

		return response.SyncResponse(true, result)
	}

	targetNames, err := d.target.GetAllNames(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	result := make([]string, 0, len(targetNames))
	for _, name := range targetNames {
		result = append(result, fmt.Sprintf("/%s/targets/%s", api.APIVersion, name))
	}

	return response.SyncResponse(true, result)
}

// swagger:operation POST /1.0/targets targets targets_post
//
//	Add a target
//
//	Creates a new target.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: target
//	    description: Target configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/IncusTarget"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func targetsPost(d *Daemon, r *http.Request) response.Response {
	var target api.Target

	// Decode into the new target.
	err := json.NewDecoder(r.Body).Decode(&target)
	if err != nil {
		return response.BadRequest(err)
	}

	_, err = d.target.Create(r.Context(), migration.Target{
		Name:       target.Name,
		TargetType: target.TargetType,
		Properties: target.Properties,
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating target %q: %w", target.Name, err))
	}

	d.checkTargetConnectivity()

	// Get the target's connectivity status to return to the client.
	currentTarget, err := d.target.GetByName(r.Context(), target.Name)
	if err != nil {
		return response.SmartError(err)
	}

	metadata := make(map[string]string)
	metadata["ConnectivityStatus"] = fmt.Sprintf("%d", currentTarget.GetExternalConnectivityStatus())

	// If waiting on fingerprint confirmation, return it to the user.
	if currentTarget.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT {
		metadata["certFingerprint"] = incusTLS.CertFingerprint(currentTarget.GetServerCertificate())
	}

	// If the target is using OIDC, get the authentication URL and return it to the user.
	if currentTarget.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_WAITING_OIDC {
		u, err := getOIDCAuthURL(d, currentTarget.Name, currentTarget.GetEndpoint())
		if err != nil {
			return response.SmartError(err)
		}

		metadata["OIDCURL"] = u
	}

	return response.SyncResponseLocation(true, metadata, "/"+api.APIVersion+"/targets/"+target.Name)
}

func getOIDCAuthURL(d *Daemon, targetName string, endpointURL string) (string, error) {
	apiEndpoint, _ := url.JoinPath(endpointURL, "/1.0")
	req, err := http.NewRequest(http.MethodGet, apiEndpoint, nil)
	if err != nil {
		return "", err
	}

	oidcClient := oidc.NewOIDCClient("", nil) // TODO -- handle TLS errors if insecure
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oidcClient.GetAccessToken()))
	tokenURL, resp, provider, err := oidcClient.FetchNewIncusTokenURL(req)
	if err != nil {
		return "", err
	}

	// Spawn a worker, since we need to wait for the user to complete the authentication workflow.
	go func() {
		err := oidcClient.WaitForToken(resp, provider)
		connectivityStatus := mapErrorToStatus(err)

		tgt, err := d.target.GetByName(context.TODO(), targetName)
		if err != nil {
			return
		}

		tgt.SetExternalConnectivityStatus(connectivityStatus)
		tgt.SetOIDCTokens(oidcClient.GetOIDCTokens())

		_, _ = d.target.UpdateByID(context.TODO(), tgt)
	}()

	return tokenURL, nil
}

// swagger:operation DELETE /1.0/targets/{name} targets target_delete
//
//	Delete the target
//
//	Removes the target.
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
func targetDelete(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	err := d.target.DeleteByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/targets/{name} targets target_get
//
//	Get the target
//
//	Gets a specific target.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Target
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
//	          $ref: "#/definitions/IncusTarget"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func targetGet(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	target, err := d.target.GetByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(
		true,
		api.Target{
			DatabaseID: target.ID,
			Name:       target.Name,
			TargetType: target.TargetType,
			Properties: target.Properties,
		},
		target,
	)
}

// swagger:operation PUT /1.0/targets/{name} targets target_put
//
//	Update the target
//
//	Updates the target definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: target
//	    description: Target definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/IncusTarget"
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
func targetPut(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	var target api.Target

	err := json.NewDecoder(r.Body).Decode(&target)
	if err != nil {
		return response.BadRequest(err)
	}

	ctx, trans := transaction.Begin(r.Context())
	defer func() {
		rollbackErr := trans.Rollback()
		if rollbackErr != nil {
			response.SmartError(fmt.Errorf("Transaction rollback failed: %v, reason: %w", rollbackErr, err))
		}
	}()

	currentTarget, err := d.target.GetByName(ctx, name)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get target %q: %w", name, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, currentTarget)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	_, err = d.target.UpdateByID(ctx, migration.Target{
		ID:         currentTarget.ID,
		Name:       target.Name,
		TargetType: target.TargetType,
		Properties: target.Properties,
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating target %q: %w", target.Name, err))
	}

	err = trans.Commit()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed commit transaction: %w", err))
	}

	d.checkTargetConnectivity()

	// Get the target's connectivity status to return to the client.
	currentTarget, err = d.target.GetByName(r.Context(), target.Name)
	if err != nil {
		return response.SmartError(err)
	}

	metadata := make(map[string]string)
	metadata["ConnectivityStatus"] = fmt.Sprintf("%d", currentTarget.GetExternalConnectivityStatus())

	// If waiting on fingerprint confirmation, return it to the user.
	if currentTarget.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_TLS_CONFIRM_FINGERPRINT {
		metadata["certFingerprint"] = incusTLS.CertFingerprint(currentTarget.GetServerCertificate())
	}

	// If the target is using OIDC, get the authentication URL and return it to the user.
	if currentTarget.GetExternalConnectivityStatus() == api.EXTERNALCONNECTIVITYSTATUS_WAITING_OIDC {
		u, err := getOIDCAuthURL(d, currentTarget.Name, currentTarget.GetEndpoint())
		if err != nil {
			return response.SmartError(err)
		}

		metadata["OIDCURL"] = u
	}

	return response.SyncResponseLocation(true, metadata, "/"+api.APIVersion+"/targets/"+target.Name)
}
