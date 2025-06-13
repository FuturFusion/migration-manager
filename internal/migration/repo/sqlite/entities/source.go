package entities

import "github.com/FuturFusion/migration-manager/shared/api"

// Code generation directives.
//
//generate-database:mapper target source.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e source objects
//generate-database:mapper stmt -e source objects-by-SourceType
//generate-database:mapper stmt -e source objects-by-Name
//generate-database:mapper stmt -e source objects-by-Name-and-SourceType
//generate-database:mapper stmt -e source names
//generate-database:mapper stmt -e source id
//generate-database:mapper stmt -e source create
//generate-database:mapper stmt -e source update
//generate-database:mapper stmt -e source rename
//generate-database:mapper stmt -e source delete-by-Name
//
//generate-database:mapper method -e source ID
//generate-database:mapper method -e source Exists
//generate-database:mapper method -e source GetOne
//generate-database:mapper method -e source GetMany
//generate-database:mapper method -e source GetNames
//generate-database:mapper method -e source Create
//generate-database:mapper method -e source Update
//generate-database:mapper method -e source Rename
//generate-database:mapper method -e source DeleteOne-by-Name

type SourceFilter struct {
	SourceType *api.SourceType
	Name       *string
}
