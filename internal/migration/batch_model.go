package migration

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/validate"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
	"github.com/FuturFusion/migration-manager/internal/scriptlet"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Batch struct {
	ID                int64
	Name              string `db:"primary=yes"`
	Status            api.BatchStatusType
	StatusMessage     string
	IncludeExpression string

	StartDate time.Time

	Constraints []BatchConstraint `db:"marshal=json"`
	Config      api.BatchConfig   `db:"marshal=json"`
	Defaults    api.BatchDefaults `db:"marshal=json"`
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

// GetIncusPlacement returns a TargetPlacement for the given instance and its networks.
// It defaults to the batch-level definitions unless the given TargetPlacement has overridden them.
func (b *Batch) GetIncusPlacement(instance Instance, usedNetworks Networks, placement api.Placement) (*api.Placement, error) {
	resp := &api.Placement{
		TargetName:    b.Defaults.Placement.Target,
		TargetProject: b.Defaults.Placement.TargetProject,
		StoragePools:  map[string]string{},
		Networks:      map[string]api.NetworkPlacement{},
	}

	// Use the same pool for all supported disks by default.
	for _, d := range instance.Properties.Disks {
		if !d.Supported {
			continue
		}

		resp.StoragePools[d.Name] = b.Defaults.Placement.StoragePool
	}

	// Handle per-network overrides.
	for _, n := range instance.Properties.NICs {
		var baseNetwork Network
		for _, net := range usedNetworks {
			if n.ID == net.Identifier && instance.Source == net.Source {
				baseNetwork = net
				break
			}
		}

		if baseNetwork.Identifier == "" {
			err := fmt.Errorf("No network %q associated with instance %q on source %q", n.ID, instance.Properties.Name, instance.Source)
			return nil, err
		}

		netCfg, err := internalAPI.GetNetworkPlacement(baseNetwork.ToAPI())
		if err != nil {
			return nil, fmt.Errorf("Unable to determine placement for neteork %q: %w", baseNetwork.ToAPI().Name(), err)
		}

		resp.Networks[n.ID] = *netCfg
	}

	// Override with placement values if set.
	if placement.TargetName != "" {
		resp.TargetName = placement.TargetName
	}

	if placement.TargetProject != "" {
		resp.TargetProject = placement.TargetProject
	}

	for id, netCfg := range placement.Networks {
		resp.Networks[id] = netCfg
	}

	for disk, pool := range placement.StoragePools {
		resp.StoragePools[disk] = pool
	}

	return resp, nil
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

	if b.Defaults.Placement.Target == "" {
		return NewValidationErrf("Invalid batch placement, target can not be empty")
	}

	if b.Defaults.Placement.TargetProject == "" {
		return NewValidationErrf("Invalid batch placement, default target project can not be empty")
	}

	if b.Defaults.Placement.StoragePool == "" {
		return NewValidationErrf("Invalid batch placement, default target storage pool can not be empty")
	}

	err = b.Status.Validate()
	if err != nil {
		return NewValidationErrf("Invalid status: %v", err)
	}

	_, err = InstanceFilterable{}.CompileIncludeExpression(b.IncludeExpression)
	if err != nil {
		return NewValidationErrf("Invalid batch %q is not a valid include expression: %v", b.IncludeExpression, err)
	}

	if b.Config.PostMigrationRetries < 0 {
		return NewValidationErrf("Invalid batch, post-migration retry count (%d) must be larger than 0", b.Config.PostMigrationRetries)
	}

	for _, c := range b.Constraints {
		err := c.Validate()
		if err != nil {
			return err
		}
	}

	if b.Status == api.BATCHSTATUS_DEFINED && !b.StartDate.IsZero() {
		return NewValidationErrf("Cannot set start time before batch %q has started", b.Name)
	}

	if b.Config.PlacementScriptlet != "" {
		err := scriptlet.BatchPlacementValidate(b.Config.PlacementScriptlet, b.Name)
		if err != nil {
			return NewValidationErrf("Invalid placement scriptlet: %v", err)
		}
	}

	syncInterval, err := time.ParseDuration(b.Config.BackgroundSyncInterval)
	if err != nil {
		return NewValidationErrf("Invalid background sync interval %q: %v", b.Config.BackgroundSyncInterval, err)
	}

	finalSyncLimit, err := time.ParseDuration(b.Config.FinalBackgroundSyncLimit)
	if err != nil {
		return NewValidationErrf("Invalid final background sync limit %q: %v", b.Config.FinalBackgroundSyncLimit, err)
	}

	if finalSyncLimit > syncInterval {
		return NewValidationErrf("Final background sync limit %q cannot be greater than the background sync interval %q", b.Config.FinalBackgroundSyncLimit, b.Config.BackgroundSyncInterval)
	}

	if finalSyncLimit <= 0 {
		return NewValidationErrf("Final background sync limit %q must be greater than 0", b.Config.FinalBackgroundSyncLimit)
	}

	if syncInterval <= 0 {
		return NewValidationErrf("Background sync interval %q must be greater than 0", b.Config.FinalBackgroundSyncLimit)
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

		if w.Locked() {
			continue
		}

		if earliestWindow == nil || w.Start.Before(earliestWindow.Start) {
			earliestWindow = &w
		}
	}

	if earliestWindow == nil {
		return nil, incusAPI.StatusErrorf(http.StatusNotFound, "No valid migration window found")
	}

	return earliestWindow, nil
}

func (w MigrationWindow) IsEmpty() bool {
	return w.Start.IsZero() && w.End.IsZero() && w.Lockout.IsZero()
}

// Begun returns whether the migration window has begun (whether its start time is in the past).
func (w MigrationWindow) Begun() bool {
	started := w.Start.IsZero() || w.Start.Before(time.Now().UTC())

	return started
}

func (w MigrationWindow) Locked() bool {
	locked := !w.Lockout.IsZero() && w.Lockout.Before(time.Now().UTC())

	return w.Ended() || locked
}

func (w MigrationWindow) Ended() bool {
	ended := !w.End.IsZero() && w.End.Before(time.Now().UTC())
	return ended
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

func (ws MigrationWindows) Validate() error {
	// Sort the windows by their start times.
	sort.Slice(ws, func(i, j int) bool {
		return ws[i].Start.Before(ws[j].Start)
	})

	for i, w := range ws {
		// Perform individual window validation.
		err := w.Validate()
		if err != nil {
			return err
		}

		// If the current window starts before the earlier window's end time, then they overlap.
		if i > 0 {
			if ws[i].Start.Before(ws[i-1].End) {
				return fmt.Errorf("Window %d with start time %q overlaps with window %d with end time %q", i, ws[i].Start.String(), i-1, ws[i-1].End.String())
			}
		}
	}

	return nil
}

func (w MigrationWindow) Validate() error {
	// If a migration window is defined, ensure sure it makes sense.
	if !w.Start.IsZero() && !w.End.IsZero() && w.End.Before(w.Start) {
		return fmt.Errorf("Batch migration window end time is before start time")
	}

	if !w.End.IsZero() && w.End.Before(time.Now().UTC()) {
		return fmt.Errorf("Batch migration window has already passed")
	}

	if !w.Lockout.IsZero() && w.Start.After(w.Lockout) {
		return fmt.Errorf("Batch migration window lockout time is before the start time")
	}

	if !w.Lockout.IsZero() && w.End.Before(w.Lockout) {
		return fmt.Errorf("Batch migration window lockout time is after the end time")
	}

	return nil
}

func (b Batch) HasValidWindow(windows []MigrationWindow) error {
	hasValidWindow := len(windows) == 0
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
			IncludeExpression: b.IncludeExpression,
			MigrationWindows:  apiWindows,
			Constraints:       constraints,
			Defaults:          b.Defaults,
			Config:            b.Config,
		},
		StartDate:     b.StartDate,
		Status:        b.Status,
		StatusMessage: b.StatusMessage,
	}
}
