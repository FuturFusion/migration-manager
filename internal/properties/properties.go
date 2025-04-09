package properties

import (
	_ "embed"
	"fmt"
	"slices"

	"gopkg.in/yaml.v2"

	"github.com/FuturFusion/migration-manager/shared/api"
)

//go:embed instance_properties.yaml
var instanceData []byte
var instanceProperties []definition

type (
	// map of targets and sources, to versions, to property mappings.
	targetPropertyInfo map[api.TargetType]map[string]PropertyInfo
	sourcePropertyInfo map[api.SourceType]map[string]PropertyInfo
)

// PropertyInfo represents a type of property on a source or target, and the key to acces it.
type PropertyInfo struct {
	// Type is the type of entity on the target or source holding the configuration.
	Type PropertyType `json:"type,omitempty" yaml:"type,omitempty"`

	// Key represents the map key name holding the value for the propery.
	Key string `json:"key" yaml:"key"`
}

// definition represents a property definition from the schema file.
type definition struct {
	mapping

	// Name is the property name.
	Name Name `json:"name" yaml:"name"`

	// Description is a description of the property.
	Description string `json:"description" yaml:"description"`

	// SubProperties is the sub-properties of this property.
	SubProperties map[Name]mapping `json:"config" yaml:"config"`
}

// mapping is a set of source and target mappings for a property.
type mapping struct {
	// SourceDefinitions are a set of property definitions for sources.
	SourceDefinitions sourcePropertyInfo `json:"source" yaml:"source"`

	// TargetDefinitions are a set of property definitions for targets.
	TargetDefinitions targetPropertyInfo `json:"target" yaml:"target"`
}

func (d *definition) UnmarshalYAML(unmarshal func(any) error) error {
	type rawDefinition struct {
		Name              string             `json:"name" yaml:"name"`
		Description       string             `json:"description" yaml:"description"`
		SourceDefinitions sourcePropertyInfo `json:"source" yaml:"source"`
		TargetDefinitions targetPropertyInfo `json:"target" yaml:"target"`
		SubProperties     map[string]mapping `json:"config" yaml:"config"`
	}

	var raw rawDefinition
	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	parsedName, err := ParseInstanceProperty(raw.Name)
	if err != nil {
		return err
	}

	*d = definition{
		Name:        parsedName,
		Description: raw.Description,
		mapping: mapping{
			SourceDefinitions: raw.SourceDefinitions,
			TargetDefinitions: raw.TargetDefinitions,
		},
		SubProperties: map[Name]mapping{},
	}

	for name, def := range raw.SubProperties {
		var parsedName Name
		switch d.Name {
		case InstanceDisks:
			parsedName, err = ParseInstanceDiskProperty(name)
		case InstanceNICs:
			parsedName, err = ParseInstanceNICProperty(name)
		case InstanceSnapshots:
			parsedName, err = ParseInstanceSnapshotProperty(name)
		default:
			return fmt.Errorf("Unexpected sub-property %q for property %q", name, d.Name.String())
		}

		if err != nil {
			return err
		}

		d.SubProperties[parsedName] = def
	}

	return nil
}

// InitDefinitions initializes the global property list.
func InitDefinitions() error {
	var localProperties []definition
	err := yaml.Unmarshal(instanceData, &localProperties)
	if err != nil {
		return err
	}

	validateDefs := func(name Name, def mapping, validProperties []Name, isSubProperty bool) error {
		if !slices.Contains(validProperties, name) {
			return fmt.Errorf("Unsupported property name %q", name.String())
		}

		if len(def.SourceDefinitions) == 0 && len(def.TargetDefinitions) == 0 {
			return fmt.Errorf("Neither source nor target defintions defined for the property %q", name.String())
		}

		for tgt, verMap := range def.TargetDefinitions {
			if len(verMap) == 0 {
				return fmt.Errorf("Target %q defined with no version for property %q", tgt, name.String())
			}

			for version, info := range verMap {
				err := validateTargetVersion(tgt, version)
				if err != nil {
					return err
				}

				if !isSubProperty {
					validTypes, err := allPropertyTypes(tgt)
					if err != nil {
						return err
					}

					if !slices.Contains(validTypes, info.Type) {
						return fmt.Errorf("Unexpected property type %q for property %q for target %q in version %q", info.Type, name.String(), tgt, version)
					}
				} else if info.Type != "" {
					return fmt.Errorf("Sub-property %q type is set for target %q in version %q", name.String(), tgt, version)
				}

				if info.Key == "" && !HasSubProperties(name) {
					return fmt.Errorf("Property %q key unset for target %q in version %q", name.String(), tgt, version)
				}
			}
		}

		for src, verMap := range def.SourceDefinitions {
			if len(verMap) == 0 {
				return fmt.Errorf("Source %q defined with no version for property %q", src, name.String())
			}

			for version, info := range verMap {
				err := validateSourceVersion(src, version)
				if err != nil {
					return err
				}

				if !isSubProperty {
					validTypes, err := allPropertyTypes(src)
					if err != nil {
						return err
					}

					if !slices.Contains(validTypes, info.Type) {
						return fmt.Errorf("Unexpected property type %q for property %q for source %q in version %q", info.Type, name.String(), src, version)
					}
				} else if info.Type != "" {
					return fmt.Errorf("Sub-property %q type is set for source %q in version %q", name.String(), src, version)
				}

				if info.Key == "" {
					return fmt.Errorf("Property %q key unset for source %q in version %q", name.String(), src, version)
				}
			}
		}

		return nil
	}

	if len(localProperties) != len(allInstanceProperties()) {
		return fmt.Errorf("Properties file does not match expected properties (expected %d, found %d)", len(allInstanceProperties()), len(localProperties))
	}

	for _, def := range localProperties {
		err := validateDefs(def.Name, def.mapping, allInstanceProperties(), false)
		if err != nil {
			return err
		}

		var subProperties []Name
		switch def.Name {
		case InstanceDisks:
			subProperties = allInstanceDiskProperties()
		case InstanceNICs:
			subProperties = allInstanceNICProperties()
		case InstanceSnapshots:
			subProperties = allInstanceSnapshotProperties()
		}

		if len(def.SubProperties) != len(subProperties) {
			return fmt.Errorf("Properties file does not match expected sub-properties for %q (expected %d, found %d)", def.Name, len(allInstanceProperties()), len(localProperties))
		}

		if len(def.SubProperties) > 0 {
			for key, cfg := range def.SubProperties {
				err := validateDefs(key, cfg, subProperties, true)
				if err != nil {
					return err
				}
			}
		}
	}

	instanceProperties = localProperties

	return nil
}
