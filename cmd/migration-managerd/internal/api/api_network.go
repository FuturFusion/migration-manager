package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var networksCmd = APIEndpoint{
	Path: "networks",

	Get: APIEndpointAction{Handler: networksGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var networkCmd = APIEndpoint{
	Path: "networks/{name}",

	Delete: APIEndpointAction{Handler: networkDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete)},
	Get:    APIEndpointAction{Handler: networkGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var networkInstancesCmd = APIEndpoint{
	Path: "networks/{name}/instances",

	Get: APIEndpointAction{Handler: networkInstancesGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView)},
}

var networkOverrideCmd = APIEndpoint{
	Path: "networks/{name}/override",

	Put: APIEndpointAction{Handler: networkOverridePut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit)},
}

// swagger:operation GET /1.0/networks networks networks_get
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
	networks, err := d.network.GetAll(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	result := make([]api.Network, 0, len(networks))
	for _, network := range networks {
		apiNet, err := network.ToAPI()
		if err != nil {
			return response.SmartError(err)
		}

		result = append(result, *apiNet)
	}

	return response.SyncResponse(true, result)
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
//	parameters:
//	  - in: query
//	    name: source
//	    description: Source where the network is defined
//	    required: true
//	    type: string
//	    example: name matches 'vcenter01'
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
	srcName := r.FormValue("source")
	if srcName == "" {
		return response.BadRequest(fmt.Errorf("Missing 'source' query paramterer"))
	}

	err := d.network.DeleteByNameAndSource(r.Context(), name, srcName)
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
//	parameters:
//	  - in: query
//	    name: source
//	    description: Source where the network is defined
//	    required: true
//	    type: string
//	    example: name matches 'vcenter01'
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

	srcName := r.FormValue("source")
	if srcName == "" {
		return response.BadRequest(fmt.Errorf("Missing 'source' query paramterer"))
	}

	network, err := d.network.GetByNameAndSource(r.Context(), name, srcName)
	if err != nil {
		return response.SmartError(err)
	}

	apiNet, err := network.ToAPI()
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(
		true,
		apiNet,
		network,
	)
}

// swagger:operation PUT /1.0/networks/{name}/override networks network_override_put
//
//	Update the network overrides
//
//	Updates the network override definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: source
//	    description: Source where the network is defined
//	    required: true
//	    type: string
//	    example: name matches 'vcenter01'
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
func networkOverridePut(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")
	srcName := r.FormValue("source")
	if srcName == "" {
		return response.BadRequest(fmt.Errorf("Missing 'source' query paramterer"))
	}

	var overrides api.NetworkPlacement

	err := json.NewDecoder(r.Body).Decode(&overrides)
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

	currentNetwork, err := d.network.GetByNameAndSource(ctx, name, srcName)
	if err != nil {
		return response.SmartError(err)
	}

	// Validate ETag
	err = util.EtagCheck(r, currentNetwork)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	err = d.network.Update(ctx, &migration.Network{
		ID:         currentNetwork.ID,
		SourceSpecificID: currentNetwork.SourceSpecificID,
		Location:   currentNetwork.Location,
		Type:       currentNetwork.Type,
		Properties: currentNetwork.Properties,
		Source:     currentNetwork.Source,
		Overrides:  overrides,
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating network %q: %w", currentNetwork.SourceSpecificID, err))
	}

	err = trans.Commit()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed commit transaction: %w", err))
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/networks/"+currentNetwork.SourceSpecificID)
}

// swagger:operation GET /1.0/networks/{name}/instances?source={source} networks networks_instances_get
//
//	Get instances for the network
//
//	Returns a list of instances assigned to this network (structs).
//
//	---
//	parameters:
//	  - in: query
//	    name: source
//	    description: Source where the network is defined
//	    required: true
//	    type: string
//	    example: name matches 'vcenter01'
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API instances
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
//	          description: List of instances
//	          items:
//	            $ref: "#/definitions/Instance"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func networkInstancesGet(d *Daemon, r *http.Request) response.Response {
	name := r.PathValue("name")
	srcName := r.FormValue("source")
	if srcName == "" {
		return response.BadRequest(fmt.Errorf("Missing 'source' query paramterer"))
	}

	result := []api.Instance{}
	err := transaction.Do(r.Context(), func(ctx context.Context) error {
		network, err := d.network.GetByNameAndSource(ctx, name, srcName)
		if err != nil {
			return err
		}

		instances, err := d.instance.GetAllBySource(ctx, srcName)
		if err != nil {
			return err
		}

		for _, inst := range instances {
			for _, nic := range inst.Properties.NICs {
				if nic.ID == network.SourceSpecificID {
					result = append(result, inst.ToAPI())
				}
			}
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, result)
}
