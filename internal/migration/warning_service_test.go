package migration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/internal/testing/queue"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestWarningService_Emit(t *testing.T) {
	testWarning := func(dbID int64, wType api.WarningType, srcName string, count int, messages ...string) migration.Warning {
		w := migration.NewSyncWarning(wType, srcName, "")
		w.Messages = messages
		now := time.Now().UTC()
		w.ID = dbID
		w.LastSeenDate = now
		w.FirstSeenDate = now
		w.UpdatedDate = now
		w.Count = count

		return w
	}

	tests := []struct {
		name    string
		warning migration.Warning

		repoGetByScopeWarnings migration.Warnings
		repoGetByScopeErr      error

		repoUpsertErr  error
		resultMessages []string

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:           "success - new warning",
			warning:        migration.NewSyncWarning(api.NetworkImportFailed, "src1", "message1"),
			resultMessages: []string{"message1"},
			assertErr:      require.NoError,
		},
		{
			name:    "success - merged",
			warning: migration.NewSyncWarning(api.NetworkImportFailed, "src1", "message2"),
			repoGetByScopeWarnings: migration.Warnings{
				testWarning(999, api.NetworkImportFailed, "src1", 10, "message0", "message1"),
			},

			resultMessages: []string{"message0", "message1", "message2"},
			assertErr:      require.NoError,
		},
		{
			name:    "success - merged, reordered message",
			warning: migration.NewSyncWarning(api.NetworkImportFailed, "src1", "message0"),
			repoGetByScopeWarnings: migration.Warnings{
				testWarning(999, api.NetworkImportFailed, "src1", 10, "message0", "message1"),
			},

			resultMessages: []string{"message1", "message0"},
			assertErr:      require.NoError,
		},
		{
			name:    "error - vague match",
			warning: migration.NewSyncWarning(api.NetworkImportFailed, "src1", "message0"),
			repoGetByScopeWarnings: migration.Warnings{
				testWarning(999, api.NetworkImportFailed, "src1", 10, "message0", "message1"),
				testWarning(998, api.NetworkImportFailed, "src2", 10, "message0", "message1"),
			},

			assertErr: require.Error,
		},
		{
			name:      "error - validation",
			warning:   migration.NewSyncWarning(api.NetworkImportFailed, "", "message0"), // no source.
			assertErr: require.Error,
		},
		{
			name:          "error - repoUpsert",
			warning:       migration.NewSyncWarning(api.NetworkImportFailed, "src1", "message0"),
			repoUpsertErr: boom.Error,
			assertErr:     boom.ErrorIs,
		},
		{
			name:              "error - repoGetByScope",
			warning:           migration.NewSyncWarning(api.NetworkImportFailed, "src1", "message0"),
			repoGetByScopeErr: boom.Error,
			assertErr:         boom.ErrorIs,
		},
	}

	for i, tc := range tests {
		t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
		t.Run(tc.name, func(t *testing.T) {
			repo := &mock.WarningRepoMock{
				GetByScopeAndTypeFunc: func(ctx context.Context, scope api.WarningScope, wType api.WarningType) (migration.Warnings, error) {
					return tc.repoGetByScopeWarnings, tc.repoGetByScopeErr
				},
				UpsertFunc: func(ctx context.Context, w migration.Warning) (int64, error) {
					var id int64 = 1
					if len(tc.repoGetByScopeWarnings) > 0 {
						id = tc.repoGetByScopeWarnings[0].ID
					}

					return id, tc.repoUpsertErr
				},
			}

			warningSvc := migration.NewWarningService(repo)

			// Perform test.
			ctx := context.Background()
			now := time.Now().UTC()
			out, err := warningSvc.Emit(ctx, tc.warning)
			tc.assertErr(t, err)
			if err == nil {
				require.NotEqual(t, out.LastSeenDate, tc.warning.LastSeenDate)
				require.NotEqual(t, out.FirstSeenDate, tc.warning.FirstSeenDate)
				id := int64(1)
				count := 1
				wUUID := tc.warning.UUID

				// Check that warnings merged.
				if len(tc.repoGetByScopeWarnings) == 1 {
					oldWarning := tc.repoGetByScopeWarnings[0]
					wUUID = oldWarning.UUID
					require.Equal(t, out.FirstSeenDate, oldWarning.FirstSeenDate)
					require.Equal(t, out.LastSeenDate, oldWarning.LastSeenDate)
					id = oldWarning.ID
					count += oldWarning.Count
				}

				require.True(t, out.UpdatedDate.After(now))
				require.Equal(t, tc.warning.Scope, out.Scope)
				require.Equal(t, tc.warning.Entity, out.Entity)
				require.Equal(t, tc.warning.EntityType, out.EntityType)
				require.Equal(t, tc.warning.Status, out.Status)
				require.Equal(t, id, out.ID)
				require.Equal(t, wUUID, out.UUID)
				require.Equal(t, count, out.Count)
				require.Equal(t, tc.resultMessages, out.Messages)
			}
		})
	}
}

func TestWarningService_RemoveStale(t *testing.T) {
	testWarning := func(srcName string, messages ...string) migration.Warning {
		warning := migration.NewSyncWarning(api.InstanceCannotMigrate, srcName, "")
		warning.Messages = messages
		return warning
	}

	testScope := func(srcName string) api.WarningScope {
		scope := api.WarningScopeSync()
		scope.Entity = srcName
		return scope
	}

	q := func(messages ...string) queue.Item[[]string] {
		return queue.Item[[]string]{Value: messages}
	}

	type list []queue.Item[[]string]
	tests := []struct {
		name     string
		warnings migration.Warnings
		scope    api.WarningScope

		repoGetAllWarnings migration.Warnings

		repoUpdate       []queue.Item[[]string]
		repoDeleteByUUID []queue.Item[[]string]

		repoGetAllErr error
		assertErr     require.ErrorAssertionFunc
	}{
		{
			name:               "success - no change in messages",
			scope:              testScope(""),
			warnings:           migration.Warnings{testWarning("a", "a"), testWarning("b", "b"), testWarning("c", "c")},
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a"), testWarning("b", "b"), testWarning("c", "c")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - no new messages, clear all matching broad scope",
			scope:              testScope(""),
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a"), testWarning("b", "b"), testWarning("c", "c")},
			repoDeleteByUUID:   list{q("a"), q("b"), q("c")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - no new messages, clear all matching narrow scope",
			scope:              testScope("a"),
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a"), testWarning("b", "b"), testWarning("c", "c")},
			repoDeleteByUUID:   list{q("a")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - new messages, remove old messages matching broad scope",
			scope:              testScope(""),
			warnings:           migration.Warnings{testWarning("a", "a"), testWarning("b", "b"), testWarning("c", "c")},
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a"), testWarning("b", "b", "b2"), testWarning("c", "c", "c2")},
			repoUpdate:         list{q("b"), q("c")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - new messages, remove old messages matching broad scope, and clear empty warnings",
			scope:              testScope(""),
			warnings:           migration.Warnings{testWarning("a", "a2"), testWarning("b", "b"), testWarning("c", "c")},
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a"), testWarning("b", "b", "b2"), testWarning("c", "c", "c2")},
			repoDeleteByUUID:   list{q("a")},
			repoUpdate:         list{q("b"), q("c")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - new messages, with duplicate scope",
			scope:              testScope(""),
			warnings:           migration.Warnings{testWarning("a", "a"), testWarning("a", "a2"), testWarning("b", "b"), testWarning("c", "c")},
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a", "a2"), testWarning("b", "b", "b2"), testWarning("c", "c", "c2")},
			repoUpdate:         list{q("b"), q("c")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - new messages, remove old messages matching broad scope, and clear empty warnings",
			scope:              testScope("b"),
			warnings:           migration.Warnings{testWarning("a", "a2"), testWarning("b", "b"), testWarning("c", "c")},
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a"), testWarning("b", "b", "b2"), testWarning("c", "c", "c2")},
			repoUpdate:         list{q("b")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - new messages, duplicate cross-scope messages, scope is broad",
			scope:              testScope(""),
			warnings:           migration.Warnings{testWarning("a", "a"), testWarning("b", "b")},
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a", "a2"), testWarning("b", "a", "a2"), testWarning("c", "a", "a2")},
			repoUpdate:         list{q("a"), q("a"), q("a")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - new messages, duplicate cross-scope messages, scope is narrow",
			scope:              testScope("a"),
			warnings:           migration.Warnings{testWarning("a", "a"), testWarning("b", "b")},
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a", "a2"), testWarning("b", "a", "a2"), testWarning("c", "a", "a2")},
			repoUpdate:         list{q("a")},
			assertErr:          require.NoError,
		},
		{
			name:               "success - new messages, unmatched duplicate cross-scope messages, scope is narrow",
			scope:              testScope("b"),
			warnings:           migration.Warnings{testWarning("a", "a"), testWarning("b", "b")},
			repoGetAllWarnings: migration.Warnings{testWarning("a", "a", "a2"), testWarning("b", "a", "a2"), testWarning("c", "a", "a2")},
			repoDeleteByUUID:   list{q("a", "a2")},
			assertErr:          require.NoError,
		},
		{
			name:          "error - repoGetAll",
			scope:         testScope("b"),
			warnings:      migration.Warnings{testWarning("a", "a"), testWarning("b", "b")},
			repoGetAllErr: boom.Error,
			assertErr:     boom.ErrorIs,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			repo := &mock.WarningRepoMock{
				DeleteByUUIDFunc: func(ctx context.Context, id uuid.UUID) error {
					var warningMessages []string
					for _, w := range tc.repoGetAllWarnings {
						if w.UUID == id {
							warningMessages = w.Messages
							break
						}
					}

					messages, err := queue.Pop(t, &tc.repoDeleteByUUID)
					if err == nil {
						require.Equal(t, messages, warningMessages)
					}

					return err
				},
				UpdateFunc: func(ctx context.Context, id uuid.UUID, w migration.Warning) error {
					messages, err := queue.Pop(t, &tc.repoUpdate)
					if err == nil {
						require.Equal(t, messages, w.Messages)
					}

					return err
				},
				GetAllFunc: func(ctx context.Context) (migration.Warnings, error) {
					return tc.repoGetAllWarnings, tc.repoGetAllErr
				},
			}

			warningSvc := migration.NewWarningService(repo)

			// Perform test.
			ctx := context.Background()
			tc.assertErr(t, warningSvc.RemoveStale(ctx, tc.scope, tc.warnings))
			require.Empty(t, tc.repoUpdate)
			require.Empty(t, tc.repoDeleteByUUID)
		})
	}
}

func TestWarningService_UpdateStatusByUUID(t *testing.T) {
	testWarning := func() *migration.Warning {
		now := time.Now().UTC()
		w := migration.NewSyncWarning(api.InstanceImportFailed, "src", "msg")
		w.LastSeenDate = now
		w.FirstSeenDate = now
		w.UpdatedDate = now
		return &w
	}

	tests := []struct {
		name   string
		status api.WarningStatus

		repoGetByUUIDWarning *migration.Warning

		repoGetByUUIDErr error
		repoUpdateErr    error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:                 "success",
			status:               api.WARNINGSTATUS_ACKNOWLEDGED,
			repoGetByUUIDWarning: testWarning(),

			assertErr: require.NoError,
		},
		{
			name:             "error - repoGetByUUID",
			status:           api.WARNINGSTATUS_ACKNOWLEDGED,
			repoGetByUUIDErr: boom.Error,

			assertErr: boom.ErrorIs,
		},
		{
			name:                 "error - repoUpdate",
			status:               api.WARNINGSTATUS_ACKNOWLEDGED,
			repoGetByUUIDWarning: testWarning(),
			repoUpdateErr:        boom.Error,

			assertErr: boom.ErrorIs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mock.WarningRepoMock{
				GetByUUIDFunc: func(ctx context.Context, id uuid.UUID) (*migration.Warning, error) {
					return tc.repoGetByUUIDWarning, tc.repoGetByUUIDErr
				},
				UpdateFunc: func(ctx context.Context, id uuid.UUID, w migration.Warning) error {
					return tc.repoUpdateErr
				},
			}

			warningSvc := migration.NewWarningService(repo)

			// Perform test.
			ctx := context.Background()
			now := time.Now().UTC()
			w, err := warningSvc.UpdateStatusByUUID(ctx, uuid.Nil, tc.status)
			tc.assertErr(t, err)
			if err == nil {
				require.Equal(t, tc.repoGetByUUIDWarning.UpdatedDate, w.LastSeenDate)
				require.Equal(t, tc.status, w.Status)
				require.True(t, w.UpdatedDate.After(now))
			}
		})
	}
}
