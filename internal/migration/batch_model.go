package migration

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Batch struct {
	ID                   int64
	Name                 string `db:"primary=yes"`
	Target               string `db:"join=targets.name"`
	TargetProject        string
	Status               api.BatchStatusType
	StatusMessage        string
	StoragePool          string
	IncludeExpression    string
	MigrationWindowStart time.Time
	MigrationWindowEnd   time.Time
}

func (b Batch) Validate() error {
	if b.ID < 0 {
		return NewValidationErrf("Invalid batch, id can not be negative")
	}

	if b.Name == "" {
		return NewValidationErrf("Invalid batch, name can not be empty")
	}

	if b.Target == "" {
		return NewValidationErrf("Invalid batch, target can not be empty")
	}

	err := b.Status.Validate()
	if err != nil {
		return NewValidationErrf("Invalid status: %v", err)
	}

	_, err = b.CompileIncludeExpression(InstanceFilterable{})
	if err != nil {
		return NewValidationErrf("Invalid batch %q is not a valid include expression: %v", b.IncludeExpression, err)
	}

	return nil
}

func (b Batch) CanStart() error {
	if b.Status != api.BATCHSTATUS_DEFINED && b.Status != api.BATCHSTATUS_QUEUED {
		return fmt.Errorf("Batch %q in state %q cannot be started", b.Name, string(b.Status))
	}

	// If a migration window is defined, ensure sure it makes sense.
	if !b.MigrationWindowStart.IsZero() && !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(b.MigrationWindowStart) {
		return fmt.Errorf("Batch %q window end time is before start time", b.Name)
	}

	if !b.MigrationWindowEnd.IsZero() && b.MigrationWindowEnd.Before(time.Now().UTC()) {
		return fmt.Errorf("Batch %q migration window has already passed", b.Name)
	}

	return nil
}

func (b Batch) CanBeModified() bool {
	switch b.Status {
	case api.BATCHSTATUS_DEFINED,
		api.BATCHSTATUS_FINISHED,
		api.BATCHSTATUS_ERROR:
		return true
	default:
		return false
	}
}

func (b Batch) InstanceMatchesCriteria(instance Instance) (bool, error) {
	filterable := instance.ToFilterable()
	includeExpr, err := b.CompileIncludeExpression(filterable)
	if err != nil {
		return false, fmt.Errorf("Failed to compile include expression %q: %v", b.IncludeExpression, err)
	}

	output, err := expr.Run(includeExpr, filterable)
	if err != nil {
		return false, fmt.Errorf("Failed to run include expression %q with instance %v: %v", b.IncludeExpression, filterable, err)
	}

	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("Include expression %q does not evaluate to boolean result: %v", b.IncludeExpression, output)
	}

	return result, nil
}

func (b Batch) CompileIncludeExpression(i InstanceFilterable) (*vm.Program, error) {
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

type Batches []Batch

// ToAPI returns the API representation of a batch.
func (b Batch) ToAPI() api.Batch {
	return api.Batch{
		BatchPut: api.BatchPut{
			Name:                 b.Name,
			Target:               b.Target,
			TargetProject:        b.TargetProject,
			StoragePool:          b.StoragePool,
			IncludeExpression:    b.IncludeExpression,
			MigrationWindowStart: b.MigrationWindowStart,
			MigrationWindowEnd:   b.MigrationWindowEnd,
		},
		Status:        b.Status,
		StatusMessage: b.StatusMessage,
	}
}
