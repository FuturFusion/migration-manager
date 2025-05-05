package entities

import (
	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// Code generation directives.
//
//generate-database:mapper target queue.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e queue_entry objects table=queue
//generate-database:mapper stmt -e queue_entry objects-by-InstanceUUID table=queue
//generate-database:mapper stmt -e queue_entry objects-by-BatchName table=queue
//generate-database:mapper stmt -e queue_entry objects-by-MigrationStatus table=queue
//generate-database:mapper stmt -e queue_entry objects-by-NeedsDiskImport table=queue
//generate-database:mapper stmt -e queue_entry objects-by-BatchName-and-MigrationStatus table=queue
//generate-database:mapper stmt -e queue_entry objects-by-BatchName-and-NeedsDiskImport table=queue
//generate-database:mapper stmt -e queue_entry objects-by-BatchName-and-MigrationStatus-and-NeedsDiskImport table=queue
//generate-database:mapper stmt -e queue_entry id table=queue
//generate-database:mapper stmt -e queue_entry create table=queue
//generate-database:mapper stmt -e queue_entry update table=queue
//generate-database:mapper stmt -e queue_entry delete-by-InstanceUUID table=queue
//generate-database:mapper stmt -e queue_entry delete-by-BatchName table=queue
//
//generate-database:mapper method -e queue_entry ID table=queue
//generate-database:mapper method -e queue_entry GetOne table=queue
//generate-database:mapper method -e queue_entry GetMany table=queue
//generate-database:mapper method -e queue_entry Create table=queue
//generate-database:mapper method -e queue_entry Update table=queue
//generate-database:mapper method -e queue_entry DeleteOne-by-InstanceUUID table=queue
//generate-database:mapper method -e queue_entry DeleteMany-by-BatchName table=queue

type QueueEntryFilter struct {
	InstanceUUID    *uuid.UUID
	BatchName       *string
	MigrationStatus *api.MigrationStatusType
	NeedsDiskImport *bool
}
