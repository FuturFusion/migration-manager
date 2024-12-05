package main

import (
	"context"
	"database/sql"
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
			var err error

			tmpDir := t.TempDir()

			daemon := newDaemon(&DaemonConfig{})
			daemon.db, err = db.OpenDatabase(tmpDir)
			require.NoError(t, err)

			router := mux.NewRouter()
			daemon.createCmd(router, "1.0", targetsCmd)
			srv := httptest.NewServer(router)
			defer srv.Close()

			daemon.db.Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
				return daemon.db.AddTarget(tx, &target.InternalIncusTarget{
					IncusTarget: api.IncusTarget{
						Name:     "foo",
						Endpoint: "bar",
						Insecure: false,
					},
				})
			})
			require.NoError(t, err)

			// Execute test
			resp, err := http.Get(srv.URL + "/1.0/targets")
			require.NoError(t, err)

			// Assert results
			require.Equal(t, tc.wantHTTPStatus, resp.StatusCode)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, tc.wantTargetCount, gjson.GetBytes(body, "metadata.#").Int())
		})
	}
}
