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
func (_d BatchRepoWithSlog) Create(ctx context.Context, batch _sourceMigration.Batch) (i1 int64, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.Any("batch", batch),
	).Debug("BatchRepoWithSlog: calling Create")
	defer func() {
		log := _d._log.With(
			slog.Int64("i1", i1),
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

// GetAllNamesByState implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) GetAllNamesByState(ctx context.Context, status api.BatchStatusType) (sa1 []string, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.Any("status", status),
	).Debug("BatchRepoWithSlog: calling GetAllNamesByState")
	defer func() {
		log := _d._log.With(
			slog.Any("sa1", sa1),
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method GetAllNamesByState returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method GetAllNamesByState finished")
		}
	}()
	return _d._base.GetAllNamesByState(ctx, status)
}

// GetByName implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) GetByName(ctx context.Context, name string) (bp1 *_sourceMigration.Batch, err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.String("name", name),
	).Debug("BatchRepoWithSlog: calling GetByName")
	defer func() {
		log := _d._log.With(
			slog.Any("bp1", bp1),
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

// Rename implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) Rename(ctx context.Context, oldName string, newName string) (err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.String("oldName", oldName),
		slog.String("newName", newName),
	).Debug("BatchRepoWithSlog: calling Rename")
	defer func() {
		log := _d._log.With(
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method Rename returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method Rename finished")
		}
	}()
	return _d._base.Rename(ctx, oldName, newName)
}

// Update implements _sourceMigration.BatchRepo
func (_d BatchRepoWithSlog) Update(ctx context.Context, batch _sourceMigration.Batch) (err error) {
	_d._log.With(
		slog.Any("ctx", ctx),
		slog.Any("batch", batch),
	).Debug("BatchRepoWithSlog: calling Update")
	defer func() {
		log := _d._log.With(
			slog.Any("err", err),
		)
		if err != nil {
			log.Error("BatchRepoWithSlog: method Update returned an error")
		} else {
			log.Debug("BatchRepoWithSlog: method Update finished")
		}
	}()
	return _d._base.Update(ctx, batch)
}
