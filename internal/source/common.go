package source

import (
	"context"
	"fmt"

	"github.com/FuturFusion/migration-manager/internal"
)

// CommonSource defines properties common to all sources.
//
// swagger:model
type CommonSource struct {
	// A human-friendly name for this source
	// Example: MySource
	Name string `json:"name" yaml:"name"`

	// An opaque integer identifier for the source
	// Example: 123
	DatabaseID int `json:"databaseID" yaml:"databaseID"`

	isConnected bool
}

// Returns a new CommonSource ready for use.
func NewCommonSource(name string) *CommonSource {
	return &CommonSource{
		Name: name,
		DatabaseID: internal.INVALID_DATABASE_ID,
		isConnected: false,
	}
}

func (s *CommonSource) Connect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *CommonSource) Disconnect(ctx context.Context) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *CommonSource) IsConnected() bool {
	return s.isConnected
}

func (s *CommonSource) DeleteVMSnapshot(ctx context.Context, vmName string, snapshotName string) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *CommonSource) ImportDisks(ctx context.Context, vmName string) error {
	return fmt.Errorf("Not implemented by CommonSource")
}

func (s *CommonSource) GetName() string {
	return s.Name
}

func (s *CommonSource) GetDatabaseID() (int, error) {
	if s.DatabaseID == internal.INVALID_DATABASE_ID {
		return internal.INVALID_DATABASE_ID, fmt.Errorf("Source has not been added to database, so it doesn't have an ID")
	}

	return s.DatabaseID, nil
}
