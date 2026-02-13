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
			if n.SourceSpecificID == net.SourceSpecificID && instance.Source == net.Source {
				apiNet, err := net.ToAPI()
				if err != nil {
					return nil, err
				}

				baseNetwork = *apiNet
				break
			}
		}

		if baseNetwork.SourceSpecificID == "" {
			return nil, fmt.Errorf("No network %q associated with instance %q on source %q", n.SourceSpecificID, instance.GetName(), instance.Source)
		}

		netCfg := baseNetwork.Placement
		netCfg.Apply(baseNetwork.Overrides)
		resp.Networks[n.HardwareAddress] = netCfg
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

func (b Batch) CanStart() bool {
	switch b.Status {
	case api.BATCHSTATUS_DEFINED,
		api.BATCHSTATUS_ERROR,
		api.BATCHSTATUS_STOPPED:
		return true
	default:
		return false
	}
}

func (b Batch) Validate() error {
	if b.ID < 0 {
		return NewValidationErrf("Invalid batch, id can not be negative")
	}

	err := validate.IsAPIName(b.Name, false)
	if err != nil {
		return NewValidationErrf("Invalid batch, %q is not a valid name: %v", b.Name, err)
	}

	err = validate.IsAPIName(b.Defaults.Placement.Target, false)
	if err != nil {
		return NewValidationErrf("Invalid batch placement, default target %q is not a valid name: %v", b.Defaults.Placement.Target, err)
	}

	err = validate.IsAPIName(b.Defaults.Placement.TargetProject, false)
	if err != nil {
		return NewValidationErrf("Invalid batch placement, default target project %q is not a valid name: %v", b.Defaults.Placement.TargetProject, err)
	}

	err = validate.IsAPIName(b.Defaults.Placement.StoragePool, false)
	if err != nil {
		return NewValidationErrf("Invalid batch placement, default storage pool %q is not a valid name: %v", b.Defaults.Placement.StoragePool, err)
	}

	existingTargets := map[string]map[string]bool{}
	for _, netCfg := range b.Defaults.MigrationNetwork {
		err := validate.IsAPIName(netCfg.Target, false)
		if err != nil {
			return NewValidationErrf("Invalid batch placement, migration network target %q is not a valid name: %v", netCfg.Target, err)
		}

		err = validate.IsAPIName(netCfg.TargetProject, false)
		if err != nil {
			return NewValidationErrf("Invalid batch placement, migration network target project %q is not a valid name: %v", netCfg.TargetProject, err)
		}

		err = validate.IsAPIName(netCfg.Network, false)
		if err != nil {
			return NewValidationErrf("Invalid batch, target migration network name %q is not a valid name: %v", netCfg.Network, err)
		}

		err = netCfg.Validate()
		if err != nil {
			return NewValidationErrf("Invalid batch, target migration network for target %q and project %q is invalid: %v", netCfg.Target, netCfg.TargetProject, err)
		}

		if existingTargets[netCfg.Target] == nil {
			existingTargets[netCfg.Target] = map[string]bool{}
		}

		if existingTargets[netCfg.Target][netCfg.TargetProject] {
			return NewValidationErrf("Invalid batch, more than one migration network defined for target %q and project %q", netCfg.Target, netCfg.TargetProject)
		}

		existingTargets[netCfg.Target][netCfg.TargetProject] = true
	}

	err = b.Status.Validate()
	if err != nil {
		return NewValidationErrf("Invalid status: %v", err)
	}

	_, _, err = Instance{}.CompileIncludeExpression(b.IncludeExpression, false)
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
		_, _, err = Instance{}.CompileIncludeExpression(b.IncludeExpression, false)
		if err != nil {
			return NewValidationErrf("Invalid constraint %q is not a valid include expression: %v", b.IncludeExpression, err)
		}

		if c.MaxConcurrentInstances < 0 {
			return NewValidationErrf("Invalid constraint max concurrent instances must not be negative")
		}

		if c.MinInstanceBootTime != (api.Duration{}) {
			if c.MinInstanceBootTime.Duration <= 0 {
				return NewValidationErrf("Invalid constraint minimum boot time %q must be greater than 0", c.MinInstanceBootTime.String())
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

	if b.Config.BackgroundSyncInterval.Duration <= 0 {
		return NewValidationErrf("Invalid background sync interval %q", b.Config.BackgroundSyncInterval)
	}

	if b.Config.FinalBackgroundSyncLimit.Duration <= 0 {
		return NewValidationErrf("Invalid final background sync limit %q", b.Config.FinalBackgroundSyncLimit)
	}

	if b.Config.FinalBackgroundSyncLimit.Duration > b.Config.BackgroundSyncInterval.Duration {
		return NewValidationErrf("Final background sync limit %q cannot be greater than the background sync interval %q", b.Config.FinalBackgroundSyncLimit, b.Config.BackgroundSyncInterval)
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
