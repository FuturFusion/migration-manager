package agent

import (
	"encoding/json"
	"os"

	"gopkg.in/yaml.v3"
)

// Helper method to instantiate an AgentConfig from a json file.
func AgentConfigFromJsonFile(filepath string) (AgentConfig, error) {
	ret := AgentConfig{}

	contents, err := os.ReadFile(filepath)
	if err != nil {
		return ret, err
	}

	err = json.Unmarshal(contents, &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

// Helper method to instantiate an AgentConfig from a yaml file.
func AgentConfigFromYamlFile(filepath string) (AgentConfig, error) {
	ret := AgentConfig{}

	contents, err := os.ReadFile(filepath)
	if err != nil {
		return ret, err
	}

	err = yaml.Unmarshal(contents, &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

// Helper method to dump an AgentConfig to a json file.
func AgentConfigToJsonFile(config AgentConfig, filepath string) error {
	contents, err := json.Marshal(config)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath, contents, 0644)
	if err != nil {
		return err
	}

	return nil
}

// Helper method to dump an AgentConfig to a yaml file.
func AgentConfigToYamlFile(config AgentConfig, filepath string) error {
	contents, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath, contents, 0644)
	if err != nil {
		return err
	}

	return nil
}
