package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/lxc/incus/v6/shared/api"
	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"

	"github.com/FuturFusion/migration-manager/internal/logger"
)

// FGA represents an OpenFGA authorizer.
type FGA struct {
	commonAuthorizer
	tls *TLS

	apiURL   string
	apiToken string
	storeID  string

	onlineMu sync.Mutex
	online   bool

	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	client *client.OpenFgaClient
}

func (f *FGA) configure(opts Opts) error {
	if opts.config == nil {
		return fmt.Errorf("Missing OpenFGA config")
	}

	val, ok := opts.config["openfga.api.token"]
	if !ok || val == nil {
		return fmt.Errorf("Missing OpenFGA API token")
	}

	f.apiToken, ok = val.(string)
	if !ok {
		return fmt.Errorf("Expected a string for configuration key %q, got: %T", "openfga.api.token", val)
	}

	val, ok = opts.config["openfga.api.url"]
	if !ok || val == nil {
		return fmt.Errorf("Missing OpenFGA API URL")
	}

	f.apiURL, ok = val.(string)
	if !ok {
		return fmt.Errorf("Expected a string for configuration key %q, got: %T", "openfga.api.url", val)
	}

	val, ok = opts.config["openfga.store.id"]
	if !ok || val == nil {
		return fmt.Errorf("Missing OpenFGA store ID")
	}

	f.storeID, ok = val.(string)
	if !ok {
		return fmt.Errorf("Expected a string for configuration key %q, got: %T", "openfga.store.id", val)
	}

	return nil
}

func (f *FGA) load(ctx context.Context, certificateFingerprints []string, opts Opts) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := f.configure(opts)
	if err != nil {
		return err
	}

	f.tls = &TLS{}
	err = f.tls.load(ctx, certificateFingerprints, opts)
	if err != nil {
		return err
	}

	conf := client.ClientConfiguration{
		ApiUrl:  f.apiURL,
		StoreId: f.storeID,
		Credentials: &credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: f.apiToken,
			},
		},
	}

	f.client, err = client.NewSdkClient(&conf)
	if err != nil {
		return fmt.Errorf("Failed to create OpenFGA client: %w", err)
	}

	f.shutdownCtx, f.shutdownCancel = context.WithCancel(context.Background())

	// Connect in the background.
	go func(ctx context.Context) {
		first := true

		for {
			// Attempt a connection.
			err := f.connect(ctx)
			if err == nil {
				slog.Info("Connection with OpenFGA established")

				f.onlineMu.Lock()
				defer f.onlineMu.Unlock() //nolint:revive
				f.online = true

				return
			}

			// Handle re-tries.
			if first {
				slog.Warn("Unable to connect to the OpenFGA server, will retry every 30s", logger.Err(err))
				first = false
			}

			select {
			case <-time.After(30 * time.Second):
				continue
			case <-f.shutdownCtx.Done():
				return
			}
		}
	}(f.shutdownCtx)

	return nil
}

// StopService stops the authorizer gracefully.
func (f *FGA) StopService(ctx context.Context) error {
	// Cancel any background routine.
	f.shutdownCancel()

	return nil
}

func (f *FGA) refreshModel(ctx context.Context) error {
	var builtinAuthorizationModel client.ClientWriteAuthorizationModelRequest
	err := json.Unmarshal([]byte(authModel), &builtinAuthorizationModel)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal built in authorization model: %w", err)
	}

	_, err = f.client.WriteAuthorizationModel(ctx).Body(builtinAuthorizationModel).Execute()
	if err != nil {
		return fmt.Errorf("Failed to write the authorization model: %w", err)
	}

	return nil
}

func (f *FGA) connect(ctx context.Context) error {
	// Load current authorization model.
	readModelResponse, err := f.client.ReadLatestAuthorizationModel(ctx).Execute()
	if err != nil {
		return fmt.Errorf("Failed to read pre-existing OpenFGA model: %w", err)
	}

	// Check if we need to upload an initial model.
	if readModelResponse.AuthorizationModel == nil {
		slog.Info("Upload initial OpenFGA model")

		// Upload the model itself.
		err := f.refreshModel(ctx)
		if err != nil {
			return fmt.Errorf("Failed to load initial model: %w", err)
		}

		// Allow basic authenticated access.
		err = f.sendTuples(ctx, []client.ClientTupleKey{
			{User: "user:*", Relation: "authenticated", Object: ObjectServer().String()},
		}, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// CheckPermission returns an error if the user does not have the given Entitlement on the given Object.
func (f *FGA) CheckPermission(ctx context.Context, r *http.Request, object Object, entitlement Entitlement) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	details, err := f.requestDetails(r)
	if err != nil {
		return api.StatusErrorf(http.StatusForbidden, "Failed to extract request details: %v", err)
	}

	// Always allow full access via local unix socket.
	if details.Protocol == "unix" {
		return nil
	}

	// Use the TLS driver if the user authenticated with TLS.
	if details.Protocol == api.AuthenticationMethodTLS {
		return f.tls.CheckPermission(ctx, r, object, entitlement)
	}

	// If offline, return a clear error to the user.
	f.onlineMu.Lock()
	online := f.online
	defer f.onlineMu.Unlock()
	if !online {
		return api.StatusErrorf(http.StatusForbidden, "The authorization server is currently offline, please try again later")
	}

	username := details.Username

	objectUser := ObjectUser(username)
	body := client.ClientCheckRequest{
		User:     objectUser.String(),
		Relation: string(entitlement),
		Object:   object.String(),
	}

	slog.Debug("Checking OpenFGA relation", slog.Any("object", object), slog.Any("entitlement", entitlement), slog.String("url", r.URL.String()), slog.String("method", r.Method), slog.String("username", username), slog.String("protocol", details.Protocol))
	resp, err := f.client.Check(ctx).Body(body).Execute()
	if err != nil {
		return fmt.Errorf("Failed to check OpenFGA relation: %w", err)
	}

	if !resp.GetAllowed() {
		return api.StatusErrorf(http.StatusForbidden, "User does not have entitlement %q on object %q", entitlement, object)
	}

	return nil
}

// sendTuples directly sends the write/deletion tuples to OpenFGA.
func (f *FGA) sendTuples(ctx context.Context, writes []client.ClientTupleKey, deletions []client.ClientTupleKeyWithoutCondition) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	opts := client.ClientWriteOptions{
		Transaction: &client.TransactionOptions{
			Disable:             true,
			MaxParallelRequests: 5,
			MaxPerChunk:         50,
		},
	}

	body := client.ClientWriteRequest{
		Writes:  []client.ClientTupleKey{},
		Deletes: []openfga.TupleKeyWithoutCondition{},
	}

	if writes != nil {
		body.Writes = writes
	}

	if deletions != nil {
		body.Deletes = deletions
	}

	clientWriteResponse, err := f.client.Write(ctx).Options(opts).Body(body).Execute()
	if err != nil {
		return fmt.Errorf("Failed to write to OpenFGA store: %w", err)
	}

	errs := []error{}

	for _, write := range clientWriteResponse.Writes {
		if write.Error != nil {
			errs = append(errs, fmt.Errorf("Failed to write tuple to OpenFGA store (user: %q; relation: %q; object: %q): %w", write.TupleKey.User, write.TupleKey.Relation, write.TupleKey.Object, write.Error))
		}
	}

	for _, deletion := range clientWriteResponse.Deletes {
		if deletion.Error != nil {
			errs = append(errs, fmt.Errorf("Failed to delete tuple from OpenFGA store (user: %q; relation: %q; object: %q): %w", deletion.TupleKey.User, deletion.TupleKey.Relation, deletion.TupleKey.Object, deletion.Error))
		}
	}

	return errors.Join(errs...)
}
