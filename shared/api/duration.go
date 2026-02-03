package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// AsDuration returns an api.Duration from the time.Duration.
func AsDuration(t time.Duration) Duration {
	return Duration{Duration: t}
}

// ParseDuration returns an api.Duration from the time.Duration.
func ParseDuration(durStr string) (Duration, error) {
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return Duration{}, err
	}

	if dur < 0 {
		return Duration{}, fmt.Errorf("Duration cannot be negative: %d", dur)
	}

	return Duration{Duration: dur}, nil
}

// Duration is a wrapper around time.Duration for easy json parsing.
type Duration struct {
	time.Duration
}

// UnmarshalYAML is a YAML unmarshaler for api.Duration.
func (d *Duration) UnmarshalYAML(unmarshal func(v any) error) error {
	var durStr string
	err := unmarshal(&durStr)
	if err != nil {
		return err
	}

	dur := Duration{}
	if durStr != "" {
		dur, err = ParseDuration(durStr)
		if err != nil {
			return err
		}
	}

	*d = dur

	return nil
}

// MarshalYAML is a YAML marshaler for api.Duration.
func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

// UnmarshalJSON is a JSON unmarshaler for api.Duration.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var durStr string
	err := json.Unmarshal(b, &durStr)
	if err != nil {
		return err
	}

	dur := Duration{}
	if durStr != "" {
		dur, err = ParseDuration(durStr)
		if err != nil {
			return err
		}
	}

	*d = dur

	return nil
}

// MarshalJSON is a JSON marshaler for api.Duration.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// String returns the api.Duration as a string.
func (d Duration) String() string {
	return d.Duration.String()
}
