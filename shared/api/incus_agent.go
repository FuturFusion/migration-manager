package api

// IncusAgentConfig configures the agent in the migrated guest.
//
// swagger:model
type IncusAgentConfig struct {
	// Feature settings; null preserves guest settings, while an empty object disables all features.
	Features *IncusAgentFeatures `json:"features" yaml:"features"`
}

// IncusAgentFeatures defines the capabilities enabled in the migrated guest agent.
//
// swagger:model
type IncusAgentFeatures struct {
	Exec        bool `json:"exec" yaml:"exec"`
	Files       bool `json:"files" yaml:"files"`
	GuestAPI    bool `json:"guestapi" yaml:"guestapi"`
	Metrics     bool `json:"metrics" yaml:"metrics"`
	Mounts      bool `json:"mounts" yaml:"mounts"`
	PortForward bool `json:"port-forward" yaml:"port-forward"`
	State       bool `json:"state" yaml:"state"`
}

// ResolveIncusAgentConfig selects the instance feature settings before the batch settings.
func ResolveIncusAgentConfig(batch *IncusAgentConfig, instance *IncusAgentConfig) *IncusAgentConfig {
	if instance != nil && instance.Features != nil {
		return instance
	}

	if batch != nil && batch.Features != nil {
		return batch
	}

	return nil
}
