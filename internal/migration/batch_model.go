package migration

import (
	"fmt"
	"time"

	"github.com/lxc/incus/v6/shared/validate"

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

	Constraints []api.BatchConstraint `db:"marshal=json"`
	Config      api.BatchConfig       `db:"marshal=json"`
	Defaults    api.BatchDefaults     `db:"marshal=json"`
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
		var baseNetwork api.Network
		for _, net := range usedNetworks {
			if n.ID == net.SourceSpecificID && instance.Source == net.Source {
				apiNet, err := net.ToAPI()
				if err != nil {
					return nil, err
				}

				baseNetwork = *apiNet
				break
			}
		}

		if baseNetwork.SourceSpecificID == "" {
			return nil, fmt.Errorf("No network %q associated with instance %q on source %q", n.ID, instance.GetName(), instance.Source)
		}

		netCfg := baseNetwork.Placement
		netCfg.Apply(baseNetwork.Overrides)
		resp.Networks[n.ID] = netCfg
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

func (b Batch) Validate() error {
	if b.ID < 0 {
		return NewValidationErrf("Invalid batch, id can not be negative")
	}

	err := validate.IsAPIName(b.Name, false)
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

	exprs := map[string]bool{}
	for _, c := range b.Constraints {
		err := validate.IsAPIName(b.Name, false)
		if err != nil {
			return NewValidationErrf("Invalid constraint, %q is not a valid name: %v", b.Name, err)
		}

		if exprs[c.IncludeExpression] {
			return NewValidationErrf("Invalid batch constraint, include expression %q cannot be used more than once", c.IncludeExpression)
		}

		exprs[c.IncludeExpression] = true
		_, err = InstanceFilterable{}.CompileIncludeExpression(b.IncludeExpression)
		if err != nil {
			return NewValidationErrf("Invalid constraint %q is not a valid include expression: %v", b.IncludeExpression, err)
		}

		if c.MaxConcurrentInstances < 0 {
			return NewValidationErrf("Invalid constraint max concurrent instances must not be negative")
		}

		if c.MinInstanceBootTime != "" {
			bootTime, err := time.ParseDuration(c.MinInstanceBootTime)
			if err != nil {
				return NewValidationErrf("Invalid constraint minimum boot time %q: %v", c.MinInstanceBootTime, err)
			}

			if bootTime <= 0 {
				return NewValidationErrf("Invalid constraint minimum boot time %q must be greater than 0", c.MinInstanceBootTime)
			}
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

type Batches []Batch

// ToAPI returns the API representation of a batch.
func (b Batch) ToAPI(windows Windows) api.Batch {
	apiWindows := make([]api.MigrationWindow, len(windows))
	for i, w := range windows {
		apiWindows[i] = w.ToAPI()
	}

	return api.Batch{
		BatchPut: api.BatchPut{
			Name:              b.Name,
			IncludeExpression: b.IncludeExpression,
			MigrationWindows:  apiWindows,
			Constraints:       b.Constraints,
			Defaults:          b.Defaults,
			Config:            b.Config,
		},
		StartDate:     b.StartDate,
		Status:        b.Status,
		StatusMessage: b.StatusMessage,
	}
}
