package batch

import (
	"fmt"
	"regexp"
	"time"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalBatch struct {
	api.Batch `yaml:",inline"`
}

// Returns a new Batch ready for use.
func NewBatch(name string, targetID int, storagePool string, includeRegex string, excludeRegex string, migrationWindowStart time.Time, migrationWindowEnd time.Time, defaultNetwork string) *InternalBatch {
	return &InternalBatch{
		Batch: api.Batch{
			Name:                 name,
			DatabaseID:           internal.INVALID_DATABASE_ID,
			TargetID:             targetID,
			Status:               api.BATCHSTATUS_DEFINED,
			StatusString:         api.BATCHSTATUS_DEFINED.String(),
			StoragePool:          storagePool,
			IncludeRegex:         includeRegex,
			ExcludeRegex:         excludeRegex,
			MigrationWindowStart: migrationWindowStart,
			MigrationWindowEnd:   migrationWindowEnd,
			DefaultNetwork:       defaultNetwork,
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

func (b *InternalBatch) GetTargetID() int {
	return b.TargetID
}

func (b *InternalBatch) CanBeModified() bool {
	switch b.Status {
	case api.BATCHSTATUS_DEFINED,
		api.BATCHSTATUS_FINISHED,
		api.BATCHSTATUS_ERROR:
		return true
	default:
		return false
	}
}

func (b *InternalBatch) GetStatus() api.BatchStatusType {
	return b.Status
}

func (b *InternalBatch) GetStoragePool() string {
	return b.StoragePool
}

func (b *InternalBatch) GetMigrationWindowStart() time.Time {
	return b.MigrationWindowStart
}

func (b *InternalBatch) GetMigrationWindowEnd() time.Time {
	return b.MigrationWindowEnd
}

func (b *InternalBatch) GetDefaultNetwork() string {
	return b.DefaultNetwork
}

func (b *InternalBatch) InstanceMatchesCriteria(i instance.Instance) bool {
	// Handle any exclusionary criteria first.
	if b.ExcludeRegex != "" {
		excludeRegex := regexp.MustCompile(b.ExcludeRegex)
		if excludeRegex.Match([]byte(i.GetInventoryPath())) {
			return false
		}
	}

	// Handle any inclusionary criteria second.
	if b.IncludeRegex != "" {
		includeRegex := regexp.MustCompile(b.IncludeRegex)
		if !includeRegex.Match([]byte(i.GetInventoryPath())) {
			return false
		}
	}

	return true
}
