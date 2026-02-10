package migration

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/validate"

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

	if i.Overrides.Name != "" {
		err := validate.IsHostname(i.Overrides.Name)
		if err != nil {
			return NewValidationErrf("Invalid instance override, name %q is not a valid hostname: %v", i.Overrides.Name, err)
		}
	}

	for _, nic := range i.Properties.NICs {
		if nic.UUID == uuid.Nil {
			return NewValidationErrf("Instance NIC %q has empty UUID", nic.Location)
		}
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

	props := i.Properties
	props.Apply(i.Overrides.InstancePropertiesConfigurable)
	err := validate.IsHostname(props.Name)
	if err != nil {
		return fmt.Errorf("Instance name %q is not a valid hostname: %w", props.Name, err)
	}

	if props.OS == "" || props.OSVersion == "" {
		if !overrides.AllowUnknownOS {
			return fmt.Errorf("Could not determine instance OS, check if guest agent is running")
		}
	}

	if props.Architecture == "" {
		return fmt.Errorf("Could not determine instance architecture, check if guest agent is running")
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

	if !i.Properties.SupportsBackgroundImport() && !overrides.AllowNoBackgroundImport {
		if i.Properties.BackgroundImport {
			return fmt.Errorf("Verifying background import support")
		}

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
	props := i.Properties
	props.Apply(i.Overrides.InstancePropertiesConfigurable)

	return props.Name
}

func (i Instance) NeedsBackgroundImportVerification() bool {
	if i.Properties.BackgroundImport {
		for _, disk := range i.Properties.Disks {
			if disk.Supported && !disk.BackgroundImportVerified {
				return true
			}
		}
	}

	return false
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
	filterable, includeExpr, err := i.CompileIncludeExpression(expression)
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

func (i Instance) CompileIncludeExpression(expression string) (*api.InstanceFilterable, *vm.Program, error) {
	filterable := i.ToAPI().ToFilterable()
	matchTag := func(exact bool, params ...any) (any, error) {
		if len(params) != 2 {
			return nil, fmt.Errorf("invalid number of arguments, expected <category> <tag>, got %d arguments", len(params))
		}

		category, ok := params[0].(string)
		if !ok {
			return nil, fmt.Errorf("invalid category argument type, expected string, got: %T", params[0])
		}

		tag, ok := params[1].(string)
		if !ok {
			return nil, fmt.Errorf("invalid tag argument type, expected string, got: %T", params[0])
		}

		containsFunc := func(s string) bool {
			return s == tag
		}

		if exact {
			containsFunc = func(s string) bool {
				return strings.Contains(s, tag)
			}
		}

		if category == "*" {
			for k, v := range filterable.Config {
				if strings.HasPrefix(k, "tag.") && containsFunc(v) {
					return true, nil
				}
			}

			return false, nil
		}

		tagList, ok := filterable.Config["tag."+category]
		if !ok {
			return false, nil
		}

		if slices.ContainsFunc(strings.Split(tagList, ","), containsFunc) {
			return true, nil
		}

		return false, nil
	}

	customFunctions := append([]expr.Option{}, pathFunctions...)
	customFunctions = append(customFunctions,
		expr.Function("has_tag", func(params ...any) (any, error) {
			return matchTag(true, params...)
		}),

		expr.Function("matches_tag", func(params ...any) (any, error) {
			return matchTag(false, params...)
		}),
	)

	// Instantiate all nil fields when compiling the expression for consistency.
	baseEnv := api.InstanceFilterable{
		InstanceProperties: api.InstanceProperties{
			InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{
				Config: map[string]string{},
			},
			NICs:      []api.InstancePropertiesNIC{},
			Disks:     []api.InstancePropertiesDisk{},
			Snapshots: []api.InstancePropertiesSnapshot{},
		},
	}

	options := append([]expr.Option{expr.Env(baseEnv)}, customFunctions...)

	program, err := expr.Compile(expression, options...)
	if err != nil {
		return nil, nil, err
	}

	return &filterable, program, nil
}

type Instances []Instance

func (i Instance) ToAPI() api.Instance {
	apiInst := api.Instance{
		Source:               i.Source,
		SourceType:           i.SourceType,
		LastUpdateFromSource: i.LastUpdateFromSource,
		InstanceProperties:   i.Properties,
		Overrides:            i.Overrides,
	}

	return apiInst
}
