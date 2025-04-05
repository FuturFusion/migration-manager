package api

import (
	"encoding/json"
)

type WorkerCommandType int

const (
	WORKERCOMMAND_UNKNOWN WorkerCommandType = iota
	WORKERCOMMAND_IDLE
	WORKERCOMMAND_IMPORT_DISKS
	WORKERCOMMAND_FINALIZE_IMPORT
)

type WorkerResponseType int

const (
	WORKERRESPONSE_UNKNOWN WorkerResponseType = iota
	WORKERRESPONSE_RUNNING
	WORKERRESPONSE_SUCCESS
	WORKERRESPONSE_FAILED
)

// WorkerCommand defines a command sent from the migration manager to a worker.
//
// swagger:model
type WorkerCommand struct {
	// The command for the worker to execute
	// Example: WORKERCOMMAND_IMPORT_DISKS
	Command WorkerCommandType `json:"command" yaml:"command"`

	// Internal path to the instance
	// Example: /SHF/vm/Migration Tests/DebianTest
	Location string `json:"location" yaml:"location"`

	// SourceType declares the type of the worker and is used as a hint to
	// correctly process the details provided in Source.
	SourceType SourceType `json:"sourceType" yaml:"sourceType"`

	// Source for the worker to fetch VM metadata and/or disk from.
	Source json.RawMessage `json:"source" yaml:"source"`

	// The name of the operating system
	// Example: Ubuntu
	OS string `json:"os" yaml:"os"`

	// The version of the operating system
	// Example: 24.04
	OSVersion string `json:"os_version" yaml:"os_version"`
}

// WorkerResponse defines a response received from a worker.
//
// swagger:model
type WorkerResponse struct {
	// The status of the command the work is/was executing.
	// Example: WORKERRESPONSE_RUNNING
	Status WorkerResponseType `json:"status" yaml:"status"`

	// A free-form string to provide additional information about the command status
	// Example: "Migration 25% complete"
	StatusString string `json:"status_string" yaml:"status_string"`
}
