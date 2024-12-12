package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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
	idleSleep  time.Duration
}

type WorkerOption func(*Worker) error

func NewWorker(endpoint string, uuid string, opts ...WorkerOption) (*Worker, error) {
	// Parse the provided URL for the migration manager endpoint.
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	wrkr := &Worker{
		endpoint:   parsedURL,
		source:     nil,
		uuid:       uuid,
		lastUpdate: time.Now().UTC(),
		idleSleep:  10 * time.Second,
	}

	for _, opt := range opts {
		err := opt(wrkr)
		if err != nil {
			return nil, err
		}
	}

	// Do a quick connectivity check to the endpoint.
	_, err = wrkr.doHTTPRequestV1("", http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	return wrkr, nil
}

func WithIdleSleep(sleep time.Duration) WorkerOption {
	return func(w *Worker) error {
		w.idleSleep = sleep
		return nil
	}
}

func WithSource(src source.Source) WorkerOption {
	return func(w *Worker) error {
		w.source = src
		return nil
	}
}

func (w *Worker) Run(ctx context.Context) {
	logger.Info("Starting up", logger.Ctx{"version": version.Version})

	for {
		done := func() (done bool) {
			resp, err := w.doHTTPRequestV1("/queue/"+w.uuid, http.MethodGet, nil)
			if err != nil {
				logger.Errorf("%s", err.Error())
				return false
			}

			cmd, err := parseReturnedCommand(resp.Metadata)
			if err != nil {
				logger.Errorf("%s", err.Error())
				return false
			}

			switch cmd.Command {
			case api.WORKERCOMMAND_IDLE:
				logger.Debug("Received IDLE command, sleeping")
				return false

			case api.WORKERCOMMAND_IMPORT_DISKS:
				w.importDisks(ctx, cmd)
				return false

			case api.WORKERCOMMAND_FINALIZE_IMPORT:
				return w.finalizeImport(ctx, cmd)

			default:
				logger.Errorf("Received unknown command (%d)", cmd.Command)
				return false
			}
		}()
		if done {
			return
		}

		t := time.NewTimer(w.idleSleep)

		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			t.Stop()
		}
	}
}

func (w *Worker) importDisks(ctx context.Context, cmd api.WorkerCommand) {
	if w.source == nil {
		err := w.connectSource(ctx, cmd.SourceType, cmd.Source)
		if err != nil {
			w.sendErrorResponse(err)
			return
		}
	}

	logger.Info("Performing disk import")

	err := w.importDisksHelper(ctx, cmd)
	if err != nil {
		w.sendErrorResponse(err)
		return
	}

	logger.Info("Disk import completed successfully")
	w.sendStatusResponse(api.WORKERRESPONSE_SUCCESS, "Disk import completed successfully")
}

func (w *Worker) importDisksHelper(ctx context.Context, cmd api.WorkerCommand) error {
	// Delete any existing migration snapshot that might be left over.
	err := w.source.DeleteVMSnapshot(ctx, cmd.InventoryPath, internal.IncusSnapshotName)
	if err != nil {
		return err
	}

	// Do the actual import.
	return w.source.ImportDisks(ctx, cmd.InventoryPath, func(status string, isImportant bool) {
		logger.Info(status)

		// Only send updates back to the server if important or once every 5 seconds.
		if isImportant || time.Since(w.lastUpdate).Seconds() >= 5 {
			w.lastUpdate = time.Now().UTC()
			w.sendStatusResponse(api.WORKERRESPONSE_RUNNING, status)
		}
	})
}

// finalizeImport performs some finalizing tasks. If everything is executed
// successfully, this function returns true, signaling the everything is done
// and migration-manager-worker can shut down.
// If there is an error or not all the finalizing work has been performed
// yet, false is returned.
func (w *Worker) finalizeImport(ctx context.Context, cmd api.WorkerCommand) (done bool) {
	if w.source == nil {
		err := w.connectSource(ctx, cmd.SourceType, cmd.Source)
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}
	}

	logger.Info("Shutting down source VM")

	err := w.source.PowerOffVM(ctx, cmd.InventoryPath)
	if err != nil {
		w.sendErrorResponse(err)
		return false
	}

	logger.Info("Source VM shutdown complete")

	logger.Info("Performing final disk sync")

	err = w.importDisksHelper(ctx, cmd)
	if err != nil {
		w.sendErrorResponse(err)
		return false
	}

	logger.Info("Performing final migration tasks")
	w.sendStatusResponse(api.WORKERRESPONSE_RUNNING, "Performing final migration tasks")

	// Windows-specific
	if strings.Contains(strings.ToLower(cmd.OS), "windows") {
		winVer, err := worker.MapWindowsVersionToAbbrev(cmd.OSVersion)
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}

		err = worker.WindowsInjectDrivers(ctx, winVer, "/dev/sda3", "/dev/sda4") // FIXME -- values are hardcoded
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}
	}

	// Linux-specific

	// Get the disto's major version, if possible.
	majorVersion := -1
	// VMware API doesn't distinguish openSUSE and Ubuntu versions.
	if !strings.Contains(strings.ToLower(cmd.OS), "opensuse") && !strings.Contains(strings.ToLower(cmd.OS), "ubuntu") {
		majorVersionRegex := regexp.MustCompile(`^\w+?(\d+)(_64)?$`)
		matches := majorVersionRegex.FindStringSubmatch(cmd.OS)
		if len(matches) > 1 {
			majorVersion, _ = strconv.Atoi(majorVersionRegex.FindStringSubmatch(cmd.OS)[1])
		}
	}

	if strings.Contains(strings.ToLower(cmd.OS), "centos") {
		err = worker.LinuxDoPostMigrationConfig("CentOS", majorVersion)
	} else if strings.Contains(strings.ToLower(cmd.OS), "debian") {
		err = worker.LinuxDoPostMigrationConfig("Debian", majorVersion)
	} else if strings.Contains(strings.ToLower(cmd.OS), "opensuse") {
		err = worker.LinuxDoPostMigrationConfig("openSUSE", majorVersion)
	} else if strings.Contains(strings.ToLower(cmd.OS), "oracle") {
		err = worker.LinuxDoPostMigrationConfig("Oracle", majorVersion)
	} else if strings.Contains(strings.ToLower(cmd.OS), "rhel") {
		err = worker.LinuxDoPostMigrationConfig("RHEL", majorVersion)
	} else if strings.Contains(strings.ToLower(cmd.OS), "sles") {
		err = worker.LinuxDoPostMigrationConfig("SUSE", majorVersion)
	} else if strings.Contains(strings.ToLower(cmd.OS), "ubuntu") {
		err = worker.LinuxDoPostMigrationConfig("Ubuntu", majorVersion)
	}

	if err != nil {
		w.sendErrorResponse(err)
		return false
	}

	// When the worker is done, the VM will be forced off, so call sync() to ensure all data is saved to disk.
	unix.Sync()

	logger.Info("Final migration tasks completed successfully")
	w.sendStatusResponse(api.WORKERRESPONSE_SUCCESS, "Final migration tasks completed successfully")

	// When we've finished the import, shutdown the worker.
	return true
}

func (w *Worker) connectSource(ctx context.Context, sourceType api.SourceType, sourceRaw json.RawMessage) error {
	var src source.Source

	switch sourceType {
	case api.SOURCETYPE_VMWARE:
		vmwareSource := api.VMwareSource{}

		err := json.Unmarshal(sourceRaw, &vmwareSource)
		if err != nil {
			return err
		}

		vmwareSrc := source.NewVMwareSource(vmwareSource.Name, vmwareSource.Endpoint, vmwareSource.Username, vmwareSource.Password)
		vmwareSrc.Insecure = vmwareSource.Insecure

		src = vmwareSrc

	default:
		return fmt.Errorf("Provided source type %q is not usable with `migration-manager-worker`", sourceType.String())
	}

	w.source = src

	return w.source.Connect(ctx)
}

func (w *Worker) sendStatusResponse(statusVal api.WorkerResponseType, statusString string) {
	resp := api.WorkerResponse{Status: statusVal, StatusString: statusString}

	content, err := json.Marshal(resp)
	if err != nil {
		logger.Errorf("Failed to marshal status response for migration manager: %s", err.Error())
		return
	}

	_, err = w.doHTTPRequestV1("/queue/"+w.uuid, http.MethodPut, content)
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

	_, err = w.doHTTPRequestV1("/queue/"+w.uuid, http.MethodPut, content)
	if err != nil {
		logger.Errorf("Failed to send error back to migration manager: %s", err.Error())
		return
	}
}

func (w *Worker) doHTTPRequestV1(endpoint string, method string, content []byte) (*incusAPI.ResponseRaw, error) {
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
