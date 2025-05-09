package migration

import (
	"fmt"
	"time"

	"github.com/lxc/incus/v6/shared/validate"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Batch struct {
	ID                int64
	Name              string `db:"primary=yes"`
	Target            string `db:"join=targets.name"`
	TargetProject     string
	Status            api.BatchStatusType
	StatusMessage     string
	StoragePool       string
	IncludeExpression string

	Constraints []BatchConstraint `db:"marshal=json"`
}

type BatchConstraint struct {
	// Name of the constraint.
	Name string `json:"name" yaml:"name"`

	// Description of the constraint.
	Description string `json:"description" yaml:"description"`

	// Expression used to select instances for the constraint.
	IncludeExpression string `json:"include_expression" yaml:"include_expression"`

	// Maximum amount of matched instances that can concurrently migrate, before moving to the next migration window.
	MaxConcurrentInstances int `json:"max_concurrent_instances" yaml:"max_concurrent_instances"`

	// Minimum amount of time required for an instance to boot after initial disk import. Migration window duration must be at least this much.
	MinInstanceBootTime time.Duration `json:"min_instance_boot_time" yaml:"min_instance_boot_time"`
}

type MigrationWindows []MigrationWindow

type MigrationWindow struct {
	ID      int64
	Start   time.Time `db:"primary=yes"`
	End     time.Time `db:"primary=yes"`
	Lockout time.Time `db:"primary=yes"`
}

func (b Batch) Validate() error {
	if b.ID < 0 {
		return NewValidationErrf("Invalid batch, id can not be negative")
	}

	err := validate.IsHostname(b.Name)
	if err != nil {
		return NewValidationErrf("Invalid batch, %q is not a valid name: %v", b.Name, err)
	}

	if b.Target == "" {
		return NewValidationErrf("Invalid batch, target can not be empty")
	}

	err = b.Status.Validate()
	if err != nil {
		return NewValidationErrf("Invalid status: %v", err)
	}

	_, err = InstanceFilterable{}.CompileIncludeExpression(b.IncludeExpression)
	if err != nil {
		return NewValidationErrf("Invalid batch %q is not a valid include expression: %v", b.IncludeExpression, err)
	}

	for _, c := range b.Constraints {
		err := c.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

func (b BatchConstraint) Validate() error {
	err := validate.IsHostname(b.Name)
	if err != nil {
		return NewValidationErrf("Invalid constraint, %q is not a valid name: %v", b.Name, err)
	}

	_, err = InstanceFilterable{}.CompileIncludeExpression(b.IncludeExpression)
	if err != nil {
		return NewValidationErrf("Invalid constraint %q is not a valid include expression: %v", b.IncludeExpression, err)
	}

	if b.MaxConcurrentInstances < 0 {
		return NewValidationErrf("Invalid constraint max concurrent instances must not be negative")
	}

	if b.MinInstanceBootTime < 0 {
		return NewValidationErrf("Invalid constraint minimum migration time must not be negative")
	}

	return nil
}

// GetEarliest returns the earliest valid migration window, or an error if none are found.
func (ws MigrationWindows) GetEarliest() (*MigrationWindow, error) {
	var earliestWindow *MigrationWindow
	if len(ws) == 0 {
		return &MigrationWindow{}, nil
	}

	for _, w := range ws {
		if w.Validate() != nil {
			continue
		}

		if earliestWindow == nil || w.Start.Before(earliestWindow.Start) {
			earliestWindow = &w
		}
	}

	if earliestWindow == nil {
		return nil, fmt.Errorf("No valid migration window found")
	}

	return earliestWindow, nil
}

// Begun returns whether the migration window has begun (whether its start time and lockout time are both in the past).
func (w MigrationWindow) Begun() bool {
	started := w.Start.IsZero() || w.Start.Before(time.Now().UTC())
	pastLockout := w.Lockout.IsZero() || w.Lockout.Before(time.Now().UTC())

	return started && pastLockout
}

func (w MigrationWindow) FitsDuration(duration time.Duration) bool {
	if w.Validate() != nil {
		return false
	}

	// If the end time is infinite, then the duration fits.
	if w.End.IsZero() {
		return true
	}

	// If the window has already started, make the comparison to now instead.
	start := w.Start
	if start.Before(time.Now().UTC()) {
		start = time.Now().UTC()
	}

	// TODO: Make this configurable per instance, once we tie instances to a migration window.
	// Set a buffer for the instance to revert migration.
	if duration > 0 {
		duration += time.Minute
	}

	return start.Add(duration).Before(w.End)
}

// Key returns an identifying key for the MigrationWindow, based on its timings.
func (w MigrationWindow) Key() string {
	return w.Start.String() + "_" + w.End.String() + "_" + w.Lockout.String()
}

func (w MigrationWindow) Validate() error {
	// If a migration window is defined, ensure sure it makes sense.
	if !w.Start.IsZero() && !w.End.IsZero() && w.End.Before(w.Start) {
		return fmt.Errorf("Batch migration window end time is before start time")
	}

	if !w.End.IsZero() && w.End.Before(time.Now().UTC()) {
		return fmt.Errorf("Batch migration window has already passed")
	}

	if !w.Lockout.IsZero() && w.Start.Before(w.Lockout) {
		return fmt.Errorf("Batch migration window lockout time is after the start time")
	}

	if !w.Lockout.IsZero() && w.End.Before(w.Lockout) {
		return fmt.Errorf("Batch migration window lockout time is after the end time")
	}

	return nil
}

func (b Batch) CanStart(windows []MigrationWindow) error {
	if b.Status != api.BATCHSTATUS_DEFINED && b.Status != api.BATCHSTATUS_QUEUED {
		return fmt.Errorf("Batch %q in state %q cannot be started", b.Name, string(b.Status))
	}

	var hasValidWindow bool
	for _, w := range windows {
		// Skip any migration windows that have since passed.
		if w.Validate() != nil {
			continue
		}

		hasValidWindow = true
		break
	}

	if !hasValidWindow {
		return fmt.Errorf("No valid migration windows found for batch %q", b.Name)
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

type Batches []Batch

// ToAPI returns the API representation of a batch.
func (b Batch) ToAPI(windows MigrationWindows) api.Batch {
	apiWindows := make([]api.MigrationWindow, len(windows))
	for i, w := range windows {
		apiWindows[i] = api.MigrationWindow{Start: w.Start, End: w.End, Lockout: w.Lockout}
	}

	constraints := make([]api.BatchConstraint, len(b.Constraints))

	for i, c := range b.Constraints {
		constraints[i] = api.BatchConstraint{
			Name:                   c.Name,
			Description:            c.Description,
			IncludeExpression:      c.IncludeExpression,
			MaxConcurrentInstances: c.MaxConcurrentInstances,
			MinInstanceBootTime:    c.MinInstanceBootTime.String(),
		}
	}

	return api.Batch{
		BatchPut: api.BatchPut{
			Name:              b.Name,
			Target:            b.Target,
			TargetProject:     b.TargetProject,
			StoragePool:       b.StoragePool,
			IncludeExpression: b.IncludeExpression,
			MigrationWindows:  apiWindows,
			Constraints:       constraints,
		},
		Status:        b.Status,
		StatusMessage: b.StatusMessage,
	}
}
