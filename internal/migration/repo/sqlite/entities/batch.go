package entities

import (
	"github.com/FuturFusion/migration-manager/shared/api"
)

// Code generation directives.
//
//generate-database:mapper target batch.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e batch objects
//generate-database:mapper stmt -e batch objects-by-ID
//generate-database:mapper stmt -e batch objects-by-Name
//generate-database:mapper stmt -e batch objects-by-Status
//generate-database:mapper stmt -e batch names
//generate-database:mapper stmt -e batch names-by-Status
//generate-database:mapper stmt -e batch id
//generate-database:mapper stmt -e batch create
//generate-database:mapper stmt -e batch update
//generate-database:mapper stmt -e batch rename
//generate-database:mapper stmt -e batch delete-by-Name
//
//generate-database:mapper method -e batch ID
//generate-database:mapper method -e batch Exists
//generate-database:mapper method -e batch GetOne
//generate-database:mapper method -e batch GetMany
//generate-database:mapper method -e batch GetNames
//generate-database:mapper method -e batch Create
//generate-database:mapper method -e batch Update
//generate-database:mapper method -e batch Rename
//generate-database:mapper method -e batch DeleteOne-by-Name

type BatchFilter struct {
	ID     *int64
	Name   *string
	Status *api.BatchStatusType
}
