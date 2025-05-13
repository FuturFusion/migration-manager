package properties

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/osarch"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// RawPropertySet holds the set of definitions for a particular source or target and version.
type RawPropertySet[T api.SourceType | api.TargetType] struct {
	// Source or target type.
	propType T

	// Source or target version.
	version string

	// Set of top-level property definitions, by name.
	props map[Name]PropertyInfo

	// A set of RawPropertySet which are sub-properties of the keyed property name.
	subProps map[Name]RawPropertySet[T]

	// Mapping of top-level properties to their determined value on the source or target.
	propValues map[Name]any

	// A list of sub-property mappings to their value, for each parent property.
	subPropValues map[Name][]map[Name]any
}

// HasSubProperties returns whether this is a property with sub-properties.
func HasSubProperties(name Name) bool {
	return slices.Contains([]Name{InstanceDisks, InstanceNICs, InstanceSnapshots}, name)
}

// newRawPropertySet instantiates a new RawPropertySet.
func newRawPropertySet[T api.SourceType | api.TargetType](t T, version string) RawPropertySet[T] {
	return RawPropertySet[T]{
		propType: t,
		version:  version,

		props:    map[Name]PropertyInfo{},
		subProps: map[Name]RawPropertySet[T]{},

		propValues:    map[Name]any{},
		subPropValues: map[Name][]map[Name]any{},
	}
}

// Definitions generates a new RawPropertySet with all supported property definitions for the given target or source and version.
func Definitions[T api.SourceType | api.TargetType](t T, version string) (RawPropertySet[T], error) {
	defs := newRawPropertySet(t, version)
	getVersionInfo := func(t T, version string, versionMap map[string]PropertyInfo) (PropertyInfo, error) {
		var highestVersion string
		var lastErr error
		for defVer := range versionMap {
			err := compareVersions(t, version, defVer)
			if err != nil {
				lastErr = err
			} else if defVer > highestVersion {
				highestVersion = defVer
			}
		}

		if highestVersion != "" {
			return versionMap[highestVersion], nil
		}

		if lastErr == nil {
			lastErr = fmt.Errorf("No supported versions found for source or target %q", t)
		}

		return PropertyInfo{}, lastErr
	}

	for _, p := range instanceProperties {
		var versionMap map[string]PropertyInfo
		var ok bool
		switch t := any(t).(type) {
		case api.SourceType:
			versionMap, ok = p.SourceDefinitions[t]
		case api.TargetType:
			versionMap, ok = p.TargetDefinitions[t]
		}

		if !ok {
			// No definition for the target or source.
			continue
		}

		info, err := getVersionInfo(t, version, versionMap)
		if err != nil {
			return RawPropertySet[T]{}, err
		}

		defs.props[p.Name] = info
		if HasSubProperties(p.Name) {
			defs.subProps[p.Name] = newRawPropertySet(t, version)
			for defName, subProp := range p.SubProperties {
				var versionMap map[string]PropertyInfo
				var ok bool
				switch t := any(t).(type) {
				case api.SourceType:
					versionMap, ok = subProp.SourceDefinitions[t]
				case api.TargetType:
					versionMap, ok = subProp.TargetDefinitions[t]
				}

				if !ok {
					// No definition for the target or source.
					continue
				}

				info, err := getVersionInfo(t, version, versionMap)
				if err != nil {
					return RawPropertySet[T]{}, err
				}

				defs.subProps[p.Name].props[defName] = info
			}
		}
	}

	return defs, nil
}

// GetAll returns a map of all properties and their definitions supported by this target or source.
func (p RawPropertySet[T]) GetAll() map[Name]PropertyInfo {
	props := make(map[Name]PropertyInfo, len(p.props))
	for k, v := range p.props {
		props[k] = v
	}

	return props
}

// Get returns the named property definition, if supported by this target or source.
func (p RawPropertySet[T]) Get(n Name) (PropertyInfo, error) {
	info, ok := p.props[n]
	if !ok {
		return PropertyInfo{}, fmt.Errorf("Property %q is not supported by %q version %q", n.String(), p.propType, p.version)
	}

	return info, nil
}

// GetValue returns the stored value for the property, if one exists. Cannot be used to fetch a group of sub-properties.
func (p RawPropertySet[T]) GetValue(n Name) (any, error) {
	if HasSubProperties(n) {
		return nil, fmt.Errorf("Cannot fetch value of property %q with sub-properties", n.String())
	}

	val, ok := p.propValues[n]
	if !ok {
		return nil, fmt.Errorf("No value assigned to property %q", n.String())
	}

	return val, nil
}

// GetSubProperties returns the RawPropertySet for the sub-properties of the named property, if supported by this target or source.
func (p RawPropertySet[T]) GetSubProperties(n Name) (RawPropertySet[T], error) {
	if !HasSubProperties(n) {
		return RawPropertySet[T]{}, fmt.Errorf("Property %q does not support sub-properties", n.String())
	}

	subProps, ok := p.subProps[n]
	if !ok {
		return RawPropertySet[T]{}, fmt.Errorf("Sub-property %q is not supported by %q version %q", n.String(), p.propType, p.version)
	}

	return RawPropertySet[T]{
		propType:      subProps.propType,
		version:       subProps.version,
		props:         subProps.props,
		subProps:      subProps.subProps,
		propValues:    map[Name]any{},
		subPropValues: map[Name][]map[Name]any{},
	}, nil
}

// Add stores the given value for the named property, performing type validation.
// If the property name is a property with sub-properties, the value is expected to be a RawPropertySet[T].
// If it receives a property name that is a sub-property key, it validates that the value must be non-empty (The whole sub-property object must be defined).
func (p *RawPropertySet[T]) Add(key Name, val any) error {
	validateProperty := func(key Name, val any) error {
		switch key {
		case InstanceName:
			strVal, ok := val.(string)
			if !ok {
				return fmt.Errorf("Cannot convert %q property %v to string", key.String(), val)
			}

			// An instance name can only contain alphanumeric and hyphen characters.
			nonalpha := regexp.MustCompile(`[^\-a-zA-Z0-9]+`)
			parsedVal := nonalpha.ReplaceAllString(strVal, "")

			if strVal != parsedVal {
				return fmt.Errorf("%q property %q must only contain alphanumeric or hyphen characters", key.String(), strVal)
			}

		case InstanceUUID:
			_, ok := val.(uuid.UUID)
			if !ok {
				return fmt.Errorf("Cannot convert %q property %v to uuid", key.String(), val)
			}

		case InstanceArchitecture:
			str, ok := val.(string)
			if !ok {
				return fmt.Errorf("Cannot convert %q property %v to string", key.String(), val)
			}

			_, err := osarch.ArchitectureID(str)
			if err != nil {
				return err
			}

		case InstanceDiskCapacity:
			flt, ok := val.(int64)
			if !ok {
				return fmt.Errorf("Cannot convert %q property %v to number", key.String(), val)
			}

			if flt <= 0 {
				return fmt.Errorf("Invalid disk capacity %d", flt)
			}

		case InstanceConfig:
			_, ok := val.(map[string]string)
			if !ok {
				return fmt.Errorf("Cannot convert %q property %v to map", key.String(), val)
			}

		case InstanceDiskName:
			fallthrough
		case InstanceNICHardwareAddress:
			fallthrough
		case InstanceNICNetwork:
			fallthrough
		case InstanceNICNetworkID:
			fallthrough
		case InstanceSnapshotName:
			str, ok := val.(string)
			if !ok {
				return fmt.Errorf("Cannot convert %q property %v to string", key.String(), val)
			}

			if str == "" {
				return fmt.Errorf("Property %q value is empty", key.String())
			}
		}

		return nil
	}

	if HasSubProperties(key) {
		switch t := val.(type) {
		case RawPropertySet[T]:
			for k, v := range t.propValues {
				err := validateProperty(k, v)
				if err != nil {
					return err
				}
			}

			propValues := make(map[Name]any, len(t.propValues))
			for k, v := range t.propValues {
				propValues[k] = v
			}

			p.subPropValues[key] = append(p.subPropValues[key], propValues)

			// Reset the property values for the sub-property object after the parent inherits them.
			t.propValues = map[Name]any{}
		default:
			return fmt.Errorf("Expected a map of properties for the device %q. Got %v", key.String(), val)
		}
	} else {
		err := validateProperty(key, val)
		if err != nil {
			return err
		}

		p.propValues[key] = val
	}

	return nil
}

// ToAPI converts the raw properties list to an API compatible type.
// Since we already validated the inputs when calling Add, this just remarshals the value maps as the API type.
func (p RawPropertySet[T]) ToAPI() (*api.InstanceProperties, error) {
	data := map[string]any{}

	for k, v := range p.propValues {
		data[k.String()] = v
	}

	for k, v := range p.subPropValues {
		strMap := make([]map[string]any, len(v))
		for i, valMap := range v {
			strMap[i] = make(map[string]any, len(valMap))
			for k, v := range valMap {
				strMap[i][k.String()] = v
			}
		}

		data[k.String()] = strMap
	}

	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var detectedProperties api.InstanceProperties
	err = json.Unmarshal(b, &detectedProperties)
	if err != nil {
		return nil, err
	}

	if detectedProperties.Config == nil {
		detectedProperties.Config = map[string]string{}
	}

	return &detectedProperties, nil
}
