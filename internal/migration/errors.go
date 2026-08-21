package migration

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound              = errors.New("Not found")
	ErrConstraintViolation   = errors.New("Constraint violation")
	ErrOperationNotPermitted = errors.New("Operation not permitted")
)

type ErrValidation string

func NewValidationErrf(format string, a ...any) error {
	return ErrValidation(fmt.Sprintf(format, a...))
}

func (e ErrValidation) Error() string {
	return string(e)
}

// ErrDisabled is returned when an instance is disabled.
type ErrDisabled struct {
	err    error
	reason DisabledReason
}

// Error implements error.Error.
func (e ErrDisabled) Error() string {
	return e.err.Error()
}

// Unwrap returns the wrapped error, so errors.Is and errors.As can inspect it.
func (e ErrDisabled) Unwrap() error {
	return e.err
}

// Reason returns the underlying reason for the error.
func (e ErrDisabled) Reason() DisabledReason {
	return e.reason
}

// NewDisabledErrf creates a new error using the specified reason and format.
func NewDisabledErrf(reason DisabledReason, format string, a ...any) error {
	return ErrDisabled{
		err:    fmt.Errorf(format, a...),
		reason: reason,
	}
}

// DisabledReason is the reason behind the error for a disabled instance.
type DisabledReason string

const (
	DISABLEDREASON_MANUALLY_DISABLED             DisabledReason = "Manually disabled"
	DISABLEDREASON_INVALID_HOSTNAME              DisabledReason = "Invalid hostname"
	DISABLEDREASON_UNKNOWN_OS                    DisabledReason = "Unknown OS"
	DISABLEDREASON_UNKNOWN_ARCHITECTURE          DisabledReason = "Unknown architecture"
	DISABLEDREASON_UNKNOWN_IP_ADDRESS            DisabledReason = "Unknown IP address"
	DISABLEDREASON_VERIFYING_BACKGROUND_IMPORT   DisabledReason = "Verifying background import"
	DISABLEDREASON_UNSUPPORTED_BACKGROUND_IMPORT DisabledReason = "Unsupported background import"
	DISABLEDREASON_UNSUPPORTED_DISK_SNAPSHOT     DisabledReason = "Unsupported disk snapshot"
)
