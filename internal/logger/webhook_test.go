package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/shared/api"
	"github.com/FuturFusion/migration-manager/shared/api/event"
)

func TestLogWebhook(t *testing.T) {
	uuidA := uuid.New()
	uuidB := uuid.New()
	uuidC := uuid.New()

	defaultLog := api.Event{Type: api.LogScopeLogging, Metadata: []byte(`{"message":"TEST","level":"ERROR","context":{"key":"val","key2":"val2"}}`)}
	cases := []struct {
		name string
		cfg  api.SystemSettingsLog

		instanceData api.Instance
		queueData    api.QueueEntry

		wantResps     []api.Event
		wantLifecycle api.EventLifecycle
		wantLog       api.EventLogging
		numReqs       int
		sendLog       func(log *slog.Logger) func(msg string, args ...any)
	}{
		{
			name:    "success - can receive lifecycle and logging",
			numReqs: 2,
			cfg: api.SystemSettingsLog{
				Name:         "webhook",
				Type:         api.LogTypeWebhook,
				Level:        "warn",
				Address:      "*", // apply test server address
				RetryCount:   3,
				RetryTimeout: "10s",
				Scopes:       []api.LogScope{api.LogScopeLifecycle, api.LogScopeLogging},
			},

			instanceData: api.Instance{
				Source: "src1",
				InstanceProperties: api.InstanceProperties{
					UUID:                           uuidA,
					Location:                       "/path/to/instance1",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Name: "instance1"},
					NICs:                           []api.InstancePropertiesNIC{{UUID: uuidB}, {UUID: uuidC}},
				},
			},
			queueData: api.QueueEntry{
				InstanceUUID:    uuidA,
				InstanceName:    "instance1",
				BatchName:       "batch1",
				MigrationWindow: api.MigrationWindow{Name: "window1", Config: api.MigrationWindowConfig{Capacity: 10}},
				Placement:       api.Placement{TargetName: "tgt1"},
			},

			wantResps: []api.Event{
				{Type: api.LogScopeLifecycle, Metadata: []byte("lifecycle")}, // apply wantLifecycle
				defaultLog,
			},
			wantLifecycle: api.EventLifecycle{
				Action: string(event.MigrationCreated),
				Entities: []string{
					"/1.0/queue/" + uuidA.String(),
					"/1.0/instances/" + uuidA.String(),
					"/1.0/batches/batch1",
					"/1.0/sources/src1",
					"/1.0/targets/tgt1",
					"/1.0/networks/" + uuidB.String(),
					"/1.0/networks/" + uuidC.String(),
				},
				Metadata: []byte("*"), // apply objects
			},
			sendLog: func(log *slog.Logger) func(msg string, args ...any) {
				return log.Error
			},
		},
		{
			name:    "success - logging omitted",
			numReqs: 1,
			cfg: api.SystemSettingsLog{
				Name:         "webhook",
				Type:         api.LogTypeWebhook,
				Level:        "warn",
				Address:      "*", // apply test server address
				RetryCount:   3,
				RetryTimeout: "10s",
				Scopes:       []api.LogScope{api.LogScopeLifecycle},
			},

			instanceData: api.Instance{
				Source: "src1",
				InstanceProperties: api.InstanceProperties{
					UUID:                           uuidA,
					Location:                       "/path/to/instance1",
					InstancePropertiesConfigurable: api.InstancePropertiesConfigurable{Name: "instance1"},
					NICs:                           []api.InstancePropertiesNIC{{UUID: uuidB}, {UUID: uuidC}},
				},
			},
			queueData: api.QueueEntry{
				InstanceUUID:    uuidA,
				InstanceName:    "instance1",
				BatchName:       "batch1",
				MigrationWindow: api.MigrationWindow{Name: "window1", Config: api.MigrationWindowConfig{Capacity: 10}},
				Placement:       api.Placement{TargetName: "tgt1"},
			},

			wantResps: []api.Event{
				{Type: api.LogScopeLifecycle, Metadata: []byte("lifecycle")}, // apply wantLifecycle
			},
			wantLifecycle: api.EventLifecycle{
				Action: string(event.MigrationCreated),
				Entities: []string{
					"/1.0/queue/" + uuidA.String(),
					"/1.0/instances/" + uuidA.String(),
					"/1.0/batches/batch1",
					"/1.0/sources/src1",
					"/1.0/targets/tgt1",
					"/1.0/networks/" + uuidB.String(),
					"/1.0/networks/" + uuidC.String(),
				},
				Metadata: []byte("*"), // apply objects
			},
			sendLog: func(log *slog.Logger) func(msg string, args ...any) {
				return log.Error
			},
		},
		{
			name:    "success - lifecycle omitted",
			numReqs: 1,
			cfg: api.SystemSettingsLog{
				Name:         "webhook",
				Type:         api.LogTypeWebhook,
				Level:        "warn",
				Address:      "*",
				RetryCount:   3,
				RetryTimeout: "10s",
				Scopes:       []api.LogScope{api.LogScopeLogging},
			},
			wantResps: []api.Event{defaultLog},
			sendLog: func(log *slog.Logger) func(msg string, args ...any) {
				return log.Error
			},
		},
		{
			name:    "success - all omitted",
			numReqs: 0,
			cfg: api.SystemSettingsLog{
				Name:         "webhook",
				Type:         api.LogTypeWebhook,
				Level:        "warn",
				Address:      "*",
				RetryCount:   3,
				RetryTimeout: "10s",
				Scopes:       []api.LogScope{},
			},
			sendLog: func(log *slog.Logger) func(msg string, args ...any) {
				return log.Error
			},
		},
		{
			name:    "success - discard log level",
			numReqs: 0,
			cfg: api.SystemSettingsLog{
				Name:         "webhook",
				Type:         api.LogTypeWebhook,
				Level:        "warn",
				Address:      "*",
				RetryCount:   3,
				RetryTimeout: "10s",
				Scopes:       []api.LogScope{api.LogScopeLogging},
			},
			sendLog: func(log *slog.Logger) func(msg string, args ...any) {
				return log.Info
			},
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			timeCtx, timeCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer timeCancel()
			ctx, cancel := context.WithCancelCause(timeCtx)
			defer cancel(context.Canceled)

			numReqs := 0
			errs := []error{}
			events := []api.Event{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				numReqs++

				if r.URL.Path != "/test" || r.Method != http.MethodPost {
					err := fmt.Errorf("Invalid endpoint. URL: %q, Method: %v", r.URL.String(), r.Method)
					errs = append(errs, err)
					cancel(err)
					return
				}

				var e api.Event
				err := json.NewDecoder(r.Body).Decode(&e)
				if err != nil {
					err := fmt.Errorf("Invalid response: %w", err)
					errs = append(errs, err)
					cancel(err)
					return
				}

				if !slices.Contains(tc.cfg.Scopes, e.Type) {
					err := fmt.Errorf("Unexpected request for omitted scope. Scope: %q, URL: %q, Method: %v", e.Type, r.URL.String(), r.Method)
					errs = append(errs, err)
					cancel(err)
					return
				}

				events = append(events, e)

				w.WriteHeader(http.StatusOK)
				if numReqs == 2 {
					cancel(context.Canceled)
				}
			}))
			defer server.Close()

			if tc.cfg.Address == "*" {
				tc.cfg.Address = server.URL + "/test"
			}

			if string(tc.wantLifecycle.Metadata) == "*" {
				b, err := json.Marshal(event.MigrationDetails{Instance: tc.instanceData.ToFilterable(), QueueEntry: tc.queueData})
				require.NoError(t, err)

				tc.wantLifecycle.Metadata = b
			}

			for i := range tc.wantResps {
				if string(tc.wantResps[i].Metadata) == "lifecycle" {
					var err error
					tc.wantResps[i].Metadata, err = json.Marshal(tc.wantLifecycle)
					require.NoError(t, err)
				}
			}

			webhook, err := NewWebhookLogger(tc.cfg)
			require.NoError(t, err)
			handler := NewLogHandler(slog.LevelWarn, slog.HandlerOptions{})
			handler.AddHandler(webhook)
			log := slog.New(handler)

			handler.SendLifecycle(context.Background(), event.NewMigrationEvent(event.MigrationCreated, tc.instanceData, tc.queueData))
			tc.sendLog(log)("TEST", slog.Any("key", "val"), slog.Any("key2", "val2"))

			<-ctx.Done()
			require.Error(t, ctx.Err())
			if ctx.Err() != nil {
				if tc.numReqs != 2 {
					require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
				} else {
					require.ErrorIs(t, ctx.Err(), context.Canceled)
				}
			}

			for _, err := range errs {
				require.Fail(t, err.Error())
			}

			require.Equal(t, tc.numReqs, numReqs)
			require.Len(t, events, len(tc.wantResps))
			require.Len(t, events, tc.numReqs)
			for _, e := range events {
				require.False(t, e.Time.IsZero())
				e.Time = time.Time{}
				require.Contains(t, tc.wantResps, e)
			}
		})
	}
}
