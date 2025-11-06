package migration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/internal/migration/repo/mock"
	"github.com/FuturFusion/migration-manager/internal/testing/boom"
	"github.com/FuturFusion/migration-manager/internal/testing/queue"
	"github.com/FuturFusion/migration-manager/shared/api"
)

func TestWindowService_DeleteByNameAndBatch(t *testing.T) {
	type q struct {
		batch  string
		window string
	}

	cases := []struct {
		name          string
		window        string
		batch         string
		queue         []q
		repoDeleteErr error
		queueGetErr   error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "success - no queue entries",
			window:    "w1",
			batch:     "b1",
			assertErr: require.NoError,
		},
		{
			name:      "success - queue entries map to another window",
			window:    "w1",
			batch:     "b1",
			queue:     []q{{batch: "b1", window: "w2"}},
			assertErr: require.NoError,
		},
		{
			name:      "success - matching queue entry for another batch (duplicate window names across batches)",
			window:    "w1",
			batch:     "b1",
			queue:     []q{{batch: "b1", window: "w2"}, {batch: "b2", window: "w1"}},
			assertErr: require.NoError,
		},
		{
			name:      "error - window in use by queue entry",
			window:    "w1",
			batch:     "b1",
			queue:     []q{{batch: "b1", window: "w2"}, {batch: "b2", window: "w1"}, {batch: "b1", window: "w1"}},
			assertErr: require.Error,
		},
		{
			name:        "error - queue.GetAllByBatch",
			window:      "w1",
			batch:       "b1",
			queueGetErr: boom.Error,
			assertErr:   boom.ErrorIs,
		},
		{
			name:          "error - repo.DeleteByNameAndBatch",
			window:        "w1",
			batch:         "b1",
			repoDeleteErr: boom.Error,
			assertErr:     boom.ErrorIs,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			repo := &mock.WindowRepoMock{
				DeleteByNameAndBatchFunc: func(ctx context.Context, name string, batchName string) error {
					return tc.repoDeleteErr
				},
			}

			queueSvc := &QueueServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.QueueEntries, error) {
					if tc.queueGetErr != nil {
						return nil, tc.queueGetErr
					}

					entries := migration.QueueEntries{}
					for _, q := range tc.queue {
						if q.batch == tc.batch {
							entries = append(entries, migration.QueueEntry{BatchName: q.batch, MigrationWindowName: sql.NullString{Valid: q.window != "", String: q.window}})
						}
					}

					return entries, nil
				},
			}

			windowSvc := migration.NewWindowService(repo)

			tc.assertErr(t, windowSvc.DeleteByNameAndBatch(context.Background(), queueSvc, tc.window, tc.batch))
		})
	}
}

func TestWindowService_ReplaceByBatch(t *testing.T) {
	type window struct {
		n string
		s int
		e int
		c int
	}

	toWindows := func(ws []window, t time.Time, batch string) migration.Windows {
		windows := make([]migration.Window, len(ws))
		for i, w := range ws {
			windows[i] = migration.Window{
				ID:    int64(i),
				Name:  w.n,
				Batch: batch,
				Start: t.Add(time.Duration(w.s) * time.Minute),
				End:   t.Add(time.Duration(w.e) * time.Minute),
				Config: api.MigrationWindowConfig{
					Capacity: w.c,
				},
			}
		}

		return windows
	}

	type item = queue.Item[string]

	cases := []struct {
		name  string
		batch string

		queueWindows []string
		oldWindows   []window
		newWindows   []window

		removedWindows []item
		updatedWindows []item
		createdWindows []item

		repoGetErr    error
		repoDeleteErr error
		repoCreateErr error
		repoUpdateErr error
		queueGetErr   error

		assertErr require.ErrorAssertionFunc
	}{
		{
			name:           "success - only create, no queue entries",
			batch:          "b1",
			oldWindows:     []window{},
			newWindows:     []window{{n: "w1", s: 10, e: 20}},
			createdWindows: []item{{Value: "w1"}},
			assertErr:      require.NoError,
		},
		{
			name:           "success - only create, unassigned queue entries",
			batch:          "b1",
			queueWindows:   []string{""},
			oldWindows:     []window{},
			newWindows:     []window{{n: "w1", s: 10, e: 20}},
			createdWindows: []item{{Value: "w1"}},
			assertErr:      require.NoError,
		},
		{
			name:           "success - create, update, delete",
			batch:          "b1",
			queueWindows:   []string{""},
			oldWindows:     []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:     []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 20, e: 30}, {n: "w4", s: 4, e: 5}},
			createdWindows: []item{{Value: "w4"}},
			updatedWindows: []item{{Value: "w2"}},
			removedWindows: []item{{Value: "w3"}},
			assertErr:      require.NoError,
		},
		{
			name:           "success - create, update, delete (queue entry window untouched)",
			batch:          "b1",
			queueWindows:   []string{"w1"},
			oldWindows:     []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:     []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 20, e: 30}, {n: "w4", s: 4, e: 5}},
			createdWindows: []item{{Value: "w4"}},
			updatedWindows: []item{{Value: "w2"}},
			removedWindows: []item{{Value: "w3"}},
			assertErr:      require.NoError,
		},
		{
			name:           "success - create, update, delete (queue entry windows made more permissive)",
			batch:          "b1",
			queueWindows:   []string{"w1", "w2"},
			oldWindows:     []window{{n: "w1", s: 10, e: 20, c: 10}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}, {n: "w5", s: 5, e: 6}},
			newWindows:     []window{{n: "w1", s: 9, e: 21, c: 11}, {n: "w2", s: 2, e: 4}, {n: "w4", s: 4, e: 5}, {n: "w5", s: 5, e: 6}},
			createdWindows: []item{{Value: "w4"}},
			updatedWindows: []item{{Value: "w1"}, {Value: "w2"}},
			removedWindows: []item{{Value: "w3"}},
			assertErr:      require.NoError,
		},
		{
			name:         "success - no changes",
			batch:        "b1",
			queueWindows: []string{"w1"},
			oldWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			assertErr:    require.NoError,
		},
		{
			name:         "error - create, update, delete (queue entry window shrink start time)",
			batch:        "b1",
			queueWindows: []string{"w3"},
			oldWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w4", s: 4, e: 5}},
			assertErr:    require.Error,
		},
		{
			name:         "error - create, update, delete (queue entry window shrink end time)",
			batch:        "b1",
			queueWindows: []string{"w3"},
			oldWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 5}},
			newWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w4", s: 3, e: 4}},
			assertErr:    require.Error,
		},
		{
			name:         "error - create, update, delete (queue entry window shrink capacity)",
			batch:        "b1",
			queueWindows: []string{"w3"},
			oldWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4, c: 10}},
			newWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w4", s: 3, e: 4, c: 9}},
			assertErr:    require.Error,
		},
		{
			name:         "error - create, update, delete (queue entry window remove attempt)",
			batch:        "b1",
			queueWindows: []string{"w3"},
			oldWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 20, e: 30}, {n: "w4", s: 4, e: 5}},
			assertErr:    require.Error,
		},
		{
			name:       "error - new windows not valid",
			batch:      "b1",
			newWindows: []window{{n: "w1", s: 1, e: 0}, {n: "", s: 2, e: 3}},
			assertErr:  require.Error,
		},
		{
			name:         "error - queueSvc.GetAllByBatch",
			batch:        "b1",
			queueWindows: []string{""},
			oldWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 20, e: 30}, {n: "w4", s: 4, e: 5}},
			queueGetErr:  boom.Error,
			assertErr:    boom.ErrorIs,
		},
		{
			name:         "error - repo.GetAllByBatch",
			batch:        "b1",
			queueWindows: []string{""},
			oldWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:   []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 20, e: 30}, {n: "w4", s: 4, e: 5}},
			repoGetErr:   boom.Error,
			assertErr:    boom.ErrorIs,
		},
		{
			name:           "error - repo.Create",
			batch:          "b1",
			queueWindows:   []string{""},
			oldWindows:     []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:     []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 20, e: 30}, {n: "w4", s: 4, e: 5}},
			updatedWindows: []item{{Value: "w2"}}, // w2 is encountered before w4.
			removedWindows: []item{{Value: "w3"}}, // w3 is encountered before w4.
			repoCreateErr:  boom.Error,
			assertErr:      boom.ErrorIs,
		},
		{
			name:          "error - repo.Update",
			batch:         "b1",
			queueWindows:  []string{""},
			oldWindows:    []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:    []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 20, e: 30}, {n: "w4", s: 4, e: 5}},
			repoUpdateErr: boom.Error,
			assertErr:     boom.ErrorIs,
		},
		{
			name:           "error - repo.DeleteByNameAndBatch",
			batch:          "b1",
			queueWindows:   []string{""},
			oldWindows:     []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 2, e: 3}, {n: "w3", s: 3, e: 4}},
			newWindows:     []window{{n: "w1", s: 1, e: 2}, {n: "w2", s: 20, e: 30}, {n: "w4", s: 4, e: 5}},
			updatedWindows: []item{{Value: "w2"}}, // w2 is encountered before w3.
			repoDeleteErr:  boom.Error,
			assertErr:      boom.ErrorIs,
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("\n\nTEST %02d: %s\n\n", i, tc.name)
			now := time.Now().UTC()
			repo := &mock.WindowRepoMock{
				GetAllByBatchFunc: func(ctx context.Context, batchName string) (migration.Windows, error) {
					return toWindows(tc.oldWindows, now, tc.batch), tc.repoGetErr
				},
				DeleteByNameAndBatchFunc: func(ctx context.Context, name string, batchName string) error {
					if tc.repoDeleteErr != nil {
						return tc.repoDeleteErr
					}

					w, err := queue.Pop(t, &tc.removedWindows)
					require.NoError(t, err)
					require.Equal(t, name, w)
					return nil
				},
				CreateFunc: func(ctx context.Context, window migration.Window) (int64, error) {
					if tc.repoCreateErr != nil {
						return -1, tc.repoCreateErr
					}

					w, err := queue.Pop(t, &tc.createdWindows)
					require.NoError(t, err)
					require.Equal(t, window.Name, w)
					return 1, nil
				},
				UpdateFunc: func(ctx context.Context, window migration.Window) error {
					if tc.repoUpdateErr != nil {
						return tc.repoUpdateErr
					}

					w, err := queue.Pop(t, &tc.updatedWindows)
					require.NoError(t, err)
					require.Equal(t, window.Name, w)
					return nil
				},
			}

			queueSvc := &QueueServiceMock{
				GetAllByBatchFunc: func(ctx context.Context, batch string) (migration.QueueEntries, error) {
					if tc.queueGetErr != nil {
						return nil, tc.queueGetErr
					}

					entries := migration.QueueEntries{}
					for _, w := range tc.queueWindows {
						entries = append(entries, migration.QueueEntry{BatchName: tc.batch, MigrationWindowName: sql.NullString{Valid: w != "", String: w}})
					}

					return entries, nil
				},
			}

			windowSvc := migration.NewWindowService(repo)
			tc.assertErr(t, windowSvc.ReplaceByBatch(context.Background(), queueSvc, tc.batch, toWindows(tc.newWindows, now, tc.batch)))
		})
	}
}
