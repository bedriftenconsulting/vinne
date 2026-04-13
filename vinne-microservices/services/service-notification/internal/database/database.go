package database

import (
	"context"
	"database/sql"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	instrumentationName = "github.com/randco/randco-microservices/services/service-notification/database"
)

type TracedDB struct {
	*sql.DB
	tracer trace.Tracer
}

func NewTracedDB(db *sql.DB) *TracedDB {
	return &TracedDB{
		DB:     db,
		tracer: otel.Tracer(instrumentationName),
	}
}

func NewTracedDBInterface(db *sql.DB) TracedDBInterface {
	return NewTracedDB(db)
}

func (db *TracedDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	ctx, span := db.tracer.Start(ctx, "db.query",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("query"),
			semconv.DBStatementKey.String(query),
			attribute.Int("db.args.count", len(args)),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	rows, err := db.DB.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("db.error", err.Error()))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return rows, err
}

func (db *TracedDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	ctx, span := db.tracer.Start(ctx, "db.query_row",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("query_row"),
			semconv.DBStatementKey.String(query),
			attribute.Int("db.args.count", len(args)),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	row := db.DB.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	span.SetStatus(codes.Ok, "")
	return row
}

func (db *TracedDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	ctx, span := db.tracer.Start(ctx, "db.exec",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("exec"),
			semconv.DBStatementKey.String(query),
			attribute.Int("db.args.count", len(args)),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()
	start := time.Now()
	result, err := db.DB.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("db.error", err.Error()))
	} else {
		span.SetStatus(codes.Ok, "")

		// Add result information if available
		if result != nil {
			if rowsAffected, err := result.RowsAffected(); err == nil {
				span.SetAttributes(attribute.Int64("db.rows_affected", rowsAffected))
			}
		}
	}

	return result, err
}

func (db *TracedDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (TracedTxInterface, error) {
	ctx, span := db.tracer.Start(ctx, "db.begin_tx",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("begin_tx"),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	tx, err := db.DB.BeginTx(ctx, opts)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &TracedTx{
		Tx:     tx,
		tracer: db.tracer,
	}, nil
}

type TracedTx struct {
	*sql.Tx
	tracer trace.Tracer
}

func (tx *TracedTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	ctx, span := tx.tracer.Start(ctx, "tx.query_row",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("tx_query_row"),
			semconv.DBStatementKey.String(query),
			attribute.Int("db.args.count", len(args)),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	rows, err := tx.Tx.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return rows, err
}

func (tx *TracedTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	ctx, span := tx.tracer.Start(ctx, "tx.query_row",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("tx_query_row"),
			semconv.DBStatementKey.String(query),
			attribute.Int("db.args.count", len(args)),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	row := tx.Tx.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	span.SetStatus(codes.Ok, "")
	return row
}

func (tx *TracedTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	ctx, span := tx.tracer.Start(ctx, "tx.exec",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("tx_exec"),
			semconv.DBStatementKey.String(query),
			attribute.Int("db.args.count", len(args)),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	result, err := tx.Tx.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")

		if result != nil {
			if rowsAffected, err := result.RowsAffected(); err == nil {
				span.SetAttributes(attribute.Int64("db.rows_affected", rowsAffected))
			}
		}
	}

	return result, err
}

func (tx *TracedTx) Commit() error {
	_, span := tx.tracer.Start(context.Background(), "tx.commit",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("tx_commit"),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	err := tx.Tx.Commit()
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return err
}

func (tx *TracedTx) Rollback() error {
	_, span := tx.tracer.Start(context.Background(), "tx.rollback",
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperationKey.String("tx_rollback"),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	err := tx.Tx.Rollback()
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int64("db.duration_ms", duration.Milliseconds()),
	)

	if err != nil && err != sql.ErrTxDone {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return err
}

type DBInterface interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type TxInterface interface {
	DBInterface
	Commit() error
	Rollback() error
}

type TracedDBInterface interface {
	DBInterface
	BeginTx(ctx context.Context, opts *sql.TxOptions) (TracedTxInterface, error)
}

type TracedTxInterface interface {
	TxInterface
}
