package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/FuturFusion/migration-manager/internal/db"
	"github.com/FuturFusion/migration-manager/internal/target"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestTargetsGet(t *testing.T) {
	tests := []struct {
		name string

		wantHTTPStatus  int
		wantTargetCount int64
	}{
		{
			name: "success",

			wantHTTPStatus:  http.StatusOK,
			wantTargetCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, srvURL := daemonSetup(t, []APIEndpoint{targetsCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, body := probeAPI(t, http.MethodGet, srvURL+"/1.0/targets", http.NoBody)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
			require.Equal(t, tc.wantTargetCount, gjson.Get(body, "metadata.#").Int())
		})
	}
}

func TestTargetsPost(t *testing.T) {
	tests := []struct {
		name string

		targetJSON string

		wantHTTPStatus int
	}{
		{
			name: "success",

			targetJSON: `{"name": "new", "endpoint": "some endpoint", "insecure": true}`,

			wantHTTPStatus: http.StatusCreated,
		},
		{
			name: "error - name already exists",

			targetJSON: `{"name": "foo", "endpoint": "some endpoint", "insecure": true}`,

			// TODO: Unique constraint violation leads to http.StatusInternalServerError
			// shouldn't this be http.BadRequest or http.StatusConflict?
			wantHTTPStatus: http.StatusInternalServerError,
		},
		{
			name: "error - invalid JSON",

			targetJSON: `{`,

			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, srvURL := daemonSetup(t, []APIEndpoint{targetsCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, _ := probeAPI(t, http.MethodPost, srvURL+"/1.0/targets", bytes.NewBufferString(tc.targetJSON))

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
		})
	}
}

func TestTargetDelete(t *testing.T) {
	tests := []struct {
		name string

		targetName string

		wantHTTPStatus int
	}{
		{
			name: "success",

			targetName: "foo",

			wantHTTPStatus: http.StatusOK,
		},
		{
			name: "error - empty name",

			targetName: "",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - empty name (with final slash)",

			targetName: "/",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - not found",

			targetName: "invalid_name",

			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, srvURL := daemonSetup(t, []APIEndpoint{targetCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, _ := probeAPI(t, http.MethodDelete, srvURL+fmt.Sprintf("/1.0/targets/%s", tc.targetName), http.NoBody)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
		})
	}
}

func TestTargetGet(t *testing.T) {
	tests := []struct {
		name string

		targetName string

		wantHTTPStatus int
		wantTargetName string
	}{
		{
			name: "success",

			targetName: "foo",

			wantHTTPStatus: http.StatusOK,
			wantTargetName: "foo",
		},
		{
			name: "error - empty name",

			targetName: "",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - empty name (with final slash)",

			targetName: "/",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - not found",

			targetName: "invalid_name",

			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, srvURL := daemonSetup(t, []APIEndpoint{targetCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, body := probeAPI(t, http.MethodGet, srvURL+fmt.Sprintf("/1.0/targets/%s", tc.targetName), http.NoBody)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
			require.Equal(t, tc.wantTargetName, gjson.Get(body, "metadata.name").String())
		})
	}
}

func TestTargetsPut(t *testing.T) {
	tests := []struct {
		name string

		targetName string
		targetJSON string

		wantHTTPStatus int
	}{
		{
			name: "success",

			targetName: "foo",
			targetJSON: `{"name": "foo", "endpoint": "some endpoint", "insecure": true}`,

			// TODO: why is http.StatusCreated returned for an update operation?
			wantHTTPStatus: http.StatusCreated,
		},
		{
			name: "error - empty name",

			targetName: "",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - empty name (with final slash)",

			targetName: "/",

			// TODO: the business logic would like to return http.BadRequest for this
			// but this gets never hit, because the router is already handling this
			// request with http.StatusNotFound.
			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name: "error - not found",

			targetName: "invalid_target",

			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name: "error - invalid JSON",

			targetName: "foo",
			targetJSON: `{`,

			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, srvURL := daemonSetup(t, []APIEndpoint{targetCmd})
			seedDBWithSingleTarget(t, daemon)

			// Execute test
			statusCode, _ := probeAPI(t, http.MethodPut, srvURL+fmt.Sprintf("/1.0/targets/%s", tc.targetName), bytes.NewBufferString(tc.targetJSON))

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, statusCode)
		})
	}
}

func probeAPI(t *testing.T, method string, url string, requestBody io.Reader) (statusCode int, responseBody string) {
	t.Helper()

	req, err := http.NewRequest(method, url, requestBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(body)
}

func daemonSetup(t *testing.T, endpoints []APIEndpoint) (*Daemon, string) {
	t.Helper()

	var err error

	tmpDir := t.TempDir()

	daemon := newDaemon(&DaemonConfig{})
	daemon.db, err = db.OpenDatabase(tmpDir)
	require.NoError(t, err)

	router := mux.NewRouter()
	for _, cmd := range endpoints {
		daemon.createCmd(router, "1.0", cmd)
	}

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return daemon, srv.URL
}

func seedDBWithSingleTarget(t *testing.T, daemon *Daemon) {
	t.Helper()

	err := daemon.db.Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		return daemon.db.AddTarget(tx, &target.InternalIncusTarget{
			IncusTarget: api.IncusTarget{
				Name:     "foo",
				Endpoint: "bar",
				Insecure: true,
			},
		})
	})
	require.NoError(t, err)
}
