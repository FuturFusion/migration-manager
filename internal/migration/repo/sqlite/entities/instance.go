package entities

import (
	"github.com/google/uuid"
)

// Code generation directives.
//
//generate-database:mapper target instance.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e instance objects
//generate-database:mapper stmt -e instance objects-by-ID
//generate-database:mapper stmt -e instance objects-by-UUID
//generate-database:mapper stmt -e instance objects-by-Source
//generate-database:mapper stmt -e instance names
//generate-database:mapper stmt -e instance names-by-UUID
//generate-database:mapper stmt -e instance names-by-Source
//generate-database:mapper stmt -e instance id
//generate-database:mapper stmt -e instance create
//generate-database:mapper stmt -e instance update
//generate-database:mapper stmt -e instance delete-by-UUID
//
//generate-database:mapper method -e instance ID
//generate-database:mapper method -e instance GetOne
//generate-database:mapper method -e instance GetMany
//generate-database:mapper method -e instance GetNames
//generate-database:mapper method -e instance Create
//generate-database:mapper method -e instance Update
//generate-database:mapper method -e instance DeleteOne-by-UUID

type InstanceFilter struct {
	ID     *int64
	UUID   *uuid.UUID
	Source *string
}
