package api

type WorkerCommandType int
const (
	WORKERCOMMAND_UNKNOWN = iota
	WORKERCOMMAND_IDLE
	WORKERCOMMAND_IMPORT_DISKS
	WORKERCOMMAND_FINALIZE_IMPORT
)

type WorkerResponseType int
const (
	WORKERRESPONSE_UNKNOWN = iota
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

	// The name of the instance
	// Example: UbuntuServer
	Name string `json:"name" yaml:"name"`

	// Source for the worker to fetch VM metadata and/or disk from
	// Example: VMwareSource{...}
	Source VMwareSource `json:"source" yaml:"source"`

	// The name of the operating system
	// Example: Ubuntu
	OS string `json:"os" yaml:"os"`

	// The version of the operating system
	// Example: 24.04
	OSVersion string `json:"osVersion" yaml:"osVersion"`
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
	StatusString string `json:"statusString" yaml:"statusString"`
}
