package batch

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalBatch struct {
	api.Batch `yaml:",inline"`
}

// Returns a new Batch ready for use.
func NewBatch(name string, targetID int, storagePool string, includeExpression string, migrationWindowStart time.Time, migrationWindowEnd time.Time, defaultNetwork string) *InternalBatch {
	return &InternalBatch{
		Batch: api.Batch{
			Name:                 name,
			DatabaseID:           internal.INVALID_DATABASE_ID,
			TargetID:             targetID,
			Status:               api.BATCHSTATUS_DEFINED,
			StatusString:         api.BATCHSTATUS_DEFINED.String(),
			StoragePool:          storagePool,
			IncludeExpression:    includeExpression,
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

type InstanceWithDetails struct {
	Name              string
	InventoryPath     string
	Annotation        string
	GuestToolsVersion int
	Architecture      string
	HardwareVersion   string
	OS                string
	OSVersion         string
	Devices           []api.InstanceDeviceInfo
	Disks             []api.InstanceDiskInfo
	NICs              []api.InstanceNICInfo
	Snapshots         []api.InstanceSnapshotInfo
	CPU               api.InstanceCPUInfo
	Memory            api.InstanceMemoryInfo
	UseLegacyBios     bool
	SecureBootEnabled bool
	TPMPresent        bool

	Source    Source
	Overrides api.InstanceOverride
}

type Source struct {
	Name       string
	SourceType string
}

type Overrides struct {
	Comment          string
	NumberCPUs       int
	MemoryInBytes    int64
	DisableMigration bool
}

func (b *InternalBatch) InstanceMatchesCriteria(i InstanceWithDetails) (bool, error) {
	includeExpr, err := b.CompileIncludeExpression(i)
	if err != nil {
		return false, fmt.Errorf("Failed to compile include expression %q: %v", b.IncludeExpression, err)
	}

	output, err := expr.Run(includeExpr, i)
	if err != nil {
		return false, fmt.Errorf("Failed to run include expression %q with instance %v: %v", b.IncludeExpression, i, err)
	}

	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("Include expression %q does not evaluate to boolean result: %v", b.IncludeExpression, output)
	}

	return result, nil
}

func (b *InternalBatch) CompileIncludeExpression(i InstanceWithDetails) (*vm.Program, error) {
	customFunctions := []expr.Option{
		expr.Function("path_base", func(params ...any) (any, error) {
			if len(params) != 1 {
				return nil, fmt.Errorf("invalid number of arguments, expected 1, got: %d", len(params))
			}

			path, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("invalid argument type, expected string, got: %T", params[0])
			}

			return filepath.Base(path), nil
		}),

		expr.Function("path_dir", func(params ...any) (any, error) {
			if len(params) != 1 {
				return nil, fmt.Errorf("invalid number of arguments, expected 1, got: %d", len(params))
			}

			path, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("invalid argument type, expected string, got: %T", params[0])
			}

			return filepath.Dir(path), nil
		}),
	}

	options := append([]expr.Option{expr.Env(i)}, customFunctions...)

	return expr.Compile(b.IncludeExpression, options...)
}
