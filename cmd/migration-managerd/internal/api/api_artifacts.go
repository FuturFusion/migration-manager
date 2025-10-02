package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"sync"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/server/response"
	"github.com/FuturFusion/migration-manager/shared/api"
)

var artifactsCmd = APIEndpoint{
	Path: "artifacts",

	Get:  APIEndpointAction{Handler: artifactsGet, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
	Post: APIEndpointAction{Handler: artifactsPost, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
}

var artifactCmd = APIEndpoint{
	Path: "artifacts/{uuid}",

	Get: APIEndpointAction{Handler: artifactGet, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
	Put: APIEndpointAction{Handler: artifactPut, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
}

var artifactFilesCmd = APIEndpoint{
	Path: "artifacts/{uuid}/files",

	Post: APIEndpointAction{Handler: artifactFilesPost, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
}

var artifactFileCmd = APIEndpoint{
	Path: "artifacts/{uuid}/files/{name}",

	Get:    APIEndpointAction{Handler: artifactFileGet, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
	Delete: APIEndpointAction{Handler: artifactFileDelete, AccessHandler: allowWithToken, Authenticator: TokenAuthenticate},
}

// artifactLock helps to manage concurrent reads and writes of artifact files.
// Many reads can occur, but a write should wait on existing reads, and once acquired block future reads.
var artifactLock sync.RWMutex

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

func artifactsPost(d *Daemon, r *http.Request) response.Response {
	var apiArtifact api.ArtifactPost
	err := json.NewDecoder(r.Body).Decode(&apiArtifact)
	if err != nil {
		return response.BadRequest(err)
	}

	art := migration.Artifact{
		UUID:       uuid.New(),
		Type:       apiArtifact.Type,
		Properties: apiArtifact.Properties,
	}

	_, err = d.artifact.Create(r.Context(), art)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseLocation(true, nil, "/"+api.APIVersion+"/artifacts/"+art.UUID.String())
}

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

	return response.EmptySyncResponse
}

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

	// If no explicit file name is given, use the default file name that we expect.
	fileName := r.FormValue("name")
	if fileName == "" {
		fileName = defaultFileName
	}

	if fileName != defaultFileName {
		return response.SmartError(fmt.Errorf("File %q not supported", fileName))
	}

	// lock the artifact for writing.
	artifactLock.Lock()
	defer artifactLock.Unlock()

	err = d.artifact.WriteFile(art.UUID, fileName, r.Body)
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

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

	return response.EmptySyncResponse
}
