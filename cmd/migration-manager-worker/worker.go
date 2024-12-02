package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/logger"
	"golang.org/x/sys/unix"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/version"
	"github.com/FuturFusion/migration-manager/internal/worker"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Worker struct {
	endpoint *url.URL
	source   source.Source
	uuid     string

	lastUpdate time.Time

	// REVIEW: I would not call this context "shutdown", since it is not related
	// to shutdown but rather to the normal execution of the application.
	// If it is cancelled, the shutdown procedure is triggered.
	// Suggestion: applicationCtx or just appCtx.
	shutdownCtx    context.Context    // Canceled when shutdown starts.
	shutdownCancel context.CancelFunc // Cancels the shutdownCtx to indicate shutdown starting.
	shutdownDoneCh chan error         // Receives the result of the w.Stop() function and tells the daemon to end.
}

func newWorker(endpoint string, uuid string) (*Worker, error) {
	// Parse the provided URL for the migration manager endpoint.
	parsedUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	// REVIEW: I would accept the context from the caller instead of creating
	// a new detached context here.
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	ret := &Worker{
		endpoint:       parsedUrl,
		source:         nil,
		uuid:           uuid,
		lastUpdate:     time.Now().UTC(),
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		shutdownDoneCh: make(chan error),
	}

	// Do a quick connectivity check to the endpoint.
	// REVIEW: This causes the migration-manager-worker to end it self, if the
	// connectivity check (temporarily) fails at startup time. Is this intentional?
	// I would consider some retry logic here, which does log, if the connection
	// could not be statblished, but keeps trying until successful.
	_, err = ret.doHttpRequestV1("/", http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (w *Worker) Start() error {
	logger.Info("Starting up", logger.Ctx{"version": version.Version})

	// REVIEW: Having the concurrency inside of the worker manager does make it
	// harder to test this code. I would put the starting of the Go routine to
	// the worker_main (cmdWorker.Run).
	go func() {
		for {
			// REVIEW: I would move the "business logic", which queries the API and then
			// performs the necessary tasks based on the response, into a separate function.
			// With this, one can have the happy-path on the left and use early return in the
			// case of an error, which makes the code way more readable.
			// In the endless loop only the call to the business logic, logging of errors
			// and the waiting for the next interval would remain.
			resp, err := w.doHttpRequestV1("/queue/"+w.uuid, http.MethodGet, nil)
			if err != nil {
				logger.Errorf("%s", err.Error())
			} else {
				cmd, err := parseReturnedCommand(resp.Metadata)
				if err != nil {
					logger.Errorf("%s", err.Error())
				} else {
					switch cmd.Command {
					case api.WORKERCOMMAND_IDLE:
						logger.Debug("Received IDLE command, sleeping")
					case api.WORKERCOMMAND_IMPORT_DISKS:
						w.importDisks(cmd)
					case api.WORKERCOMMAND_FINALIZE_IMPORT:
						w.finalizeImport(cmd)
					default:
						logger.Errorf("Received unknown command (%d)", cmd.Command)
					}
				}
			}

			t := time.NewTimer(time.Duration(time.Second * 10))

			select {
			case <-w.shutdownCtx.Done():
				t.Stop()
				return
			case <-t.C:
				t.Stop()
			}
		}
	}()

	return nil
}

// REVIEW: I wonder, if the whole logic would be come simpler, if instead of
// having a `Stop` method, we would just accept the application conntext
// in `newWorker`. Then the signal handler could just cancel the context
// (what is done automatically with signal.NotifyContext) and the worker
// would initiate the shutdown procedure, when ever the context go cancelled.
func (w *Worker) Stop(ctx context.Context, sig os.Signal) error {
	logger.Info("Worker stopped")

	return nil
}

func (w *Worker) importDisks(cmd api.WorkerCommand) {
	if w.source == nil {
		// REVIEW: the usage of the context here is a little bit confusing to me, since
		// the method connectSource is defined on Worker and with this, would have
		// access to w.shutdownCtx it self, but instead, it accepts the context
		// again as argument and then w.shutdownCtx is passed. Is there a
		// special reasoning for this that I don't see?
		err := w.connectSource(w.shutdownCtx, cmd.Source)
		if err != nil {
			w.sendErrorResponse(err)
			return
		}
	}

	logger.Info("Performing disk import")

	err := w.importDisksHelper(cmd)
	if err != nil {
		w.sendErrorResponse(err)
		return
	}

	logger.Info("Disk import completed successfully")
	w.sendStatusResponse(api.WORKERRESPONSE_SUCCESS, "Disk import completed successfully")
}

func (w *Worker) importDisksHelper(cmd api.WorkerCommand) error {
	// Delete any existing migration snapshot that might be left over.
	err := w.source.DeleteVMSnapshot(w.shutdownCtx, cmd.Name, internal.IncusSnapshotName)
	if err != nil {
		return err
	}

	// Do the actual import.
	return w.source.ImportDisks(w.shutdownCtx, cmd.Name, func(status string) {
		logger.Info(status)

		// Don't send updates back to the server more than once every 30 seconds.
		if time.Since(w.lastUpdate).Seconds() < 30 {
			return
		}

		w.lastUpdate = time.Now().UTC()
		w.sendStatusResponse(api.WORKERRESPONSE_RUNNING, status)
	})
}

func (w *Worker) finalizeImport(cmd api.WorkerCommand) {
	if w.source == nil {
		err := w.connectSource(w.shutdownCtx, cmd.Source)
		if err != nil {
			w.sendErrorResponse(err)
			return
		}
	}

	logger.Info("Performing final disk sync")

	err := w.importDisksHelper(cmd)
	if err != nil {
		w.sendErrorResponse(err)
		return
	}

	logger.Info("Performing final migration tasks")
	w.sendStatusResponse(api.WORKERRESPONSE_RUNNING, "Performing final migration tasks")

	// Windows-specific
	// REVIEW: Have you considered to use build tags for the different OS dependent implementation parts?
	if strings.Contains(strings.ToLower(cmd.OS), "windows") {
		winVer, err := worker.MapWindowsVersionToAbbrev(cmd.OSVersion)
		if err != nil {
			w.sendErrorResponse(err)
			return
		}
		err = worker.WindowsInjectDrivers(w.shutdownCtx, winVer, "/dev/sda3", "/dev/sda4") // FIXME -- values are hardcoded
		if err != nil {
			w.sendErrorResponse(err)
			return
		}
	}

	// Linux-specific
	if strings.Contains(strings.ToLower(cmd.OS), "debian") {
		err = worker.LinuxDoPostMigrationConfig("Debian")
	} else if strings.Contains(strings.ToLower(cmd.OS), "ubuntu") {
		err = worker.LinuxDoPostMigrationConfig("Ubuntu")
	}
	if err != nil {
		w.sendErrorResponse(err)
		return
	}

	// When the worker is done, the VM will be forced off, so call sync() to ensure all data is saved to disk.
	// REVIEW: is this also necessary for Windows?
	unix.Sync()

	logger.Info("Final migration tasks completed successfully")
	w.sendStatusResponse(api.WORKERRESPONSE_SUCCESS, "Final migration tasks completed successfully")

	// When we've finished the import, shutdown the worker.
	// REVIEW: this feels pretty rude and does make it hard to actually test this code.
	os.Exit(0)
}

func (w *Worker) connectSource(ctx context.Context, s api.VMwareSource) error {
	w.source = source.NewVMwareSource(s.Name, s.Endpoint, s.Username, s.Password)
	if s.Insecure {
		w.source.SetInsecureTLS(true)
	}

	return w.source.Connect(ctx)
}

func (w *Worker) sendStatusResponse(statusVal api.WorkerResponseType, statusString string) {
	resp := api.WorkerResponse{Status: statusVal, StatusString: statusString}

	content, err := json.Marshal(resp)
	if err != nil {
		logger.Errorf("Failed to send status back to migration manager: %s", err.Error())
		return
	}

	_, err = w.doHttpRequestV1("/queue/"+w.uuid, http.MethodPut, content)
	if err != nil {
		logger.Errorf("Failed to send status back to migration manager: %s", err.Error())
		return
	}
}

func (w *Worker) sendErrorResponse(err error) {
	logger.Errorf("%s", err.Error())
	resp := api.WorkerResponse{Status: api.WORKERRESPONSE_FAILED, StatusString: err.Error()}

	content, err := json.Marshal(resp)
	if err != nil {
		logger.Errorf("Failed to send error back to migration manager: %s", err.Error())
		return
	}

	_, err = w.doHttpRequestV1("/queue/"+w.uuid, http.MethodPut, content)
	if err != nil {
		logger.Errorf("Failed to send error back to migration manager: %s", err.Error())
		return
	}
}

func (w *Worker) doHttpRequestV1(endpoint string, method string, content []byte) (*incusAPI.ResponseRaw, error) {
	var err error
	w.endpoint.Path, err = url.JoinPath("/1.0/", endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, w.endpoint.String(), bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jsonResp incusAPI.ResponseRaw
	err = json.Unmarshal(bodyBytes, &jsonResp)
	if err != nil {
		return nil, err
	} else if jsonResp.Code != 0 {
		return &jsonResp, fmt.Errorf("Received an error from the endpoint: %s", jsonResp.Error)
	}

	return &jsonResp, nil
}

func parseReturnedCommand(c any) (api.WorkerCommand, error) {
	reJsonified, err := json.Marshal(c)
	if err != nil {
		return api.WorkerCommand{}, err
	}

	ret := api.WorkerCommand{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return api.WorkerCommand{}, err
	}

	return ret, nil
}
