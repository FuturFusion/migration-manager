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
func NewBatch(name string, includeRegex string, excludeRegex string, migrationWindowStart time.Time, migrationWindowEnd time.Time, defaultNetwork string) *InternalBatch {
	var status api.BatchStatusType = api.BATCHSTATUS_DEFINED
	return &InternalBatch{
		Batch: api.Batch{
			Name: name,
			DatabaseID: internal.INVALID_DATABASE_ID,
			Status: status,
			StatusString: status.String(),
			IncludeRegex: includeRegex,
			ExcludeRegex: excludeRegex,
			MigrationWindowStart: migrationWindowStart,
			MigrationWindowEnd: migrationWindowEnd,
			DefaultNetwork: defaultNetwork,
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

func (b *InternalBatch) GetStatus() api.BatchStatusType {
	return b.Status
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
		if excludeRegex.Match([]byte(i.GetName())) {
			return false
		}
	}

	// Handle any inclusionary criteria second.
	if b.IncludeRegex != "" {
		includeRegex := regexp.MustCompile(b.IncludeRegex)
		if !includeRegex.Match([]byte(i.GetName())) {
			return false
		}
	}

	return true
}
