package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager-worker/internal/worker"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/shared/api"
)

const uuid = "ced6491e-b614-11ef-a01b-677fcc190026"

var errGracefulEndOfTest = fmt.Errorf("graceful end of test")

type instanceDetails struct {
	InventoryPath string
	OS            string
	OSVersion     string
}

func TestNewWorker(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	tests := []struct {
		name     string
		endpoint string

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:     "success",
			endpoint: ts.URL,

			assertErr: require.NoError,
		},
		{
			name:     "error - invalid endpoint",
			endpoint: "http://invalid{}endpoint/",

			assertErr: require.Error,
		},
		{
			name:     "error - unreachable endpoint",
			endpoint: "invalid.endpoint.local",

			assertErr: require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := worker.NewWorker(tc.endpoint, uuid, "")
			tc.assertErr(t, err)
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name                       string
		migrationManagerdResponses []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request)
		instanceSpec               instanceDetails
		sourceDeleteVMSnapshotErr  error
		sourceImportDisksErr       error
		sourcePowerOffVMErr        error

		wantWorkerResponses []api.WorkerResponseType
		wantEndOfTestCause  error
	}{
		{
			name: "success - idle",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false),
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},

			wantEndOfTestCause: errGracefulEndOfTest,
		},
		{
			name: "success - import disks",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				workerCommandResponse(api.WORKERCOMMAND_IMPORT_DISKS, false),
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},

			wantWorkerResponses: []api.WorkerResponseType{
				api.WORKERRESPONSE_SUCCESS,
			},
			wantEndOfTestCause: errGracefulEndOfTest,
		},
		{
			name: "success - finalize import",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				workerCommandResponse(api.WORKERCOMMAND_FINALIZE_IMPORT, false),
			},

			wantWorkerResponses: []api.WorkerResponseType{
				api.WORKERRESPONSE_RUNNING,
				api.WORKERRESPONSE_SUCCESS,
			},
			wantEndOfTestCause: nil, // if finalize import is successful, the worker ends it self gracefully, so no cause is given.
		},
		// FIXME: currently hard to test due to the hard coded file system access for the injecting of drivers.
		// {
		// 	name: "success - finalize import for windows",
		// 	migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
		// 		workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
		// 		workerCommandResponse(api.WORKERCOMMAND_FINALIZE_IMPORT, false),
		// 	},
		// 	instanceSpec: instanceDetails{
		// 		OS:        "Windows",
		// 		OSVersion: "Server 2022",
		// 	},

		// 	wantWorkerResponses: []api.WorkerResponseType{
		// 		api.WORKERRESPONSE_RUNNING,
		// 		api.WORKERRESPONSE_SUCCESS,
		// 	},
		// 	wantEndOfTestCause: nil, // if finalize import is successful, the worker ends it self gracefully, so no cause is given.
		// },
		// FIXME: currently hard to test due to the hard coded file system access for post migration.
		// {
		// 	name: "success - finalize import for centos",
		// 	migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
		// 		workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
		// 		workerCommandResponse(api.WORKERCOMMAND_FINALIZE_IMPORT, false),
		// 	},
		// 	instanceSpec: instanceDetails{
		// 		OS: "centos",
		// 	},

		// 	wantWorkerResponses: []api.WorkerResponseType{
		// 		api.WORKERRESPONSE_RUNNING,
		// 		api.WORKERRESPONSE_SUCCESS,
		// 	},
		// 	wantEndOfTestCause: nil, // if finalize import is successful, the worker ends it self gracefully, so no cause is given.
		// },
		{
			name: "error - unknown command is ignored gracefully",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				workerCommandResponse(-1, false),                     // invalid command
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},

			wantEndOfTestCause: errGracefulEndOfTest,
		},
		{
			name: "error - invalid response from migration-managerd is ignored gracefully",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				func(_ instanceDetails, _ context.CancelCauseFunc, w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte(`{`)) // invalid JSON
				},
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},

			wantEndOfTestCause: errGracefulEndOfTest,
		},
		{
			name: "error - invalid metadata from migration-managerd is ignored gracefully",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				func(_ instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, _ *http.Request) {
					resp := incusAPI.Response{
						Type:       incusAPI.SyncResponse,
						Status:     incusAPI.Success.String(),
						StatusCode: int(incusAPI.Success),
						Metadata:   json.RawMessage(`{"command": true}`), // boolean value is invalid for "command", string is expected.
					}

					err := json.NewEncoder(w).Encode(resp)
					if err != nil {
						cancel(fmt.Errorf("commandResponse failed: %w", err))
					}
				},
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},

			wantEndOfTestCause: errGracefulEndOfTest,
		},
		{
			name: "error - import disks delete VM snapshot error",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				workerCommandResponse(api.WORKERCOMMAND_IMPORT_DISKS, false),
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},
			sourceDeleteVMSnapshotErr: fmt.Errorf("boom!"),

			wantWorkerResponses: []api.WorkerResponseType{
				api.WORKERRESPONSE_FAILED,
			},
			wantEndOfTestCause: errGracefulEndOfTest,
		},
		{
			name: "error - finalize import power off vm error",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				workerCommandResponse(api.WORKERCOMMAND_FINALIZE_IMPORT, false),
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},
			sourcePowerOffVMErr: fmt.Errorf("boom!"),

			wantWorkerResponses: []api.WorkerResponseType{
				api.WORKERRESPONSE_FAILED,
			},
			wantEndOfTestCause: errGracefulEndOfTest,
		},
		{
			name: "error - finalize import disks delete VM snapshot error",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				workerCommandResponse(api.WORKERCOMMAND_FINALIZE_IMPORT, false),
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},
			sourceDeleteVMSnapshotErr: fmt.Errorf("boom!"),

			wantWorkerResponses: []api.WorkerResponseType{
				api.WORKERRESPONSE_FAILED,
			},
			wantEndOfTestCause: errGracefulEndOfTest,
		},
		{
			name: "error - finalize import unknown version of windows",
			migrationManagerdResponses: []func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request){
				workerCommandResponse(api.WORKERCOMMAND_IDLE, false), // newWorker connectivity test
				workerCommandResponse(api.WORKERCOMMAND_FINALIZE_IMPORT, false),
				workerCommandResponse(api.WORKERCOMMAND_IDLE, true),
			},
			instanceSpec: instanceDetails{
				OS:        "Windows",
				OSVersion: "invalid",
			},

			wantWorkerResponses: []api.WorkerResponseType{
				api.WORKERRESPONSE_RUNNING,
				api.WORKERRESPONSE_FAILED,
			},
			wantEndOfTestCause: errGracefulEndOfTest,
		},
	}

	// Enable logging if tests are executed in verbose mode.
	if testing.Verbose() {
		err := logger.InitLogger("", true, true)
		require.NoError(t, err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Context with cancel cause is used to end tests on error and return the
			// cause. For the expected "end of test case", errGracefulEndOfTest is
			// returned.
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(fmt.Errorf("deferred cancel"))

			if len(tc.migrationManagerdResponses) == 0 {
				t.Fatal("invalid test case, at least on migration-managerd response is required")
			}

			// Create migration-managerd double.
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.RequestURI {
				case "/1.0":
					if r.Method != http.MethodGet {
						cancel(fmt.Errorf("Unsupported method %q", r.Method))
						return
					}

					fallthrough
				case fmt.Sprintf("/1.0/queue/%s/worker/command?secret=", uuid):
					if r.RequestURI != "/1.0" {
						if r.Method != http.MethodPost {
							cancel(fmt.Errorf("Unsupported method %q", r.Method))
							return
						}
					}

					if len(tc.migrationManagerdResponses) == 1 {
						tc.migrationManagerdResponses[0](tc.instanceSpec, cancel, w, r)
						return
					}

					respFunc := tc.migrationManagerdResponses[0]
					tc.migrationManagerdResponses = tc.migrationManagerdResponses[1:]

					respFunc(tc.instanceSpec, cancel, w, r)
				case fmt.Sprintf("/1.0/queue/%s/worker?secret=", uuid):
					if r.Method != http.MethodPost {
						cancel(fmt.Errorf("Unsupported method %q", r.Method))
						return
					}

					var resp api.WorkerResponse
					err := json.NewDecoder(r.Body).Decode(&resp)
					if err != nil {
						cancel(fmt.Errorf("API response decode: %w", err))
						return
					}

					defer r.Body.Close()

					if len(tc.wantWorkerResponses) == 0 {
						cancel(fmt.Errorf("want worker responses queue already drained, got: %d", resp.Status))
						return
					}

					wantResponse := tc.wantWorkerResponses[0]
					tc.wantWorkerResponses = tc.wantWorkerResponses[1:]

					if wantResponse != resp.Status {
						cancel(fmt.Errorf("expected worker response: %d, got: %d (%s)", wantResponse, resp.Status, resp.StatusString))
					}

					_, _ = w.Write([]byte(`{}`))
				default:
					cancel(fmt.Errorf("Unsupported request %q", r.RequestURI))
					return
				}
			}))
			defer ts.Close()

			source := &source.SourceMock{
				DeleteVMSnapshotFunc: func(ctx context.Context, vmName string, snapshotName string) error {
					return tc.sourceDeleteVMSnapshotErr
				},
				ImportDisksFunc: func(ctx context.Context, vmName string, statusCallback func(string, bool)) error {
					return tc.sourceImportDisksErr
				},
				PowerOffVMFunc: func(ctx context.Context, vmName string) error {
					return tc.sourcePowerOffVMErr
				},
			}

			worker, err := worker.NewWorker(ts.URL, uuid, "",
				// Inject source into worker.
				worker.WithSource(source),
				// No need to wait for a long time during tests.
				worker.WithIdleSleep(1*time.Microsecond),
			)
			require.NoError(t, err)

			wg := sync.WaitGroup{}
			wg.Add(1)

			// Start system under test.
			go func() {
				defer wg.Done()
				worker.Run(ctx)
			}()

			// Make sure our test does not run forever if we mess up.
			go func() {
				t := time.NewTimer(1 * time.Second)
				defer t.Stop()

				select {
				case <-t.C:
					cancel(fmt.Errorf("test case timed out"))
				case <-ctx.Done():
				}
			}()

			wg.Wait()

			require.ErrorIs(t, context.Cause(ctx), tc.wantEndOfTestCause)
		})
	}
}

func workerCommandResponse(command api.WorkerCommandType, signalEndOfTest bool) func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request) {
	return func(instanceSpec instanceDetails, cancel context.CancelCauseFunc, w http.ResponseWriter, r *http.Request) {
		cmd := api.WorkerCommand{
			Command:       command,
			OS:            instanceSpec.OS,
			OSVersion:     instanceSpec.OSVersion,
			InventoryPath: instanceSpec.InventoryPath,
		}

		metadata, err := json.Marshal(cmd)
		if err != nil {
			cancel(fmt.Errorf("commandResponse failed: %w", err))
		}

		resp := incusAPI.Response{
			Type:       incusAPI.SyncResponse,
			Status:     incusAPI.Success.String(),
			StatusCode: int(incusAPI.Success),
			Metadata:   metadata,
		}

		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			cancel(fmt.Errorf("commandResponse failed: %w", err))
		}

		if signalEndOfTest {
			cancel(errGracefulEndOfTest)
		}
	}
}
