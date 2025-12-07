package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Event represents a Migration Manager event.
type Event struct {
	Time     time.Time       `json:"time"`
	Type     LogScope        `json:"type"`
	Metadata json.RawMessage `json:"metadata"`
}

// EventLogging represents a logging type event.
type EventLogging struct {
	Message string            `yaml:"message" json:"message"`
	Level   string            `yaml:"level" json:"level"`
	Context map[string]string `yaml:"context" json:"context"`
}

// EventLifecycle represents a lifecycle type event.
type EventLifecycle struct {
	Action   string   `yaml:"action" json:"action"`
	Entities []string `yaml:"entities" json:"entities"`

	Requestor *EventLifecycleRequestor `yaml:"requestor,omitempty" json:"requestor,omitempty"`

	Metadata json.RawMessage `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// EventLifecycleRequestor represents the initial requestor for an event.
type EventLifecycleRequestor struct {
	Username string `yaml:"username" json:"username"`
	Protocol string `yaml:"protocol" json:"protocol"`
	Address  string `yaml:"address" json:"address"`
}

func (e EventLifecycleRequestor) MarshalText() ([]byte, error) {
	return []byte(e.Protocol + "/" + e.Username + " (" + e.Address + ")"), nil
}

func (e *EventLifecycleRequestor) UnmarshalText(b []byte) error {
	r := EventLifecycleRequestor{}

	before, after, ok := strings.Cut(string(b), " ")
	if !ok || before == "" || after == "" {
		return fmt.Errorf("Invalid requestor format %q", string(b))
	}

	r.Protocol, r.Username, ok = strings.Cut(before, "/")
	if !ok || r.Protocol == "" || r.Username == "" {
		return fmt.Errorf("Invalid requestor format %q", string(b))
	}

	r.Address, _ = strings.CutPrefix(after, "(")
	r.Address, ok = strings.CutSuffix(r.Address, ")")
	if !ok || r.Address == "" {
		return fmt.Errorf("Invalid requestor format %q", string(b))
	}

	*e = r

	return nil
}

type LifecycleAction string
