package migration

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/google/uuid"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Instance struct {
	ID   int64
	UUID uuid.UUID `db:"primary=yes"`

	Source               string         `db:"join=sources.name&order=yes"`
	SourceType           api.SourceType `db:"join=sources.source_type&omit=create,update"`
	LastUpdateFromSource time.Time

	Overrides  api.InstanceOverride   `db:"marshal=json"`
	Properties api.InstanceProperties `db:"marshal=json"`
}

type InstanceFilterable struct {
	api.InstanceProperties

	Source               string         `expr:"source"`
	SourceType           api.SourceType `expr:"source_type"`
	LastUpdateFromSource time.Time      `expr:"last_update_from_source"`
}

func (i Instance) Validate() error {
	if i.UUID == uuid.Nil {
		return NewValidationErrf("Invalid instance, UUID can not be empty")
	}

	if i.Properties.Location == "" {
		return NewValidationErrf("Invalid instance, inventory path can not be empty")
	}

	if i.Properties.Name == "" {
		return NewValidationErrf("Invalid instance, name can not be empty")
	}

	if i.Source == "" {
		return NewValidationErrf("Invalid instance, source id can not be empty")
	}

	return nil
}

// DisabledReason returns the underlying reason for why the instance is disabled.
func (i Instance) DisabledReason(overrides api.InstanceRestrictionOverride) error {
	if i.Overrides.DisableMigration {
		return fmt.Errorf("Migration is manually disabled")
	}

	if i.Overrides.IgnoreRestrictions {
		return nil
	}

	if i.Properties.OS == "" || i.Properties.OSVersion == "" {
		if !overrides.AllowUnknownOS {
			return fmt.Errorf("Could not determine instance OS, check if guest agent is running")
		}
	}

	ipRestrict := len(i.Properties.NICs) > 0
	for _, nic := range i.Properties.NICs {
		if nic.IPv4Address != "" {
			ipRestrict = false
			break
		}
	}

	if ipRestrict && !overrides.AllowNoIPv4 {
		return fmt.Errorf("Could not determine instance IP, check if guest agent is running")
	}

	if !i.Properties.BackgroundImport && !overrides.AllowNoBackgroundImport {
		return fmt.Errorf("Background import is not supported")
	}

	for _, d := range i.Properties.Disks {
		if !d.Supported {
			return fmt.Errorf("Disk %q does not support snapshots", d.Name)
		}
	}

	return nil
}

// GetName returns the name of the instance, which may not be unique among all instances for a given source.
// If a unique, human-readable identifier is needed, use the Location property.
func (i Instance) GetName() string {
	return i.Properties.Name
}

// GetOSType returns the OS type, as determined from https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.GuestOsDescriptor.GuestOsIdentifier.html
func (i *Instance) GetOSType() api.OSType {
	if strings.HasPrefix(strings.ToLower(i.Properties.OS), "win") {
		return api.OSTYPE_WINDOWS
	}

	if strings.HasPrefix(i.Properties.Description, "FortiGate") {
		return api.OSTYPE_FORTIGATE
	}

	return api.OSTYPE_LINUX
}

func (i Instance) MatchesCriteria(expression string) (bool, error) {
	filterable := i.ToFilterable()
	includeExpr, err := filterable.CompileIncludeExpression(expression)
	if err != nil {
		return false, fmt.Errorf("Failed to compile include expression %q: %v", expression, err)
	}

	output, err := expr.Run(includeExpr, filterable)
	if err != nil {
		return false, fmt.Errorf("Failed to run include expression %q with instance %v: %v", expression, filterable, err)
	}

	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("Include expression %q does not evaluate to boolean result: %v", expression, output)
	}

	return result, nil
}

func (i InstanceFilterable) CompileIncludeExpression(expression string) (*vm.Program, error) {
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

	return expr.Compile(expression, options...)
}

type Instances []Instance

func (i Instance) ToFilterable() InstanceFilterable {
	props := i.Properties
	props.Apply(i.Overrides.Properties)

	return InstanceFilterable{
		InstanceProperties:   props,
		Source:               i.Source,
		SourceType:           i.SourceType,
		LastUpdateFromSource: i.LastUpdateFromSource,
	}
}

func (i Instance) ToAPI() api.Instance {
	apiInst := api.Instance{
		Source:               i.Source,
		SourceType:           i.SourceType,
		LastUpdateFromSource: i.LastUpdateFromSource,
		Properties:           i.Properties,
		Overrides:            i.Overrides,
	}

	return apiInst
}
