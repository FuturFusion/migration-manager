package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/instance"
	"github.com/FuturFusion/migration-manager/shared/api"
)

const instanceUUID = "35247892-bb9e-11ef-b6f8-d332f724e06b"

func TestInstanceStatePut(t *testing.T) {
	tests := []struct {
		name                  string
		migrationStatus       api.MigrationStatusType
		migrationUserDisabled string
		instanceUUID          string

		wantHTTPStatus int
	}{
		{
			name:                  "success - migration_user_disabled=true",
			migrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			migrationUserDisabled: "true",
			instanceUUID:          instanceUUID,

			wantHTTPStatus: http.StatusCreated,
		},
		{
			name:                  "success - migration_user_disabled=false",
			migrationStatus:       api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
			migrationUserDisabled: "false",
			instanceUUID:          instanceUUID,

			wantHTTPStatus: http.StatusCreated,
		},
		{
			name:                  "error - migration_user_disabled=true not allowed for migration state other then MIGRATIONSTATUS_NOT_ASSIGNED_BATCH",
			migrationStatus:       api.MIGRATIONSTATUS_ASSIGNED_BATCH,
			migrationUserDisabled: "true",
			instanceUUID:          instanceUUID,

			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name:                  "error - migration_user_disabled=false not allowed for migration state other then MIGRATIONSTATUS_USER_DISABLED_MIGRATION",
			migrationStatus:       api.MIGRATIONSTATUS_USER_DISABLED_MIGRATION,
			migrationUserDisabled: "true",
			instanceUUID:          instanceUUID,

			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name:         "error - invalid instance UUID",
			instanceUUID: "invalid", // invalid UUID

			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name:         "error - empty instance UUID",
			instanceUUID: "", // empty instance UUID

			wantHTTPStatus: http.StatusNotFound,
		},
		{
			name:                  "error - invalid value for migration_user_disabled",
			migrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			migrationUserDisabled: "invalid", // invalid value
			instanceUUID:          instanceUUID,

			wantHTTPStatus: http.StatusBadRequest,
		},
		{
			name:                  "error - invalid value for migration_user_disabled",
			migrationStatus:       api.MIGRATIONSTATUS_NOT_ASSIGNED_BATCH,
			migrationUserDisabled: "true",
			instanceUUID:          "8bdc89d0-bbae-11ef-bf8d-ebc627b4135a", // instance UUID does not exist

			wantHTTPStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			daemon, srvURL := daemonSetup(t, []APIEndpoint{instanceStateCmd})
			seedDBWithSingleInstance(t, daemon, tc.migrationStatus)

			// Execute test
			statusCode, body := probeAPI(t, http.MethodPut, srvURL+fmt.Sprintf("/1.0/instances/%s/state?migration_user_disabled=%s", tc.instanceUUID, tc.migrationUserDisabled), http.NoBody, nil)

			// Assert results
			t.Log(body)
			require.Equal(t, tc.wantHTTPStatus, statusCode)
		})
	}
}

func seedDBWithSingleInstance(t *testing.T, daemon *Daemon, migrationStatus api.MigrationStatusType) {
	t.Helper()

	err := daemon.db.Transaction(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		source := api.Source{
			Name:       "source",
			SourceType: api.SOURCETYPE_COMMON,
			Properties: json.RawMessage(`{}`),
		}

		source, err := daemon.db.AddSource(tx, source)
		if err != nil {
			return err
		}

		return daemon.db.AddInstance(tx, &instance.InternalInstance{
			Instance: api.Instance{
				UUID:            uuid.MustParse(instanceUUID),
				MigrationStatus: migrationStatus,
				SourceID:        source.DatabaseID,
			},
		})
	})
	require.NoError(t, err)
}
