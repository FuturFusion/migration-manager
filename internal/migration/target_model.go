package migration

import (
	"encoding/json"
	"net/url"

	"github.com/FuturFusion/migration-manager/shared/api"
)

type Target struct {
	ID         int
	Name       string
	TargetType api.TargetType

	Properties json.RawMessage
}

func (t Target) Validate() error {
	if t.ID < 0 {
		return NewValidationErrf("Invalid target, id can not be negative")
	}

	if t.Name == "" {
		return NewValidationErrf("Invalid target, name can not be empty")
	}

	if t.TargetType < api.TARGETTYPE_INCUS || t.TargetType > api.TARGETTYPE_INCUS {
		return NewValidationErrf("Invalid target, %d is not a valid target type", t.TargetType)
	}

	if t.Properties == nil {
		return NewValidationErrf("Invalid target, properties can not be null")
	}

	var err error
	switch t.TargetType {
	case api.TARGETTYPE_INCUS:
		err = t.validateTargetTypeIncus()
	}

	if err != nil {
		return err
	}

	return nil
}

func (t Target) validateTargetTypeIncus() error {
	var properties api.IncusProperties

	err := json.Unmarshal(t.Properties, &properties)
	if err != nil {
		return NewValidationErrf("Invalid properties for Incus type: %v", err)
	}

	_, err = url.Parse(properties.Endpoint)
	if err != nil {
		return NewValidationErrf("Invalid target, endpoint %q is not a valid URL: %v", properties.Endpoint, err)
	}

	return nil
}

type Targets []Target
