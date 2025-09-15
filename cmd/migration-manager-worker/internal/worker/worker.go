package worker

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	incusAPI "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/cancel"
	"github.com/lxc/incus/v6/shared/revert"
	incusTLS "github.com/lxc/incus/v6/shared/tls"
	"golang.org/x/mod/semver"
	"golang.org/x/sys/unix"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/source"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/internal/version"
	"github.com/FuturFusion/migration-manager/internal/worker"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type Worker struct {
	endpoint           *url.URL
	trustedFingerprint string
	source             source.Source
	uuid               string
	token              string

	lastUpdate time.Time
	idleSleep  time.Duration
}

type WorkerOption func(*Worker) error

func NewWorker(ctx context.Context, client *http.Client, opts ...WorkerOption) (*Worker, error) {
	// Parse the provided URL for the migration manager endpoint.

	endpoint, err := getIncusConfig(ctx, client, "user.migration.endpoint")
	if err != nil {
		return nil, fmt.Errorf("Failed to find worker endpoint from Incus: %w", err)
	}

	token, err := getIncusConfig(ctx, client, "user.migration.token")
	if err != nil {
		return nil, fmt.Errorf("Failed to find connection token from Incus: %w", err)
	}

	fingerprint, err := getIncusConfig(ctx, client, "user.migration.fingerprint")
	if err != nil {
		return nil, fmt.Errorf("Failed to find trusted certificate fingerprint from Incus: %w", err)
	}

	uuid, err := getIncusConfig(ctx, client, "user.migration.uuid")
	if err != nil {
		return nil, fmt.Errorf("Failed to find instance UUID from Incus: %w", err)
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	wrkr := &Worker{
		endpoint:           parsedURL,
		source:             nil,
		uuid:               uuid,
		token:              token,
		trustedFingerprint: fingerprint,
		lastUpdate:         time.Now().UTC(),
		idleSleep:          10 * time.Second,
	}

	for _, opt := range opts {
		err := opt(wrkr)
		if err != nil {
			return nil, err
		}
	}

	// Do a quick connectivity check to the endpoint.
	_, err = wrkr.doHTTPRequestV1("", http.MethodGet, "", nil)
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
	slog.Info("Starting up", slog.String("version", version.Version))

	for {
		done := func() (done bool) {
			resp, err := w.doHTTPRequestV1("/queue/"+w.uuid+"/worker/command", http.MethodPost, "secret="+w.token, nil)
			if err != nil {
				slog.Error("HTTP request failed", logger.Err(err))
				return false
			}

			cmd := api.WorkerCommand{}

			err = responseToStruct(resp, &cmd)
			if err != nil {
				slog.Error("Failed to unmarshal http response", logger.Err(err))
				return false
			}

			switch cmd.Command {
			case api.WORKERCOMMAND_IDLE:
				slog.Debug("Received IDLE command, sleeping")
				return false

			case api.WORKERCOMMAND_IMPORT_DISKS:
				w.importDisks(ctx, cmd)
				return false

			case api.WORKERCOMMAND_FINALIZE_IMPORT:
				w.importDisks(ctx, cmd)
				return false

			case api.WORKERCOMMAND_POST_IMPORT:
				return w.postImportTasks(ctx, cmd)

			default:
				slog.Error("Received unknown command", slog.Any("command", cmd.Command))
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

	if cmd.Command == api.WORKERCOMMAND_FINALIZE_IMPORT {
		slog.Info("Shutting down source VM")

		err := w.source.PowerOffVM(ctx, cmd.Location)
		if err != nil {
			w.sendErrorResponse(err)
			return
		}

		slog.Info("Source VM shutdown complete")
	}

	slog.Info("Performing disk import")

	err := w.importDisksHelper(ctx, cmd)
	if err != nil {
		w.sendErrorResponse(err)
		return
	}

	slog.Info("Disk import completed successfully")
	w.sendStatusResponse(api.WORKERRESPONSE_SUCCESS, "Disk import completed successfully")
}

func (w *Worker) importDisksHelper(ctx context.Context, cmd api.WorkerCommand) error {
	// Delete any existing migration snapshot that might be left over.
	err := w.source.DeleteVMSnapshot(ctx, cmd.Location, internal.IncusSnapshotName)
	if err != nil {
		return err
	}

	sdkFile, cleanup, err := w.getArtifact(api.ARTIFACTTYPE_SDK, cmd, "")
	if err != nil {
		return err
	}

	defer cleanup()

	// unpack the vmware SDK.
	err = util.UnpackTarball("/tmp/vmware", sdkFile)
	if err != nil {
		return fmt.Errorf("Failed to unpack SDK: %w", err)
	}

	// Do the actual import.
	return w.source.ImportDisks(ctx, cmd.Location, func(status string, isImportant bool) {
		slog.Info(status) //nolint:sloglint

		// Only send updates back to the server if important or once every 5 seconds.
		if isImportant || time.Since(w.lastUpdate).Seconds() >= 5 {
			w.lastUpdate = time.Now().UTC()
			w.sendStatusResponse(api.WORKERRESPONSE_RUNNING, status)
		}
	})
}

// postImportTasks performs some finalizing tasks. If everything is executed
// successfully, this function returns true, signaling the everything is done
// and migration-manager-worker can shut down.
// If there is an error or not all the finalizing work has been performed
// yet, false is returned.
func (w *Worker) postImportTasks(ctx context.Context, cmd api.WorkerCommand) (done bool) {
	if w.source == nil {
		err := w.connectSource(ctx, cmd.SourceType, cmd.Source)
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}
	}

	slog.Info("Performing final migration tasks")
	w.sendStatusResponse(api.WORKERRESPONSE_RUNNING, "Performing final migration tasks")

	switch cmd.OSType {
	case api.OSTYPE_WINDOWS:
		file, cleanup, err := w.getArtifact(api.ARTIFACTTYPE_DRIVER, cmd, "")
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}

		defer cleanup()

		winVer, err := worker.MapWindowsVersionToAbbrev(cmd.OSVersion)
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}

		base, recovery, err := worker.DetermineWindowsPartitions()
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}

		err = worker.WindowsInjectDrivers(ctx, winVer, "/dev/"+base, "/dev/"+recovery, file)
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}

	case api.OSTYPE_FORTIGATE:
		ver, err := worker.DetermineFortigateVersion()
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}

		file, cleanup, err := w.getArtifact(api.ARTIFACTTYPE_OSIMAGE, cmd, ver)
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}

		defer cleanup()

		err = worker.ReplaceFortigateBoot(file)
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

	distro := ""
	if strings.Contains(strings.ToLower(cmd.OS), "centos") {
		distro = "CentOS"
	} else if strings.Contains(strings.ToLower(cmd.OS), "debian") {
		distro = "Debian"
	} else if strings.Contains(strings.ToLower(cmd.OS), "opensuse") {
		distro = "openSUSE"
	} else if strings.Contains(strings.ToLower(cmd.OS), "oracle") {
		distro = "Oracle"
	} else if strings.Contains(strings.ToLower(cmd.OS), "rhel") {
		distro = "RHEL"
	} else if strings.Contains(strings.ToLower(cmd.OS), "sles") {
		distro = "SUSE"
	} else if strings.Contains(strings.ToLower(cmd.OS), "ubuntu") {
		distro = "Ubuntu"
	}

	if distro != "" {
		err := worker.LinuxDoPostMigrationConfig(ctx, distro, majorVersion)
		if err != nil {
			w.sendErrorResponse(err)
			return false
		}
	}

	// When the worker is done, the VM will be forced off, so call sync() to ensure all data is saved to disk.
	unix.Sync()

	slog.Info("Final migration tasks completed successfully")
	w.sendStatusResponse(api.WORKERRESPONSE_SUCCESS, "Final migration tasks completed successfully")

	// When we've finished the import, shutdown the worker.
	return true
}

func (w *Worker) connectSource(ctx context.Context, sourceType api.SourceType, sourceRaw json.RawMessage) error {
	var src api.Source

	err := json.Unmarshal(sourceRaw, &src)
	if err != nil {
		return err
	}

	if sourceType != src.SourceType {
		return fmt.Errorf("Source type mismatch; expecting %s but got %s", sourceType, src.SourceType)
	}

	switch src.SourceType {
	case api.SOURCETYPE_VMWARE:
		w.source, err = source.NewInternalVMwareSourceFrom(src)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("Provided source type %q is not usable with `migration-manager-worker`", sourceType)
	}

	return w.source.Connect(ctx)
}

func (w *Worker) sendStatusResponse(statusVal api.WorkerResponseType, statusMessage string) {
	resp := api.WorkerResponse{Status: statusVal, StatusMessage: statusMessage}

	content, err := json.Marshal(resp)
	if err != nil {
		slog.Error("Failed to marshal status response for migration manager", logger.Err(err))
		return
	}

	_, err = w.doHTTPRequestV1("/queue/"+w.uuid+"/worker", http.MethodPost, "secret="+w.token, content)
	if err != nil {
		slog.Error("Failed to send status back to migration manager", logger.Err(err))
		return
	}
}

func (w *Worker) sendErrorResponse(err error) {
	slog.Error("worker error", logger.Err(err))
	resp := api.WorkerResponse{Status: api.WORKERRESPONSE_FAILED, StatusMessage: err.Error()}

	content, err := json.Marshal(resp)
	if err != nil {
		slog.Error("Failed to send error back to migration manager", logger.Err(err))
		return
	}

	_, err = w.doHTTPRequestV1("/queue/"+w.uuid+"/worker", http.MethodPost, "secret="+w.token, content)
	if err != nil {
		slog.Error("Failed to send error back to migration manager", logger.Err(err))
		return
	}
}

func (w *Worker) makeRequest(endpoint string, method string, query string, reader io.Reader) (*http.Request, *http.Client, error) {
	var err error
	w.endpoint.Path, err = url.JoinPath("/1.0/", endpoint)
	if err != nil {
		return nil, nil, err
	}

	w.endpoint.RawQuery = query

	req, err := http.NewRequest(method, w.endpoint.String(), reader)
	if err != nil {
		return nil, nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				if len(rawCerts) == 0 {
					return &tls.CertificateVerificationError{Err: fmt.Errorf("No TLS certificates found")}
				}

				if w.trustedFingerprint == "" {
					return &tls.CertificateVerificationError{Err: fmt.Errorf("No trusted fingerprint found")}
				}

				fingerprint, err := incusTLS.CertFingerprintStr(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rawCerts[0]})))
				if err != nil {
					return &tls.CertificateVerificationError{Err: err}
				}

				trustedFingerprint := strings.ToLower(strings.ReplaceAll(w.trustedFingerprint, ":", ""))
				if fingerprint != strings.ToLower(strings.ReplaceAll(w.trustedFingerprint, ":", "")) {
					return &tls.CertificateVerificationError{Err: fmt.Errorf("Fingerprints do not match. Expected (%s), Got (%s)", trustedFingerprint, fingerprint)}
				}

				return nil
			},
		},
	}

	client := &http.Client{Transport: transport}
	return req, client, nil
}

func (w *Worker) doHTTPRequestV1Writer(endpoint string, method string, query string, writer io.WriteSeeker) error {
	req, client, err := w.makeRequest(endpoint, method, query, nil)
	if err != nil {
		return err
	}

	resp, doneCh, err := cancel.CancelableDownload(nil, client.Do, req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()
	defer close(doneCh)

	if resp.StatusCode != http.StatusOK {
		decoder := json.NewDecoder(resp.Body)
		response := incusAPI.Response{}
		err = decoder.Decode(&response)
		if err != nil {
			return err
		} else if response.Code != 0 {
			return fmt.Errorf("Received an error from the endpoint: %s", response.Error)
		}
	}

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (w *Worker) doHTTPRequestV1(endpoint string, method string, query string, content []byte) (*incusAPI.Response, error) {
	req, client, err := w.makeRequest(endpoint, method, query, bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	response := incusAPI.Response{}

	err = decoder.Decode(&response)
	if err != nil {
		return nil, err
	} else if response.Code != 0 {
		return &response, fmt.Errorf("Received an error from the endpoint: %s", response.Error)
	}

	return &response, nil
}

func responseToStruct(response *incusAPI.Response, targetStruct any) error {
	return json.Unmarshal(response.Metadata, &targetStruct)
}

func getIncusConfig(ctx context.Context, client *http.Client, key string) (string, error) {
	ctx, ctxCancel := context.WithTimeout(ctx, time.Second*5)
	defer ctxCancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://unix.socket/1.0/config/%s", key), nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func (w *Worker) getArtifact(artifactType api.ArtifactType, cmd api.WorkerCommand, osVersion string) (string, func(), error) {
	reverter := revert.New()
	defer reverter.Fail()

	query := fmt.Sprintf("secret=%s&instance=%s", w.token, w.uuid)
	resp, err := w.doHTTPRequestV1("/artifacts", http.MethodGet, query, nil)
	if err != nil {
		return "", nil, err
	}

	var artifacts []api.Artifact
	err = responseToStruct(resp, &artifacts)
	if err != nil {
		return "", nil, err
	}

	var artifact *api.Artifact
	var dir string
	switch artifactType {
	case api.ARTIFACTTYPE_DRIVER:
		artifact, err = w.matchDriverArtifact(artifacts, cmd)
	case api.ARTIFACTTYPE_OSIMAGE:
		artifact, err = w.matchImageArtifact(artifacts, cmd, osVersion)
	case api.ARTIFACTTYPE_SDK:
		artifact, err = w.matchSourceArtifact(artifacts, cmd)
	default:
		return "", nil, fmt.Errorf("Unknown artifact type %q", artifactType)
	}

	if err != nil {
		return "", nil, err
	}

	dir = filepath.Join("/tmp", artifact.UUID.String())
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return "", nil, err
	}

	reverter.Add(func() { _ = os.RemoveAll(dir) })
	requiredFile, err := artifact.DefaultArtifactFile()
	if err != nil {
		return "", nil, err
	}

	var hasRequiredFile bool
	for _, file := range artifact.Files {
		if file != requiredFile {
			continue
		}

		hasRequiredFile = true
		f, err := os.Create(filepath.Join(dir, file))
		if err != nil {
			return "", nil, err
		}

		defer func() { _ = f.Close() }() //nolint:revive
		err = w.doHTTPRequestV1Writer("/artifacts/"+artifact.UUID.String()+"/files/"+file, http.MethodGet, query, f)
		if err != nil {
			return "", nil, err
		}

		break
	}

	if !hasRequiredFile {
		return "", nil, fmt.Errorf("Required file %q not found", requiredFile)
	}

	cleanup := reverter.Clone().Fail

	reverter.Success()

	return filepath.Join(dir, requiredFile), cleanup, nil
}

func (w *Worker) matchSourceArtifact(artifacts []api.Artifact, cmd api.WorkerCommand) (*api.Artifact, error) {
	var artifact *api.Artifact
	for _, a := range artifacts {
		if a.Type == api.ARTIFACTTYPE_SDK && a.Properties.SourceType == cmd.SourceType {
			artifact = &a
			break
		}
	}

	if artifact == nil {
		return nil, fmt.Errorf("Failed to find matching artifact for source type %q", cmd.SourceType)
	}

	return artifact, nil
}

func (w *Worker) matchDriverArtifact(artifacts []api.Artifact, cmd api.WorkerCommand) (*api.Artifact, error) {
	var artifact *api.Artifact
	for _, a := range artifacts {
		match := a.Type == api.ARTIFACTTYPE_DRIVER && a.Properties.OS == cmd.OSType && slices.Contains(a.Properties.Architectures, cmd.Architecture)
		if match {
			artifact = &a
			break
		}
	}

	if artifact == nil {
		return nil, fmt.Errorf("Failed to find a matching artifact for %q architecture %q", cmd.OSType, cmd.Architecture)
	}

	return artifact, nil
}

func (w *Worker) matchImageArtifact(artifacts []api.Artifact, cmd api.WorkerCommand, osVersion string) (*api.Artifact, error) {
	var artifact *api.Artifact
	for _, a := range artifacts {
		match := a.Type == api.ARTIFACTTYPE_OSIMAGE && a.Properties.OS == cmd.OSType && slices.Contains(a.Properties.Architectures, cmd.Architecture)
		expected := semver.Canonical("v" + osVersion)
		if expected == "" {
			return nil, fmt.Errorf("Invalid OS version %q", osVersion)
		}

		var versionsMatch bool
		for _, v := range a.Properties.Versions {
			if semver.Compare(semver.MajorMinor("v"+v), semver.MajorMinor(expected)) == 0 {
				versionsMatch = true
				break
			}
		}

		match = match && versionsMatch
		if match {
			artifact = &a
			break
		}
	}

	if artifact == nil {
		return nil, fmt.Errorf("Failed to find a matching artifact for %q architecture %q version %q", cmd.OSType, cmd.Architecture, osVersion)
	}

	return artifact, nil
}
