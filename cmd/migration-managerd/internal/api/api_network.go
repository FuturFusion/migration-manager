package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var networksCmd = APIEndpoint{
	Path: "networks",

	Get:  APIEndpointAction{Handler: networksGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Post: APIEndpointAction{Handler: networksPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanCreate)},
}

var networkCmd = APIEndpoint{
	Path: "networks/{name}",

	Delete: APIEndpointAction{Handler: networkDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
	Get:    APIEndpointAction{Handler: networkGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
	Put:    APIEndpointAction{Handler: networkPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

// swagger:operation GET /1.0/networks networks networks_get
//
//	Get the networks
//
//	Returns a list of networks (URLs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API networks
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
//	          description: List of networks
//                items:
//                  type: string
//                example: |-
//                  [
//                    "/1.0/networks/foo",
//                    "/1.0/networks/bar"
//                  ]
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/networks?recursion=1 networks networks_get_recursion
//
//	Get the networks
//
//	Returns a list of networks (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API networks
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
//	          description: List of networks
//	          items:
//	            $ref: "#/definitions/Network"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func networksGet(d *Daemon, r *http.Request) response.Response {
	// Parse the recursion field.
	recursion, err := strconv.Atoi(r.FormValue("recursion"))
	if err != nil {
		recursion = 0
	}

	if recursion == 1 {
		networks, err := d.network.GetAll(r.Context())
		if err != nil {
			return response.SmartError(err)
		}

		result := make([]api.Network, 0, len(networks))
		for _, network := range networks {
			result = append(result, api.Network{
				DatabaseID: network.ID,
				Name:       network.Name,
				Config:     network.Config,
			})
		}

		return response.SyncResponse(true, result)
	}

	networkNames, err := d.network.GetAllNames(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	result := make([]string, 0, len(networkNames))
	for _, name := range networkNames {
		result = append(result, fmt.Sprintf("/%s/networks/%s", api.APIVersion, name))
	}

	return response.SyncResponse(true, result)
}

// swagger:operation POST /1.0/networks networks networks_post
//
//	Add a network
//
//	Creates a new network.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: network
//	    description: Network configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/Network"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func networksPost(d *Daemon, r *http.Request) response.Response {
	var network api.Network

	// Decode into the new network.
	err := json.NewDecoder(r.Body).Decode(&network)
	if err != nil {
		return response.BadRequest(err)
	}

	_, err = d.network.Create(r.Context(), migration.Network{
		ID:     network.DatabaseID,
		Name:   network.Name,
		Config: network.Config,
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating network %q: %w", network.Name, err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/networks/"+network.Name)
}

// swagger:operation DELETE /1.0/networks/{name} networks network_delete
//
//	Delete the network
//
//	Removes the network.
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
func networkDelete(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	err := d.network.DeleteByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/networks/{name} networks network_get
//
//	Get the network
//
//	Gets a specific network.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Network
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
//	          $ref: "#/definitions/Network"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func networkGet(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	network, err := d.network.GetByName(r.Context(), name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(
		true,
		api.Network{
			DatabaseID: network.ID,
			Name:       network.Name,
			Config:     network.Config,
		},
		network,
	)
}

// swagger:operation PUT /1.0/networks/{name} networks network_put
//
//	Update the network
//
//	Updates the network definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: network
//	    description: Network definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/Network"
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
func networkPut(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")

	var network api.Network

	err := json.NewDecoder(r.Body).Decode(&network)
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

	currentNetwork, err := d.network.GetByName(ctx, name)
	if err != nil {
		return response.SmartError(err)
	}

	// Validate ETag
	err = util.EtagCheck(r, currentNetwork)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	_, err = d.network.UpdateByID(ctx, migration.Network{
		ID:     currentNetwork.ID,
		Name:   network.Name,
		Config: network.Config,
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating network %q: %w", network.Name, err))
	}

	err = trans.Commit()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed commit transaction: %w", err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/networks/"+network.Name)
}
