package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
)

// EventBus interface for publishing events
type EventBus interface {
	Publish(ctx context.Context, topic string, event Event) error
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
	Close() error
}

// EventHandler processes events
type EventHandler func(ctx context.Context, event *EventEnvelope) error

// KafkaEventBus implements EventBus using Kafka
type KafkaEventBus struct {
	producer      sarama.SyncProducer
	consumer      sarama.Consumer      // Legacy consumer (deprecated)
	consumerGroup sarama.ConsumerGroup // New consumer group (recommended)
	groupID       string
}

// NewKafkaEventBus creates a new Kafka event bus with legacy consumer (deprecated)
// Use NewKafkaEventBusWithGroup for new implementations
func NewKafkaEventBus(brokers []string) (EventBus, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Idempotent = true
	config.Net.MaxOpenRequests = 1 // Required for idempotent producer

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	consumer, err := sarama.NewConsumer(brokers, config)
	if err != nil {
		_ = producer.Close()
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	return &KafkaEventBus{
		producer: producer,
		consumer: consumer,
	}, nil
}

// NewKafkaEventBusWithGroup creates a new Kafka event bus with consumer group support
// This is the recommended constructor for production use as it provides:
// - Automatic offset management and commit
// - Consumer group coordination for multiple instances
// - Rebalancing support
// - No message skipping on restart
func NewKafkaEventBusWithGroup(brokers []string, groupID string) (EventBus, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0

	// Producer configuration
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Idempotent = true
	config.Net.MaxOpenRequests = 1

	// Consumer group configuration
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest // Start from latest offset to avoid processing old messages
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = 1000 // Commit every 1 second
	config.Consumer.Return.Errors = true

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		_ = producer.Close()
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	return &KafkaEventBus{
		producer:      producer,
		consumerGroup: consumerGroup,
		groupID:       groupID,
	}, nil
}

// Publish publishes an event to Kafka
func (eb *KafkaEventBus) Publish(ctx context.Context, topic string, event Event) error {
	envelope, err := NewEventEnvelope(topic, event.GetEventID(), event)
	if err != nil {
		return fmt.Errorf("failed to create envelope: %w", err)
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(event.GetEventID()),
		Value: sarama.ByteEncoder(payload),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("event_id"),
				Value: []byte(event.GetEventID()),
			},
			{
				Key:   []byte("event_type"),
				Value: []byte(string(event.GetEventType())),
			},
			{
				Key:   []byte("source"),
				Value: []byte(event.GetSource()),
			},
		},
	}

	_, _, err = eb.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	handler EventHandler
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		// Parse event envelope
		var envelope EventEnvelope
		envelope.Topic = message.Topic
		envelope.Key = string(message.Key)
		envelope.Headers = make(map[string]string)
		for _, header := range message.Headers {
			envelope.Headers[string(header.Key)] = string(header.Value)
		}

		if err := json.Unmarshal(message.Value, &envelope.Payload); err != nil {
			fmt.Printf("Failed to unmarshal event envelope: %v\n", err)
			// Mark message as processed even on unmarshal error to avoid getting stuck
			session.MarkMessage(message, "")
			continue
		}

		// Handle event
		if err := h.handler(session.Context(), &envelope); err != nil {
			fmt.Printf("Failed to handle event: %v\n", err)
			// Still mark as processed - idempotency will handle retries if needed
			// Don't block the entire consumer on one failed message
		}

		// Mark message as successfully processed (commits offset)
		session.MarkMessage(message, "")
	}
	return nil
}

// Subscribe subscribes to events from a topic
func (eb *KafkaEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) error {
	// Use consumer group if available (recommended path)
	if eb.consumerGroup != nil {
		return eb.subscribeWithConsumerGroup(ctx, topic, handler)
	}

	// Fall back to legacy consumer (deprecated)
	return eb.subscribeWithLegacyConsumer(ctx, topic, handler)
}

// subscribeWithConsumerGroup subscribes using consumer group (recommended)
func (eb *KafkaEventBus) subscribeWithConsumerGroup(ctx context.Context, topic string, handler EventHandler) error {
	groupHandler := &consumerGroupHandler{handler: handler}

	go func() {
		for {
			// Consume joins the consumer group and starts consuming
			// This call blocks until the context is cancelled or an error occurs
			if err := eb.consumerGroup.Consume(ctx, []string{topic}, groupHandler); err != nil {
				fmt.Printf("Consumer group error: %v\n", err)
			}

			// Check if context was cancelled
			if ctx.Err() != nil {
				return
			}

			// If we get here, the consumer group session ended but context is still active
			// This can happen during rebalancing - just continue to rejoin
		}
	}()

	// Monitor consumer group errors
	go func() {
		for err := range eb.consumerGroup.Errors() {
			fmt.Printf("Consumer group error: %v\n", err)
		}
	}()

	return nil
}

// subscribeWithLegacyConsumer subscribes using legacy consumer (deprecated)
func (eb *KafkaEventBus) subscribeWithLegacyConsumer(ctx context.Context, topic string, handler EventHandler) error {
	partitions, err := eb.consumer.Partitions(topic)
	if err != nil {
		return fmt.Errorf("failed to get partitions: %w", err)
	}

	for _, partition := range partitions {
		pc, err := eb.consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
		if err != nil {
			return fmt.Errorf("failed to consume partition %d: %w", partition, err)
		}

		go func(pc sarama.PartitionConsumer) {
			defer func() {
				_ = pc.Close()
			}()

			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-pc.Messages():
					var envelope EventEnvelope
					envelope.Topic = msg.Topic
					envelope.Key = string(msg.Key)
					envelope.Headers = make(map[string]string)
					for _, header := range msg.Headers {
						envelope.Headers[string(header.Key)] = string(header.Value)
					}
					if err := json.Unmarshal(msg.Value, &envelope.Payload); err != nil {
						fmt.Printf("Failed to unmarshal as EventEnvelope, treating as direct message: %v\n", err)
					}

					if err := handler(ctx, &envelope); err != nil {
						fmt.Printf("Failed to handle event: %v\n", err)
					}
				case err := <-pc.Errors():
					fmt.Printf("Consumer error: %v\n", err)
				}
			}
		}(pc)
	}

	return nil
}

// Close closes the event bus
func (eb *KafkaEventBus) Close() error {
	if err := eb.producer.Close(); err != nil {
		return err
	}

	// Close consumer group if present (recommended path)
	if eb.consumerGroup != nil {
		if err := eb.consumerGroup.Close(); err != nil {
			return err
		}
	}

	// Close legacy consumer if present (deprecated path)
	if eb.consumer != nil {
		if err := eb.consumer.Close(); err != nil {
			return err
		}
	}

	return nil
}

// InMemoryEventBus for testing
type InMemoryEventBus struct {
	events []Event
}

// NewInMemoryEventBus creates a new in-memory event bus
func NewInMemoryEventBus() EventBus {
	return &InMemoryEventBus{
		events: make([]Event, 0),
	}
}

// Publish stores the event in memory
func (eb *InMemoryEventBus) Publish(ctx context.Context, topic string, event Event) error {
	eb.events = append(eb.events, event)
	return nil
}

// Subscribe is not implemented for in-memory bus
func (eb *InMemoryEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) error {
	return nil
}

// Close does nothing for in-memory bus
func (eb *InMemoryEventBus) Close() error {
	return nil
}
