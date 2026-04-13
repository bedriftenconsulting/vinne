package events

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName     = "service-wallet"
	walletTopic     = "wallet-events"
	commissionTopic = "commission-events"
)

// Publisher handles event publishing for the wallet service
type Publisher struct {
	tracer trace.Tracer
}

// NewPublisher creates a new event publisher
func NewPublisher(brokers []string) (*Publisher, error) {
	return &Publisher{
		tracer: otel.Tracer(serviceName),
	}, nil
}

// Close closes the publisher
func (p *Publisher) Close() error {
	return nil
}
