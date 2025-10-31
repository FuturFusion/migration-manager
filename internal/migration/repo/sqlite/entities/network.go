package entities

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/internal/migration"
)

// Code generation directives.
//
//generate-database:mapper target network.mapper.go
//generate-database:mapper reset
//
//generate-database:mapper stmt -e network objects
//generate-database:mapper stmt -e network objects-by-SourceSpecificID
//generate-database:mapper stmt -e network objects-by-SourceSpecificID-and-Source
//generate-database:mapper stmt -e network objects-by-Source
//generate-database:mapper stmt -e network objects-by-UUID
//generate-database:mapper stmt -e network id
//generate-database:mapper stmt -e network create
//generate-database:mapper stmt -e network update
//generate-database:mapper stmt -e network delete-by-SourceSpecificID-and-Source
//
//generate-database:mapper method -e network ID
//generate-database:mapper method -e network Exists
//generate-database:mapper method -e network GetOne
//generate-database:mapper method -e network GetMany
//generate-database:mapper method -e network Create
//generate-database:mapper method -e network Update
//generate-database:mapper method -e network DeleteOne-by-SourceSpecificID-and-Source

type NetworkFilter struct {
	SourceSpecificID *string
	Source           *string
	UUID             *uuid.UUID
}

// GetNetworkByUUID returns the network with the given UUID.
func GetNetworkByUUID(ctx context.Context, db dbtx, id uuid.UUID) (_ *migration.Network, _err error) {
	defer func() {
		_err = mapErr(_err, "Network")
	}()

	filter := NetworkFilter{UUID: &id}
	objects, err := GetNetworks(ctx, db, filter)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch from \"networks\" table: %w", err)
	}

	switch len(objects) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return &objects[0], nil
	default:
		return nil, fmt.Errorf("More than one \"networks\" entry matches")
	}
}
