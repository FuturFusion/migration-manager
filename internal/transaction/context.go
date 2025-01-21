package transaction

import (
	"context"
	"database/sql"
	"errors"
)

type tcKey struct{}

type tx interface {
	DBTX
	Commit() error
	Rollback() error
}

type Transaction interface {
	Commit() error
	Rollback() error
}

func Begin(ctx context.Context) (context.Context, Transaction) {
	existingTC := ctx.Value(tcKey{})
	if existingTC != nil {
		return ctx, &noopTransactionContainer{}
	}

	tc := &transactionContainer{}
	return context.WithValue(ctx, tcKey{}, tc), tc
}

type transactionContainer struct {
	tx tx
}

var _ Transaction = &transactionContainer{}

func (t *transactionContainer) Commit() error {
	if t.tx == nil {
		return nil
	}

	return t.tx.Commit()
}

func (t *transactionContainer) Rollback() error {
	if t.tx == nil {
		return nil
	}

	err := t.tx.Rollback()
	if !errors.Is(err, sql.ErrTxDone) {
		return err
	}

	return nil
}

type noopTransactionContainer struct{}

var _ Transaction = noopTransactionContainer{}

func (n noopTransactionContainer) Commit() error {
	return nil
}

func (n noopTransactionContainer) Rollback() error {
	return nil
}
