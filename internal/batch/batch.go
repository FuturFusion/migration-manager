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

// REVIEW: in order to ensure, that InternalBatch actually implements Batch
// I prefer to add this controll instruction:
var _ Batch = &InternalBatch{}

// Returns a new Batch ready for use.
func NewBatch(name string, includeRegex string, excludeRegex string, migrationWindowStart time.Time, migrationWindowEnd time.Time, defaultNetwork string) *InternalBatch {
	return &InternalBatch{
		Batch: api.Batch{
			Name:                 name,
			DatabaseID:           internal.INVALID_DATABASE_ID,
			Status:               api.BATCHSTATUS_DEFINED,
			StatusString:         api.BATCHSTATUS_DEFINED.String(),
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

// REVIEW: I think, it should be documented, how the matching works.
// E.g. it works differently as one might assume from e.g. .gitignore, since
// if it matches the exclude regex, it gets excluded regardless if it would be
// included by the includeRegex.
func (b *InternalBatch) InstanceMatchesCriteria(i instance.Instance) bool {
	// Handle any exclusionary criteria first.
	if b.ExcludeRegex != "" {
		// REVIEW: The regular expressions are user provided content. Are we sure,
		// that we would want to panic, if we fail to compile the regex?
		excludeRegex := regexp.MustCompile(b.ExcludeRegex)
		if excludeRegex.Match([]byte(i.GetName())) {
			return false
		}
	}

	// Handle any inclusionary criteria second.
	if b.IncludeRegex != "" {
		// REVIEW: The regular expressions are user provided content. Are we sure,
		// that we would want to panic, if we fail to compile the regex?
		includeRegex := regexp.MustCompile(b.IncludeRegex)
		if !includeRegex.Match([]byte(i.GetName())) {
			return false
		}
	}

	return true
}
