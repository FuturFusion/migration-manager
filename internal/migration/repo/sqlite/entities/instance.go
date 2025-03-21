package entities

import (
	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// Code generation directives.
//
//generate-database:mapper target instance.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e instance objects
//generate-database:mapper stmt -e instance objects-by-UUID
//generate-database:mapper stmt -e instance objects-by-Batch
//generate-database:mapper stmt -e instance objects-by-MigrationStatus
//generate-database:mapper stmt -e instance objects-by-Batch-and-MigrationStatus
//generate-database:mapper stmt -e instance names
//generate-database:mapper stmt -e instance names-by-UUID
//generate-database:mapper stmt -e instance names-by-Batch
//generate-database:mapper stmt -e instance names-by-MigrationStatus
//generate-database:mapper stmt -e instance names-by-Batch-and-MigrationStatus
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
	UUID            *uuid.UUID
	Batch           *string
	MigrationStatus *api.MigrationStatusType
}
