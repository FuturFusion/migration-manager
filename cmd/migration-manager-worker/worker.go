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

	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/version"
	"github.com/FuturFusion/migration-manager/internal/worker"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Worker struct {
	endpoint       *url.URL
	source         source.Source
	uuid           string

	lastUpdate     time.Time

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

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	ret := &Worker{
		endpoint: parsedUrl,
		source: nil,
		uuid: uuid,
		lastUpdate: time.Now().UTC(),
		shutdownCtx: shutdownCtx,
		shutdownCancel: shutdownCancel,
		shutdownDoneCh: make(chan error),
	}

	// Do a quick connectivity check to the endpoint.
	_, err = ret.doHttpRequest("/1.0", http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (w *Worker) Start() error {
	logger.Info("Starting up", logger.Ctx{"version": version.Version})

	go func() {
		for {
			resp, err := w.doHttpRequest("/1.0/queue/" + w.uuid, http.MethodGet, nil)
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

func (w *Worker) Stop(ctx context.Context, sig os.Signal) error {
	logger.Info("Worker stopped")

	return nil
}

func (w *Worker) importDisks(cmd api.WorkerCommand) {
	if w.source == nil {
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
	err = w.source.ImportDisks(w.shutdownCtx, cmd.Name, func(disk string, percentage float64) {
		// Don't send updates more than once every 30 seconds.
		if time.Since(w.lastUpdate).Seconds() < 30 {
			return
		}

		w.lastUpdate = time.Now().UTC()

		status := fmt.Sprintf("Importing disk '%s': %02.2f%% complete", disk, percentage)
		logger.Info(status)
		w.sendStatusResponse(api.WORKERRESPONSE_RUNNING, status)
	})
	if err != nil {
		return err
	}

	return nil
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
	if strings.Contains(strings.ToLower(cmd.OS), "windows") {
		err := worker.WindowsInjectDrivers(w.shutdownCtx, "w11", "/dev/sda3", "/dev/sda4") // FIXME -- values are hardcoded
		if err != nil {
			w.sendErrorResponse(err)
			return
		}
	}

	// Linux-specific
	if strings.Contains(strings.ToLower(cmd.OS), "debian") {
		err := worker.LinuxDoPostMigrationConfig("Debian", "/dev/sda1") // FIXME -- value is hardcoded
		if err != nil {
			w.sendErrorResponse(err)
			return
		}
	} else if strings.Contains(strings.ToLower(cmd.OS), "ubuntu") {
		err := worker.LinuxDoPostMigrationConfig("Ubuntu", "/dev/sda1") // FIXME -- value is hardcoded
		if err != nil {
			w.sendErrorResponse(err)
			return
		}
	}

	logger.Info("Final migration tasks completed successfully")
	w.sendStatusResponse(api.WORKERRESPONSE_SUCCESS, "Final migration tasks completed successfully")

	// When we've finished the import, shutdown the worker.
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

	_, err = w.doHttpRequest("/1.0/queue/" + w.uuid, http.MethodPut, content)
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

	_, err = w.doHttpRequest("/1.0/queue/" + w.uuid, http.MethodPut, content)
	if err != nil {
		logger.Errorf("Failed to send error back to migration manager: %s", err.Error())
		return
	}
}

func (w *Worker) doHttpRequest(path string, method string, content []byte) (*api.ResponseRaw, error) {
	w.endpoint.Path = path
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

	var jsonResp api.ResponseRaw
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

	var ret = api.WorkerCommand{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return api.WorkerCommand{}, err
	}

	return ret, nil
}
