package push

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/randco/randco-microservices/shared/common/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/api/option"
)

// FirebaseProvider implements push notification provider using Firebase Cloud Messaging
type FirebaseProvider struct {
	client *messaging.Client
	logger logger.Logger
}

// NewFirebaseProvider creates a new Firebase push notification provider
func NewFirebaseProvider(credentialsPath string, logger logger.Logger) (*FirebaseProvider, error) {
	ctx := context.Background()

	var app *firebase.App
	var err error

	if credentialsPath != "" {
		// Initialize with service account credentials
		opt := option.WithCredentialsFile(credentialsPath)
		app, err = firebase.NewApp(ctx, nil, opt)
	} else {
		// Initialize with default credentials (for production with ADC)
		app, err = firebase.NewApp(ctx, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firebase messaging client: %w", err)
	}

	logger.Info("Firebase provider initialized successfully")

	return &FirebaseProvider{
		client: client,
		logger: logger,
	}, nil
}

// SendPushNotification sends a push notification via Firebase Cloud Messaging
func (p *FirebaseProvider) SendPushNotification(ctx context.Context, req *PushNotificationRequest) (*PushNotificationResponse, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "provider.firebase.send_push")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "firebase"),
		attribute.String("device_token", req.DeviceToken[:20]+"..."), // Truncate for security
		attribute.String("title", req.Title),
	)

	// Build FCM message
	message := &messaging.Message{
		Token: req.DeviceToken,
		Notification: &messaging.Notification{
			Title: req.Title,
			Body:  req.Body,
		},
		Data: req.Data,
		Android: &messaging.AndroidConfig{
			Priority: p.getPriority(req.Priority),
			Notification: &messaging.AndroidNotification{
				ChannelID: "randlottery_notifications",
				Sound:     "default",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
				},
			},
		},
	}

	// Send message
	messageID, err := p.client.Send(ctx, message)
	if err != nil {
		span.RecordError(err)
		p.logger.Error("Failed to send FCM push notification",
			"error", err,
			"device_token", req.DeviceToken[:20]+"...",
			"title", req.Title,
		)
		return nil, fmt.Errorf("failed to send FCM message: %w", err)
	}

	p.logger.Info("Push notification sent successfully",
		"message_id", messageID,
		"device_token", req.DeviceToken[:20]+"...",
		"title", req.Title,
	)

	return &PushNotificationResponse{
		MessageID: messageID,
		Success:   true,
	}, nil
}

// SendBatchPushNotifications sends multiple push notifications via Firebase Cloud Messaging
func (p *FirebaseProvider) SendBatchPushNotifications(ctx context.Context, requests []*PushNotificationRequest) (*BatchPushNotificationResponse, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "provider.firebase.send_batch_push")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "firebase"),
		attribute.Int("batch_size", len(requests)),
	)

	// Build messages
	messages := make([]*messaging.Message, len(requests))
	for i, req := range requests {
		messages[i] = &messaging.Message{
			Token: req.DeviceToken,
			Notification: &messaging.Notification{
				Title: req.Title,
				Body:  req.Body,
			},
			Data: req.Data,
			Android: &messaging.AndroidConfig{
				Priority: p.getPriority(req.Priority),
				Notification: &messaging.AndroidNotification{
					ChannelID: "randlottery_notifications",
					Sound:     "default",
				},
			},
			APNS: &messaging.APNSConfig{
				Payload: &messaging.APNSPayload{
					Aps: &messaging.Aps{
						Sound: "default",
					},
				},
			},
		}
	}

	// Send batch using SendEach (SendAll is deprecated)
	batchResponse, err := p.client.SendEach(ctx, messages)
	if err != nil {
		span.RecordError(err)
		p.logger.Error("Failed to send batch FCM push notifications", "error", err, "batch_size", len(requests))
		return nil, fmt.Errorf("failed to send FCM batch: %w", err)
	}

	p.logger.Info("Batch push notifications sent",
		"total", len(requests),
		"success_count", batchResponse.SuccessCount,
		"failure_count", batchResponse.FailureCount,
	)

	// Process responses
	responses := make([]*PushNotificationResponse, len(batchResponse.Responses))
	for i, resp := range batchResponse.Responses {
		if resp.Success {
			responses[i] = &PushNotificationResponse{
				MessageID: resp.MessageID,
				Success:   true,
			}
		} else {
			responses[i] = &PushNotificationResponse{
				Success: false,
				Error:   resp.Error.Error(),
			}
		}
	}

	return &BatchPushNotificationResponse{
		Responses:    responses,
		SuccessCount: batchResponse.SuccessCount,
		FailureCount: batchResponse.FailureCount,
	}, nil
}

// getPriority converts string priority to FCM priority
func (p *FirebaseProvider) getPriority(priority string) string {
	switch priority {
	case "high", "critical":
		return "high"
	default:
		return "normal"
	}
}

// Close closes the Firebase provider (no-op for Firebase)
func (p *FirebaseProvider) Close() error {
	p.logger.Info("Firebase provider closed")
	return nil
}
