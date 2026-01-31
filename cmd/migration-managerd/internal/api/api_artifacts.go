package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"sync"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/auth"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/internal/transaction"
	"github.com/FuturFusion/migration-manager/shared/api"
	"github.com/FuturFusion/migration-manager/shared/api/event"
)

var artifactsCmd = APIEndpoint{
	Path: "artifacts",

	Get:  APIEndpointAction{Handler: artifactsGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView), Authenticator: TokenAuthenticate},
	Post: APIEndpointAction{Handler: artifactsPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanCreate), Authenticator: TokenAuthenticate},
}

var artifactCmd = APIEndpoint{
	Path: "artifacts/{uuid}",

	Get:    APIEndpointAction{Handler: artifactGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView), Authenticator: TokenAuthenticate},
	Put:    APIEndpointAction{Handler: artifactPut, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit), Authenticator: TokenAuthenticate},
	Delete: APIEndpointAction{Handler: artifactDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete), Authenticator: TokenAuthenticate},
}

var artifactFilesCmd = APIEndpoint{
	Path: "artifacts/{uuid}/files",

	Post: APIEndpointAction{Handler: artifactFilesPost, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanEdit), Authenticator: TokenAuthenticate},
}

var artifactFileCmd = APIEndpoint{
	Path: "artifacts/{uuid}/files/{name}",

	Get:    APIEndpointAction{Handler: artifactFileGet, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanView), Authenticator: TokenAuthenticate},
	Delete: APIEndpointAction{Handler: artifactFileDelete, AccessHandler: allowPermission(auth.ObjectTypeServer, auth.EntitlementCanDelete), Authenticator: TokenAuthenticate},
}

// artifactLock helps to manage concurrent reads and writes of artifact files.
// Many reads can occur, but a write should wait on existing reads, and once acquired block future reads.
var artifactLock sync.RWMutex

// swagger:operation GET /1.0/artifacts artifacts artifacts_get
//
//	Get all artifacts
//
//	Returns a list of artifacts (structs).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API artifacts
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
//	          description: List of artifacts
//	          items:
//	            $ref: "#/definitions/Artifact"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func artifactsGet(d *Daemon, r *http.Request) response.Response {
	dbArts, err := d.artifact.GetAll(r.Context())
	if err != nil {
		return response.SmartError(err)
	}

	artifacts := make([]api.Artifact, 0, len(dbArts))
	for _, a := range dbArts {
		artifacts = append(artifacts, a.ToAPI())
	}

	return response.SyncResponse(true, artifacts)
}

// swagger:operation POST /1.0/artifacts artifacts artifacts_post
//
//	Add an artifact
//
//	Creates a new artifact record.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: artifact
//	    description: Artifact configuration
//	    required: true
//	    schema:
//	      $ref: "#/definitions/ArtifactPost"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func artifactsPost(d *Daemon, r *http.Request) response.Response {
	var apiArtifact api.ArtifactPost
	err := json.NewDecoder(r.Body).Decode(&apiArtifact)
	if err != nil {
		return response.BadRequest(err)
	}

	art := migration.Artifact{
		UUID:       uuid.New(),
		Type:       apiArtifact.Type,
		Properties: apiArtifact.ArtifactPut,
	}

	_, err = d.artifact.Create(r.Context(), art)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewArtifactEvent(event.ArtifactCreated, r, art.ToAPI(), art.UUID))

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/artifacts/"+art.UUID.String())
}

// swagger:operation GET /1.0/artifacts/{uuid} artifacts artifact_get
//
//	Get an artifact.
//
//	Gets a specific artifact.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Artifact
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
//	          $ref: "#/definitions/Artifact"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func artifactGet(d *Daemon, r *http.Request) response.Response {
	artUUIDStr := r.PathValue("uuid")
	artUUID, err := uuid.Parse(artUUIDStr)
	if err != nil {
		return response.BadRequest(err)
	}

	art, err := d.artifact.GetByUUID(r.Context(), artUUID)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, art.ToAPI())
}

// swagger:operation PUT /1.0/artifacts/{uuid} artifacts artifact_put
//
//	Update an artifact
//
//	Updates the artifact definition.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: artifact
//	    description: Artifact definition
//	    required: true
//	    schema:
//	      $ref: "#/definitions/ArtifactPut"
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
func artifactPut(d *Daemon, r *http.Request) response.Response {
	artUUIDStr := r.PathValue("uuid")
	artUUID, err := uuid.Parse(artUUIDStr)
	if err != nil {
		return response.BadRequest(err)
	}

	var apiArtifact api.ArtifactPut
	err = json.NewDecoder(r.Body).Decode(&apiArtifact)
	if err != nil {
		return response.BadRequest(err)
	}

	art, err := d.artifact.GetByUUID(r.Context(), artUUID)
	if err != nil {
		return response.SmartError(err)
	}

	// lock the artifact for writing.
	artifactLock.Lock()
	defer artifactLock.Unlock()

	art.Properties = apiArtifact
	err = d.artifact.Update(r.Context(), artUUID, art)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewArtifactEvent(event.ArtifactModified, r, art.ToAPI(), art.UUID))

	return response.EmptySyncResponse
}

// swagger:operation DELETE /1.0/artifacts/{uuid} artifacts artifact_delete
//
//	Delete an artifact
//
//	Removes an artifact and all its files (if forced).
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: force
//	    description: Whether to forcibly delete all artifact files.
//	    type: string
//	    example: "1"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func artifactDelete(d *Daemon, r *http.Request) response.Response {
	artUUIDStr := r.PathValue("uuid")
	artUUID, err := uuid.Parse(artUUIDStr)
	if err != nil {
		return response.BadRequest(err)
	}

	// lock the artifact for writing.
	artifactLock.Lock()
	defer artifactLock.Unlock()

	force := r.URL.Query().Get("force") == "1"
	var art *migration.Artifact
	err = transaction.Do(r.Context(), func(ctx context.Context) error {
		var err error
		art, err = d.artifact.GetByUUID(ctx, artUUID)
		if err != nil {
			return err
		}

		if len(art.Files) > 0 && !force {
			return fmt.Errorf("Cannot remove artifact %q with %d files", artUUID.String(), len(art.Files))
		}

		for _, f := range art.Files {
			err = d.artifact.DeleteFile(art.UUID, f)
			if err != nil {
				return err
			}
		}

		return d.artifact.DeleteByUUID(ctx, art.UUID)
	})
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewArtifactEvent(event.ArtifactRemoved, r, art.ToAPI(), art.UUID))

	return response.EmptySyncResponse
}

// swagger:operation POST /1.0/artifacts/{uuid}/files artifacts artifacts_file_post
//
//	Add a file to an artifact
//
//	Upload a file to an artifact record.
//
//	---
//	consumes:
//	  - application/octet-stream
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: raw_file
//	    description: Raw file content
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func artifactFilesPost(d *Daemon, r *http.Request) response.Response {
	artUUIDStr := r.PathValue("uuid")
	artUUID, err := uuid.Parse(artUUIDStr)
	if err != nil {
		return response.BadRequest(err)
	}

	art, err := d.artifact.GetByUUID(r.Context(), artUUID)
	if err != nil {
		return response.SmartError(err)
	}

	defaultFileName, err := art.ToAPI().DefaultArtifactFile()
	if err != nil {
		return response.SmartError(err)
	}

	// lock the artifact for writing.
	artifactLock.Lock()
	defer artifactLock.Unlock()

	err = d.artifact.WriteFile(art.UUID, defaultFileName, r.Body)
	if err != nil {
		return response.SmartError(err)
	}

	art.Files, err = d.artifact.GetFiles(art.UUID)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewArtifactEvent(event.ArtifactModified, r, art.ToAPI(), art.UUID))

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/artifacts/{uuid}/files/{name} artifacts artifacts_file_get
//
//	Get an artifact file
//
//	Download a file from an artifact.
//
//	---
//	produces:
//	  - application/octet-stream
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func artifactFileGet(d *Daemon, r *http.Request) response.Response {
	artUUIDStr := r.PathValue("uuid")
	artUUID, err := uuid.Parse(artUUIDStr)
	if err != nil {
		return response.BadRequest(err)
	}

	fileName := r.PathValue("name")
	if fileName == "" {
		return response.BadRequest(fmt.Errorf("Required 'name' value is missing"))
	}

	art, err := d.artifact.GetByUUID(r.Context(), artUUID)
	if err != nil {
		return response.SmartError(err)
	}

	// lock the artifact for reading.
	artifactLock.RLock()
	defer artifactLock.RUnlock()

	if !slices.Contains(art.Files, fileName) {
		return response.NotFound(fmt.Errorf("File %q not found in artifact %q", fileName, artUUID))
	}

	filePath := filepath.Join(d.artifact.FileDirectory(art.UUID), fileName)
	return response.FileResponse(r, []response.FileResponseEntry{{Path: filePath}}, nil)
}

// swagger:operation DELETE /1.0/artifacts/{uuid}/files/{name} artifacts artifact_file_delete
//
//	Delete an artifact file
//
//	Removes the file from the artifact.
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
func artifactFileDelete(d *Daemon, r *http.Request) response.Response {
	artUUIDStr := r.PathValue("uuid")
	artUUID, err := uuid.Parse(artUUIDStr)
	if err != nil {
		return response.BadRequest(err)
	}

	fileName := r.PathValue("name")
	if fileName == "" {
		return response.BadRequest(fmt.Errorf("Required 'name' value is missing"))
	}

	art, err := d.artifact.GetByUUID(r.Context(), artUUID)
	if err != nil {
		return response.SmartError(err)
	}

	// lock the artifact for writing.
	artifactLock.Lock()
	defer artifactLock.Unlock()

	err = d.artifact.DeleteFile(art.UUID, fileName)
	if err != nil {
		return response.SmartError(err)
	}

	d.logHandler.SendLifecycle(r.Context(), event.NewArtifactEvent(event.ArtifactRemoved, r, art.ToAPI(), art.UUID))

	return response.EmptySyncResponse
}
