package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"

	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/server/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var networksCmd = APIEndpoint{
	Path: "networks",

	Get:  APIEndpointAction{Handler: networksGet, AllowUntrusted: true},
	Post: APIEndpointAction{Handler: networksPost, AllowUntrusted: true},
}

var networkCmd = APIEndpoint{
	Path: "networks/{name}",

	Delete: APIEndpointAction{Handler: networkDelete, AllowUntrusted: true},
	Get:    APIEndpointAction{Handler: networkGet, AllowUntrusted: true},
	Put:    APIEndpointAction{Handler: networkPut, AllowUntrusted: true},
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
//	          description: List of sources
//	          items:
//	            $ref: "#/definitions/Network"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func networksGet(d *Daemon, r *http.Request) response.Response {
	result := []api.Network{}
	err := d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		networks, err := d.db.GetAllNetworks(tx)
		if err != nil {
			return err
		}

		result = networks
		return nil
	})
	if err != nil {
		return response.SmartError(err)
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
	var n api.Network

	// Decode into the new network.
	err := json.NewDecoder(r.Body).Decode(&n)
	if err != nil {
		return response.BadRequest(err)
	}

	// Insert into database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.AddNetwork(tx, &n)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed creating network %q: %w", n.Name, err))
	}

	return response.SyncResponseLocation(true, nil, "/" + api.APIVersion + "/networks/" + n.Name)
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
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	if name == "" {
		return response.BadRequest(fmt.Errorf("Network name cannot be empty"))
	}

	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.DeleteNetwork(tx, name)
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to delete network '%s': %w", name, err))
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
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	if name == "" {
		return response.BadRequest(fmt.Errorf("Network name cannot be empty"))
	}

	var n api.Network
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbNetwork, err := d.db.GetNetwork(tx, name)
		if err != nil {
			return err
		}

		n = dbNetwork
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get network '%s': %w", name, err))
	}

	return response.SyncResponseETag(true, n, n)
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
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	if name == "" {
		return response.BadRequest(fmt.Errorf("Network name cannot be empty"))
	}

	// Get the existing network.
	var n api.Network
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		dbNetwork, err := d.db.GetNetwork(tx, name)
		if err != nil {
			return err
		}

		n = dbNetwork
		return nil
	})
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get network '%s': %w", name, err))
	}

	// Validate ETag
	err = util.EtagCheck(r, n)
	if err != nil {
		return response.PreconditionFailed(err)
	}

	// Decode into the existing network.
	err = json.NewDecoder(r.Body).Decode(&n)
	if err != nil {
		return response.BadRequest(err)
	}

	// Update network in the database.
	err = d.db.Transaction(r.Context(), func(ctx context.Context, tx *sql.Tx) error {
		return d.db.UpdateNetwork(tx, n)
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed updating network %q: %w", n.Name, err))
	}

	return response.SyncResponseLocation(true, nil, "/" + api.APIVersion + "/networks/" + n.Name)
}
