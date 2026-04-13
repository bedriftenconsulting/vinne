package tracing

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TraceFunc is a simplified function for tracing any operation
// It automatically handles span creation, error recording, and status setting
func TraceFunc(ctx context.Context, name string, fn func(context.Context) error, attrs ...attribute.KeyValue) error {
	ctx, span := StartSpanFromContext(ctx, name)
	defer span.End()
	
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	
	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	
	return err
}

// TraceFuncWithResult is for operations that return a value along with error
func TraceFuncWithResult[T any](ctx context.Context, name string, fn func(context.Context) (T, error), attrs ...attribute.KeyValue) (T, error) {
	ctx, span := StartSpanFromContext(ctx, name)
	defer span.End()
	
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	
	result, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	
	return result, err
}

// DatabaseSpan creates a span specifically for database operations
type DatabaseSpan struct {
	span trace.Span
	ctx  context.Context
}

// TraceDB creates a new database operation span with common attributes
func TraceDB(ctx context.Context, operation, table string) *DatabaseSpan {
	spanName := fmt.Sprintf("db.%s.%s", table, operation)
	ctx, span := StartSpanFromContext(ctx, spanName)
	
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", operation),
		attribute.String("db.table", table),
	)
	
	return &DatabaseSpan{
		span: span,
		ctx:  ctx,
	}
}

// Context returns the context with the span
func (ds *DatabaseSpan) Context() context.Context {
	return ds.ctx
}

// SetQuery sets the SQL query attribute (sanitized)
func (ds *DatabaseSpan) SetQuery(query string) *DatabaseSpan {
	// You might want to sanitize the query here to remove sensitive data
	ds.span.SetAttributes(attribute.String("db.statement", query))
	return ds
}

// SetRowsAffected sets the number of rows affected
func (ds *DatabaseSpan) SetRowsAffected(count int64) *DatabaseSpan {
	ds.span.SetAttributes(attribute.Int64("db.rows_affected", count))
	return ds
}

// SetID sets an entity ID being operated on
func (ds *DatabaseSpan) SetID(id string) *DatabaseSpan {
	ds.span.SetAttributes(attribute.String("entity.id", id))
	return ds
}

// End completes the span with appropriate status based on error
func (ds *DatabaseSpan) End(err error) error {
	defer ds.span.End()
	
	if err != nil {
		if err == sql.ErrNoRows {
			ds.span.SetAttributes(attribute.Bool("db.found", false))
			ds.span.SetStatus(codes.Ok, "no rows found") // This is often expected
		} else {
			ds.span.RecordError(err)
			ds.span.SetStatus(codes.Error, err.Error())
		}
	} else {
		ds.span.SetAttributes(attribute.Bool("db.found", true))
		ds.span.SetStatus(codes.Ok, "")
	}
	
	return err
}

// CacheSpan creates a span specifically for cache operations
type CacheSpan struct {
	span trace.Span
	ctx  context.Context
}

// TraceCache creates a new cache operation span
func TraceCache(ctx context.Context, operation, key string) *CacheSpan {
	spanName := fmt.Sprintf("cache.%s", operation)
	ctx, span := StartSpanFromContext(ctx, spanName)
	
	span.SetAttributes(
		attribute.String("cache.operation", operation),
		attribute.String("cache.key", key),
	)
	
	return &CacheSpan{
		span: span,
		ctx:  ctx,
	}
}

// Context returns the context with the span
func (cs *CacheSpan) Context() context.Context {
	return cs.ctx
}

// SetHit marks whether the cache operation was a hit or miss
func (cs *CacheSpan) SetHit(hit bool) *CacheSpan {
	cs.span.SetAttributes(attribute.Bool("cache.hit", hit))
	return cs
}

// SetTTL sets the TTL for cache set operations
func (cs *CacheSpan) SetTTL(seconds int) *CacheSpan {
	cs.span.SetAttributes(attribute.Int("cache.ttl_seconds", seconds))
	return cs
}

// End completes the span
func (cs *CacheSpan) End(err error) error {
	defer cs.span.End()
	
	if err != nil {
		cs.span.RecordError(err)
		cs.span.SetStatus(codes.Error, err.Error())
	} else {
		cs.span.SetStatus(codes.Ok, "")
	}
	
	return err
}

// ServiceSpan creates a span specifically for service/business logic operations
type ServiceSpan struct {
	span trace.Span
	ctx  context.Context
}

// TraceService creates a new service operation span
func TraceService(ctx context.Context, service, operation string) *ServiceSpan {
	spanName := fmt.Sprintf("service.%s.%s", service, operation)
	ctx, span := StartSpanFromContext(ctx, spanName)
	
	span.SetAttributes(
		attribute.String("service.name", service),
		attribute.String("service.operation", operation),
	)
	
	return &ServiceSpan{
		span: span,
		ctx:  ctx,
	}
}

// Context returns the context with the span
func (ss *ServiceSpan) Context() context.Context {
	return ss.ctx
}

// SetUser sets user information
func (ss *ServiceSpan) SetUser(userID, email, role string) *ServiceSpan {
	attrs := []attribute.KeyValue{}
	if userID != "" {
		attrs = append(attrs, attribute.String("user.id", userID))
	}
	if email != "" {
		attrs = append(attrs, attribute.String("user.email", email))
	}
	if role != "" {
		attrs = append(attrs, attribute.String("user.role", role))
	}
	ss.span.SetAttributes(attrs...)
	return ss
}

// SetRequestID sets a request ID for correlation
func (ss *ServiceSpan) SetRequestID(requestID string) *ServiceSpan {
	ss.span.SetAttributes(attribute.String("request.id", requestID))
	return ss
}

// AddEvent adds a significant event to the span
func (ss *ServiceSpan) AddEvent(name string, attrs ...attribute.KeyValue) *ServiceSpan {
	ss.span.AddEvent(name, trace.WithAttributes(attrs...))
	return ss
}

// End completes the span
func (ss *ServiceSpan) End(err error) error {
	defer ss.span.End()
	
	if err != nil {
		ss.span.RecordError(err)
		ss.span.SetStatus(codes.Error, err.Error())
	} else {
		ss.span.SetStatus(codes.Ok, "")
	}
	
	return err
}

// EndWithResult completes the span and returns a result with error
func (ss *ServiceSpan) EndWithResult(result interface{}, err error) (interface{}, error) {
	defer ss.span.End()
	
	if err != nil {
		ss.span.RecordError(err)
		ss.span.SetStatus(codes.Error, err.Error())
	} else {
		ss.span.SetStatus(codes.Ok, "")
	}
	
	return result, err
}