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
	WORKERCOMMAND_POST_IMPORT
)

type WorkerResponseType int

const (
	WORKERRESPONSE_UNKNOWN WorkerResponseType = iota
	WORKERRESPONSE_RUNNING
	WORKERRESPONSE_SUCCESS
	WORKERRESPONSE_FAILED
)

// WorkerCommand defines a command sent from the migration manager to a worker.
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

	// Operating system type
	// Example: linux
	OSType OSType `json:"os_type" yaml:"os_type"`

	// Architecture of the instance
	// Example: x86_64
	Architecture string `json:"architecture" yaml:"architecture"`
}

// WorkerResponse defines a response received from a worker.
type WorkerResponse struct {
	// The status of the command the work is/was executing.
	// Example: WORKERRESPONSE_RUNNING
	Status WorkerResponseType `json:"status" yaml:"status"`

	// A free-form string to provide additional information about the command status
	// Example: "Migration 25% complete"
	StatusMessage string `json:"status_message" yaml:"status_message"`
}
