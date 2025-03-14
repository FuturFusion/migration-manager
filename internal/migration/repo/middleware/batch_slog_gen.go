// Code generated by gowrap. DO NOT EDIT.
// template: ../../../logger/slog.gotmpl
// gowrap: http://github.com/hexdigest/gowrap

package middleware

import (
	"context"
	"log/slog"

	_sourceMigration "github.com/FuturFusion/migration-manager/internal/migration"
	"github.com/FuturFusion/migration-manager/shared/api"
)

// BatchRepoWithSlog implements _sourceMigration.BatchRepo that is instrumented with slog logger
type BatchRepoWithSlog struct {
	_log  *slog.Logger
	_base _sourceMigration.BatchRepo
}

// NewBatchRepoWithSlog instruments an implementation of the _sourceMigration.BatchRepo with simple logging
func NewBatchRepoWithSlog(base _sourceMigration.BatchRepo, log *slog.Logger) BatchRepoWithSlog {
	return BatchRepoWithSlog{
		_base: base,
		_log:  log,
	}
}

// Create implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) Create(ctx context.Context, batch _sourceMigration.Batch) (b1 _sourceMigration.Batch, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.Any("batch", batch),
	).Debug("BatchRepoWithSlog: calling Create")
	defer func() {
		log := _d._log.With(
			slog.Any("b1", b1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method Create returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method Create finished")
		}
	}()
	return _d._base.Create(ctx, batch)
}

// DeleteByName implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) DeleteByName(ctx context.Context, name string) (err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.String("name", name),
	).Debug("BatchRepoWithSlog: calling DeleteByName")
	defer func() {
		log := _d._log.With(
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method DeleteByName returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method DeleteByName finished")
		}
	}()
	return _d._base.DeleteByName(ctx, name)
}

// GetAll implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) GetAll(ctx context.Context) (b1 _sourceMigration.Batches, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
	).Debug("BatchRepoWithSlog: calling GetAll")
	defer func() {
		log := _d._log.With(
			slog.Any("b1", b1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method GetAll returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method GetAll finished")
		}
	}()
	return _d._base.GetAll(ctx)
}

// GetAllByState implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) GetAllByState(ctx context.Context, status api.BatchStatusType) (b1 _sourceMigration.Batches, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.Any("status", status),
	).Debug("BatchRepoWithSlog: calling GetAllByState")
	defer func() {
		log := _d._log.With(
			slog.Any("b1", b1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method GetAllByState returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method GetAllByState finished")
		}
	}()
	return _d._base.GetAllByState(ctx, status)
}

// GetAllNames implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) GetAllNames(ctx context.Context) (sa1 []string, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
	).Debug("BatchRepoWithSlog: calling GetAllNames")
	defer func() {
		log := _d._log.With(
			slog.Any("sa1", sa1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method GetAllNames returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method GetAllNames finished")
		}
	}()
	return _d._base.GetAllNames(ctx)
}

// GetByID implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) GetByID(ctx context.Context, id int) (b1 _sourceMigration.Batch, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.Int("id", id),
	).Debug("BatchRepoWithSlog: calling GetByID")
	defer func() {
		log := _d._log.With(
			slog.Any("b1", b1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method GetByID returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method GetByID finished")
		}
	}()
	return _d._base.GetByID(ctx, id)
}

// GetByName implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) GetByName(ctx context.Context, name string) (b1 _sourceMigration.Batch, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.String("name", name),
	).Debug("BatchRepoWithSlog: calling GetByName")
	defer func() {
		log := _d._log.With(
			slog.Any("b1", b1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method GetByName returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method GetByName finished")
		}
	}()
	return _d._base.GetByName(ctx, name)
}

// UpdateByID implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) UpdateByID(ctx context.Context, batch _sourceMigration.Batch) (b1 _sourceMigration.Batch, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.Any("batch", batch),
	).Debug("BatchRepoWithSlog: calling UpdateByID")
	defer func() {
		log := _d._log.With(
			slog.Any("b1", b1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method UpdateByID returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method UpdateByID finished")
		}
	}()
	return _d._base.UpdateByID(ctx, batch)
}

// UpdateStatusByID implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) UpdateStatusByID(ctx context.Context, id int, status api.BatchStatusType, statusString string) (b1 _sourceMigration.Batch, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.Int("id", id),
		slog.Any("status", status),
		slog.String("statusString", statusString),
	).Debug("BatchRepoWithSlog: calling UpdateStatusByID")
	defer func() {
		log := _d._log.With(
			slog.Any("b1", b1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method UpdateStatusByID returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method UpdateStatusByID finished")
		}
	}()
	return _d._base.UpdateStatusByID(ctx, id, status, statusString)
}
