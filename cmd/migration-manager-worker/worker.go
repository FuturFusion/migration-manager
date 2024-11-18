package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal/version"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Worker struct {
	endpoint       *url.URL
	uuid           string

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
		uuid: uuid,
		shutdownCtx: shutdownCtx,
		shutdownCancel: shutdownCancel,
		shutdownDoneCh: make(chan error),
	}

	// Do a quick connectivity check to the endpoint.
	_, err = ret.doHttpRequest("/1.0")
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (w *Worker) Start() error {
	logger.Info("Starting up", logger.Ctx{"version": version.Version})

	go func() {
		for {
			resp, err := w.doHttpRequest("/1.0/queue/" + w.uuid)
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
	logger.Info("Doing disk import!")
}

func (w *Worker) finalizeImport(cmd api.WorkerCommand) {
	logger.Warn("Finalize not yet implemented")
}

func (w *Worker) doHttpRequest(path string) (*api.ResponseRaw, error) {
	w.endpoint.Path = path
	req, err := http.NewRequest(http.MethodGet, w.endpoint.String(), nil)
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
