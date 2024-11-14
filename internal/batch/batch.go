package batch

import (
	"fmt"
	"time"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalBatch struct {
	api.Batch `yaml:",inline"`
}

// Returns a new Batch ready for use.
func NewBatch(name string, includeRegex string, excludeRegex string, migrationWindowStart time.Time, migrationWindowEnd time.Time) *InternalBatch {
	return &InternalBatch{
		Batch: api.Batch{
			Name: name,
			DatabaseID: internal.INVALID_DATABASE_ID,
			Status: api.BATCHSTATUS_DEFINED,
			IncludeRegex: includeRegex,
			ExcludeRegex: excludeRegex,
			MigrationWindowStart: migrationWindowStart,
			MigrationWindowEnd: migrationWindowEnd,
		},
	}
}

func (b *InternalBatch) GetName() string {
	return b.Name
}

func (b *InternalBatch) GetDatabaseID() (int, error) {
	if b.DatabaseID == internal.INVALID_DATABASE_ID {
		return internal.INVALID_DATABASE_ID, fmt.Errorf("Batch has not been added to database, so it doesn't have an ID")
	}

	return b.DatabaseID, nil
}

func (b *InternalBatch) CanBeModified() bool {
	return b.Status == api.BATCHSTATUS_DEFINED || b.Status == api.BATCHSTATUS_FINISHED || b.Status == api.BATCHSTATUS_ERROR
}
