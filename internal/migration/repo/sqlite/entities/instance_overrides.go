package entities

import (
	"github.com/google/uuid"
)

// Code generation directives.
//
//generate-database:mapper target instance_override.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e instance_override objects table=instance_overrides
//generate-database:mapper stmt -e instance_override objects-by-UUID table=instance_overrides
//generate-database:mapper stmt -e instance_override id table=instance_overrides
//generate-database:mapper stmt -e instance_override create table=instance_overrides
//generate-database:mapper stmt -e instance_override update table=instance_overrides
//generate-database:mapper stmt -e instance_override delete-by-UUID table=instance_overrides
//
//generate-database:mapper method -e instance_override ID table=instance_overrides
//generate-database:mapper method -e instance_override GetOne table=instance_overrides
//generate-database:mapper method -e instance_override GetMany table=instance_overrides
//generate-database:mapper method -e instance_override Create table=instance_overrides
//generate-database:mapper method -e instance_override Update table=instance_overrides
//generate-database:mapper method -e instance_override DeleteOne-by-UUID table=instance_overrides

type InstanceOverrideFilter struct {
	UUID *uuid.UUID
}
