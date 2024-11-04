package worker

import (
	"encoding/json"
	"os"

	"gopkg.in/yaml.v3"
)

// Helper method to instantiate an WorkerConfig from a json file.
func WorkerConfigFromJsonFile(filepath string) (WorkerConfig, error) {
	ret := WorkerConfig{}

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

// Helper method to instantiate an WorkerConfig from a yaml file.
func WorkerConfigFromYamlFile(filepath string) (WorkerConfig, error) {
	ret := WorkerConfig{}

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

// Helper method to dump an WorkerConfig to a json file.
func WorkerConfigToJsonFile(config WorkerConfig, filepath string) error {
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

// Helper method to dump an WorkerConfig to a yaml file.
func WorkerConfigToYamlFile(config WorkerConfig, filepath string) error {
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
