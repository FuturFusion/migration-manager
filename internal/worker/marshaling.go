package worker

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/source"
)

// Implement the encoding/json Marshaler interface.
func (a WorkerConfig) MarshalJSON() ([]byte, error) {
	// Determine the source's type.
	sourceType := source.SOURCETYPE_UNKNOWN
	switch a.Source.(type) {
	case *source.InternalCommonSource:
		sourceType = source.SOURCETYPE_COMMON
	case *source.InternalVMwareSource:
		sourceType = source.SOURCETYPE_VMWARE
	default:
		return nil, fmt.Errorf("Unsupported source type %T", a.Source)
	}

	// Marshal into a json object.
	type WorkerConfigWrapper WorkerConfig
	return json.Marshal(&struct {
		TYPE int `json:"TYPE"`
		WorkerConfigWrapper
	}{
		TYPE: sourceType,
		WorkerConfigWrapper: (WorkerConfigWrapper)(a),
	})
}

// Implement the encoding/json Unmarshaler interface.
func (a *WorkerConfig) UnmarshalJSON(data []byte) error {
	// Unmarshal the data into a map so we can figure out what the source type is.
	unmarshaledData := make(map[string]interface{})
	err := json.Unmarshal(data, &unmarshaledData)
	if err != nil {
		return err
	}
	_, typeExists := unmarshaledData["TYPE"]
	if !typeExists {
		return fmt.Errorf("TYPE field is not present")
	}

	// Set a correct Source for the WorkerConfig based on the type.
	newWorkerConfig := new(WorkerConfig)
	switch unmarshaledData["TYPE"].(float64) {
	case source.SOURCETYPE_COMMON:
		newWorkerConfig.Source = &source.InternalCommonSource{}
	case source.SOURCETYPE_VMWARE:
		newWorkerConfig.Source = &source.InternalVMwareSource{}
	default:
		return fmt.Errorf("Unsupported source type %d", unmarshaledData["TYPE"])
	}

	// Unmarshal the json object into an WorkerConfig.
	type WorkerConfigWrapper WorkerConfig
	aux := &struct {
		TYPE int `json:"TYPE"`
		*WorkerConfigWrapper
	}{
		WorkerConfigWrapper: (*WorkerConfigWrapper)(newWorkerConfig),
	}
	err = json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}

	*a = *newWorkerConfig
	return nil
}

// Implement the gopkg.in/yaml.v3 Marshaler interface.
func (a WorkerConfig) MarshalYAML() (interface{}, error) {
	// Determine the source's type.
	sourceType := source.SOURCETYPE_UNKNOWN
	switch a.Source.(type) {
	case *source.InternalCommonSource:
		sourceType = source.SOURCETYPE_COMMON
	case *source.InternalVMwareSource:
		sourceType = source.SOURCETYPE_VMWARE
	default:
		return nil, fmt.Errorf("Unsupported source type %T", a.Source)
	}

	// Marshal into a yaml document.
	type WorkerConfigWrapper WorkerConfig
	val, err := yaml.Marshal(&struct {
		TYPE int `yaml:"TYPE"`
		WorkerConfigWrapper `yaml:",inline"`
	}{
		TYPE: sourceType,
		WorkerConfigWrapper: (WorkerConfigWrapper)(a),
	})

	if err != nil {
		return nil, err
	}

	return string(val), nil
}

// Implement the gopkg.in/yaml.v3 Unmarshaler interface.
func (a *WorkerConfig) UnmarshalYAML(value *yaml.Node) error {
	// Unmarshal the data into a map so we can figure out what the source type is.
	unmarshaledData := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(value.Value), &unmarshaledData)
	if err != nil {
		return err
	}
	_, typeExists := unmarshaledData["TYPE"]
	if !typeExists {
		return fmt.Errorf("TYPE field is not present")
	}

	// Ugh, need to do this by hand manually...
	sourceVals, ok := unmarshaledData["source"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Error extracting source")
	}

	newWorkerConfig := new(WorkerConfig)
	newWorkerConfig.MigrationManagerEndpoint, _ = unmarshaledData["migrationManagerEndpoint"].(string)
	newWorkerConfig.VMName, _ = unmarshaledData["vmName"].(string)
	newWorkerConfig.VMOperatingSystemName, _ = unmarshaledData["vmOperatingSystemName"].(string)
	newWorkerConfig.VMOperatingSystemVersion, _ = unmarshaledData["vmOperatingSystemVersion"].(string)

	switch unmarshaledData["TYPE"] {
	case source.SOURCETYPE_COMMON:
		s := &source.InternalCommonSource{}
		s.Name, _ = sourceVals["name"].(string)
		s.DatabaseID, _ = sourceVals["databaseID"].(int)
		newWorkerConfig.Source = s
	case source.SOURCETYPE_VMWARE:
		s := &source.InternalVMwareSource{}
		s.Name, _ = sourceVals["name"].(string)
		s.DatabaseID, _ = sourceVals["databaseID"].(int)
		s.Endpoint, _ = sourceVals["endpoint"].(string)
		s.Username, _ = sourceVals["username"].(string)
		s.Password, _ = sourceVals["password"].(string)
		s.Insecure, _ = sourceVals["insecure"].(bool)
		newWorkerConfig.Source = s
	default:
		return fmt.Errorf("Unsupported source type %d", unmarshaledData["TYPE"])
	}

	*a = *newWorkerConfig
	return nil
}
