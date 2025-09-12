package entities

import (
	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// Code generation directives.
//
//generate-database:mapper target artifact.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e artifact objects
//generate-database:mapper stmt -e artifact objects-by-ID
//generate-database:mapper stmt -e artifact objects-by-UUID
//generate-database:mapper stmt -e artifact objects-by-Type
//generate-database:mapper stmt -e artifact id
//generate-database:mapper stmt -e artifact create
//generate-database:mapper stmt -e artifact update
//generate-database:mapper stmt -e artifact delete-by-UUID
//
//generate-database:mapper method -e artifact ID
//generate-database:mapper method -e artifact Exists
//generate-database:mapper method -e artifact GetOne
//generate-database:mapper method -e artifact GetMany
//generate-database:mapper method -e artifact Create
//generate-database:mapper method -e artifact Update
//generate-database:mapper method -e artifact DeleteOne-by-UUID

type ArtifactFilter struct {
	ID   *int64
	UUID *uuid.UUID
	Type *api.ArtifactType
}
