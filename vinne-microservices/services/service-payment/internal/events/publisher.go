package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("payment-service/events")

// Publisher defines the interface for event publishing
type Publisher interface {
	PublishTransactionEvent(ctx context.Context, event *TransactionEvent) error
	PublishDepositEvent(ctx context.Context, event *DepositEvent) error
	PublishWithdrawalEvent(ctx context.Context, event *WithdrawalEvent) error
	PublishSagaEvent(ctx context.Context, event *SagaEvent) error
	PublishProviderEvent(ctx context.Context, event *ProviderEvent) error
	Close() error
}

// KafkaPublisher implements Publisher using Kafka
type KafkaPublisher struct {
	writer *kafka.Writer
}

// PublisherConfig holds Kafka publisher configuration
type PublisherConfig struct {
	Brokers []string
	Topic   string
}

// NewKafkaPublisher creates a new Kafka event publisher
func NewKafkaPublisher(config *PublisherConfig) *KafkaPublisher {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(config.Brokers...),
		Topic:        config.Topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		Async:        false, // Synchronous for reliability
	}

	return &KafkaPublisher{
		writer: writer,
	}
}

// PublishTransactionEvent publishes a transaction event
func (p *KafkaPublisher) PublishTransactionEvent(ctx context.Context, event *TransactionEvent) error {
	ctx, span := tracer.Start(ctx, "kafka_publisher.publish_transaction_event",
		trace.WithAttributes(
			attribute.String("event_type", string(event.EventType)),
			attribute.String("transaction_id", event.TransactionID),
			attribute.String("reference", event.Reference),
		))
	defer span.End()

	return p.publishEvent(ctx, event.EventType, event.Reference, event)
}

// PublishDepositEvent publishes a deposit event
func (p *KafkaPublisher) PublishDepositEvent(ctx context.Context, event *DepositEvent) error {
	ctx, span := tracer.Start(ctx, "kafka_publisher.publish_deposit_event",
		trace.WithAttributes(
			attribute.String("event_type", string(event.EventType)),
			attribute.String("transaction_id", event.TransactionID),
			attribute.String("reference", event.Reference),
		))
	defer span.End()

	return p.publishEvent(ctx, event.EventType, event.Reference, event)
}

// PublishWithdrawalEvent publishes a withdrawal event
func (p *KafkaPublisher) PublishWithdrawalEvent(ctx context.Context, event *WithdrawalEvent) error {
	ctx, span := tracer.Start(ctx, "kafka_publisher.publish_withdrawal_event",
		trace.WithAttributes(
			attribute.String("event_type", string(event.EventType)),
			attribute.String("transaction_id", event.TransactionID),
			attribute.String("reference", event.Reference),
		))
	defer span.End()

	return p.publishEvent(ctx, event.EventType, event.Reference, event)
}

// PublishSagaEvent publishes a saga event
func (p *KafkaPublisher) PublishSagaEvent(ctx context.Context, event *SagaEvent) error {
	ctx, span := tracer.Start(ctx, "kafka_publisher.publish_saga_event",
		trace.WithAttributes(
			attribute.String("event_type", string(event.EventType)),
			attribute.String("saga_id", event.SagaID),
			attribute.String("transaction_id", event.TransactionID),
		))
	defer span.End()

	return p.publishEvent(ctx, event.EventType, event.SagaID, event)
}

// PublishProviderEvent publishes a provider event
func (p *KafkaPublisher) PublishProviderEvent(ctx context.Context, event *ProviderEvent) error {
	ctx, span := tracer.Start(ctx, "kafka_publisher.publish_provider_event",
		trace.WithAttributes(
			attribute.String("event_type", string(event.EventType)),
			attribute.String("provider_name", event.ProviderName),
			attribute.String("operation", event.Operation),
		))
	defer span.End()

	// Use provider name as key for partition distribution
	key := event.ProviderName
	if event.Reference != "" {
		key = event.Reference
	}

	return p.publishEvent(ctx, event.EventType, key, event)
}

// publishEvent is the internal method that handles event serialization and publishing
func (p *KafkaPublisher) publishEvent(ctx context.Context, eventType EventType, key string, event interface{}) error {
	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create Kafka message
	message := kafka.Message{
		Key:   []byte(key),
		Value: data,
		Headers: []kafka.Header{
			{Key: "event-type", Value: []byte(eventType)},
			{Key: "source", Value: []byte("payment-service")},
			{Key: "timestamp", Value: []byte(time.Now().Format(time.RFC3339))},
		},
	}

	// Publish to Kafka
	startTime := time.Now()
	err = p.writer.WriteMessages(ctx, message)
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Add span attributes for observability
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetAttributes(
			attribute.Int("message_size", len(data)),
			attribute.Int64("publish_duration_ms", duration.Milliseconds()),
			attribute.Bool("published", true),
		)
	}

	return nil
}

// Close closes the Kafka writer
func (p *KafkaPublisher) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}

// AsyncPublisher wraps Publisher to publish events asynchronously (fire and forget)
type AsyncPublisher struct {
	publisher Publisher
	eventChan chan func()
	workers   int
}

// NewAsyncPublisher creates an async event publisher
func NewAsyncPublisher(publisher Publisher, workers int) *AsyncPublisher {
	ap := &AsyncPublisher{
		publisher: publisher,
		eventChan: make(chan func(), 1000), // Buffer 1000 events
		workers:   workers,
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		go ap.worker()
	}

	return ap
}

// worker processes events from the channel
func (ap *AsyncPublisher) worker() {
	for fn := range ap.eventChan {
		fn()
	}
}

// PublishTransactionEvent publishes asynchronously
func (ap *AsyncPublisher) PublishTransactionEvent(ctx context.Context, event *TransactionEvent) error {
	select {
	case ap.eventChan <- func() {
		_ = ap.publisher.PublishTransactionEvent(ctx, event)
	}:
		return nil
	default:
		// Channel full - publish synchronously to avoid data loss
		return ap.publisher.PublishTransactionEvent(ctx, event)
	}
}

// PublishDepositEvent publishes asynchronously
func (ap *AsyncPublisher) PublishDepositEvent(ctx context.Context, event *DepositEvent) error {
	select {
	case ap.eventChan <- func() {
		_ = ap.publisher.PublishDepositEvent(ctx, event)
	}:
		return nil
	default:
		return ap.publisher.PublishDepositEvent(ctx, event)
	}
}

// PublishWithdrawalEvent publishes asynchronously
func (ap *AsyncPublisher) PublishWithdrawalEvent(ctx context.Context, event *WithdrawalEvent) error {
	select {
	case ap.eventChan <- func() {
		_ = ap.publisher.PublishWithdrawalEvent(ctx, event)
	}:
		return nil
	default:
		return ap.publisher.PublishWithdrawalEvent(ctx, event)
	}
}

// PublishSagaEvent publishes asynchronously
func (ap *AsyncPublisher) PublishSagaEvent(ctx context.Context, event *SagaEvent) error {
	select {
	case ap.eventChan <- func() {
		_ = ap.publisher.PublishSagaEvent(ctx, event)
	}:
		return nil
	default:
		return ap.publisher.PublishSagaEvent(ctx, event)
	}
}

// PublishProviderEvent publishes asynchronously
func (ap *AsyncPublisher) PublishProviderEvent(ctx context.Context, event *ProviderEvent) error {
	select {
	case ap.eventChan <- func() {
		_ = ap.publisher.PublishProviderEvent(ctx, event)
	}:
		return nil
	default:
		return ap.publisher.PublishProviderEvent(ctx, event)
	}
}

// Close closes the async publisher
func (ap *AsyncPublisher) Close() error {
	close(ap.eventChan)
	return ap.publisher.Close()
}
