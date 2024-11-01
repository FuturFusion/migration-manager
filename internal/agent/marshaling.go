package agent

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/source"
)

// Implement the encoding/json Marshaler interface.
func (a AgentConfig) MarshalJSON() ([]byte, error) {
	// Determine the source's type.
	sourceType := ""
	switch a.Source.(type) {
	case *source.CommonSource:
		sourceType = "Common"
	case *source.VMwareSource:
		sourceType = "VMware"
	default:
		return nil, fmt.Errorf("Unsupported source type %T", a.Source)
	}

	// Marshal into a json object.
	type AgentConfigWrapper AgentConfig
	return json.Marshal(&struct {
		TYPE string `json:"TYPE"`
		AgentConfigWrapper
	}{
		TYPE: sourceType,
		AgentConfigWrapper: (AgentConfigWrapper)(a),
	})
}

// Implement the encoding/json Unmarshaler interface.
func (a *AgentConfig) UnmarshalJSON(data []byte) error {
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

	// Set a correct Source for the AgentConfig based on the type.
	newAgentConfig := new(AgentConfig)
	switch unmarshaledData["TYPE"] {
	case "Common":
		newAgentConfig.Source = &source.CommonSource{}
	case "VMware":
		newAgentConfig.Source = &source.VMwareSource{}
	default:
		return fmt.Errorf("Unsupported source type %s", unmarshaledData["TYPE"])
	}

	// Unmarshal the json object into an AgentConfig.
	type AgentConfigWrapper AgentConfig
	aux := &struct {
		TYPE string `json:"TYPE"`
		*AgentConfigWrapper
	}{
		AgentConfigWrapper: (*AgentConfigWrapper)(newAgentConfig),
	}
	err = json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}

	*a = *newAgentConfig
	return nil
}

// Implement the gopkg.in/yaml.v3 Marshaler interface.
func (a AgentConfig) MarshalYAML() (interface{}, error) {
	// Determine the source's type.
	sourceType := ""
	switch a.Source.(type) {
	case *source.CommonSource:
		sourceType = "Common"
	case *source.VMwareSource:
		sourceType = "VMware"
	default:
		return nil, fmt.Errorf("Unsupported source type %T", a.Source)
	}

	// Marshal into a yaml document.
	type AgentConfigWrapper AgentConfig
	val, err := yaml.Marshal(&struct {
		TYPE string `yaml:"TYPE"`
		AgentConfigWrapper
	}{
		TYPE: sourceType,
		AgentConfigWrapper: (AgentConfigWrapper)(a),
	})

	if err != nil {
		return nil, err
	}

	return string(val), nil
}

// Implement the gopkg.in/yaml.v3 Unmarshaler interface.
func (a *AgentConfig) UnmarshalYAML(value *yaml.Node) error {
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
	configVals, ok := unmarshaledData["agentconfigwrapper"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Error extracting agentconfigwrapper")
	}

	sourceVals, ok := configVals["source"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Error extracting source")
	}

	newAgentConfig := new(AgentConfig)
	newAgentConfig.MigrationManagerEndpoint, _ = configVals["migrationManagerEndpoint"].(string)
	newAgentConfig.VMName, _ = configVals["vmName"].(string)
	newAgentConfig.VMOperatingSystemName, _ = configVals["vmOperatingSystemName"].(string)
	newAgentConfig.VMOperatingSystemVersion, _ = configVals["vmOperatingSystemVersion"].(string)

	if unmarshaledData["TYPE"] == "Common" {
		s := &source.CommonSource{}
		s.Name, _ = sourceVals["name"].(string)
		newAgentConfig.Source = s
	} else if unmarshaledData["TYPE"] == "VMware" {
		commonVals, _ := sourceVals["commonsource"].(map[string]interface{})
		s := &source.VMwareSource{}
		s.Name, _ = commonVals["name"].(string)
		s.Endpoint, _ = sourceVals["endpoint"].(string)
		s.Username, _ = sourceVals["username"].(string)
		s.Password, _ = sourceVals["password"].(string)
		s.Insecure, _ = sourceVals["insecure"].(bool)
		newAgentConfig.Source = s
	} else {
		return fmt.Errorf("Unsupported source type %s", unmarshaledData["TYPE"])
	}

	*a = *newAgentConfig
	return nil
}
