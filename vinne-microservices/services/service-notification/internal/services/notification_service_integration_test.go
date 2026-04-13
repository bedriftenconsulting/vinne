package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type NotificationServiceIntegrationTestSuite struct {
	suite.Suite
	pgContainer *postgres.PostgresContainer
	db          *sql.DB
	repo        repositories.NotificationRepository
	service     NotificationService
	ctx         context.Context
	cancel      context.CancelFunc
}

func (suite *NotificationServiceIntegrationTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())

	pgContainer, err := postgres.Run(suite.ctx,
		"postgres:17",
		postgres.WithDatabase("notification_service_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Minute),
		),
	)
	suite.Require().NoError(err)
	suite.pgContainer = pgContainer

	connStr, err := pgContainer.ConnectionString(suite.ctx, "sslmode=disable")
	suite.Require().NoError(err)

	suite.db, err = sql.Open("postgres", connStr)
	suite.Require().NoError(err)
	suite.Require().NoError(suite.db.Ping())

	err = suite.runMigrations(connStr)
	suite.Require().NoError(err)

	suite.repo = repositories.NewNotificationRepository(suite.db)
	testLogger := logger.NewLogger(logger.Config{
		Level:       "info",
		Format:      "text",
		ServiceName: "test",
	})
	suite.service = NewNotificationService(suite.repo, testLogger)
}

func (suite *NotificationServiceIntegrationTestSuite) TearDownSuite() {
	if suite.db != nil {
		_ = suite.db.Close()
	}

	if suite.pgContainer != nil {
		_ = suite.pgContainer.Terminate(suite.ctx)
	}

	suite.cancel()
}

func (suite *NotificationServiceIntegrationTestSuite) SetupTest() {
	suite.cleanupDatabase()
}

func (suite *NotificationServiceIntegrationTestSuite) runMigrations(connStr string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	migrationsDir := ""
	for i := 0; i < 10; i++ {
		testDir := filepath.Join(wd, "migrations")
		if _, err := os.Stat(testDir); err == nil {
			migrationsDir = testDir
			break
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}

	if migrationsDir == "" {
		return fmt.Errorf("migrations directory not found")
	}

	cmd := exec.Command("goose", "-dir", migrationsDir, "postgres", connStr, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (suite *NotificationServiceIntegrationTestSuite) cleanupDatabase() {
	tables := []string{
		"notification_events",
		"recipients",
		"notifications",
	}

	for _, table := range tables {
		_, err := suite.db.ExecContext(suite.ctx, "DELETE FROM "+table)
		suite.Require().NoError(err)
	}
}

func (suite *NotificationServiceIntegrationTestSuite) TestCreateNotificationIntegration() {
	req := models.CreateNotificationRequest{
		IdempotencyKey: "test-key-123",
		Type:           models.NotificationTypeEmail,
		Subject:        "Integration Test Email",
		Content:        "This is a test email content",
		Provider:       "sendgrid",
		Recipients: []models.CreateRecipientRequest{
			{Address: "test@example.com"},
		},
		CC: []models.CreateRecipientRequest{
			{Address: "cc@example.com"},
		},
		BCC: []models.CreateRecipientRequest{
			{Address: "bcc@example.com"},
		},
	}

	notification, err := suite.service.CreateNotification(suite.ctx, req)
	suite.Require().NoError(err)
	suite.NotEmpty(notification.ID)
	suite.Equal(req.IdempotencyKey, notification.IdempotencyKey)
	suite.Equal(req.Type, notification.Type)
	suite.Equal(req.Subject, notification.Subject)
	suite.Equal(req.Content, notification.Content)
	suite.Equal(models.NotificationStatusQueued, notification.Status)
	suite.NotZero(notification.CreatedAt)
	suite.NotZero(notification.UpdatedAt)

	retrieved, err := suite.service.GetNotification(suite.ctx, notification.ID)
	suite.Require().NoError(err)
	suite.Equal(notification.ID, retrieved.ID)
	suite.Equal(notification.Type, retrieved.Type)
	suite.Equal(notification.Status, retrieved.Status)
}

func (suite *NotificationServiceIntegrationTestSuite) TestIdempotencyIntegration() {
	req := models.CreateNotificationRequest{
		IdempotencyKey: "duplicate-test-key",
		Type:           models.NotificationTypeSMS,
		Content:        "Test SMS content",
		Provider:       "twilio",
		Recipients: []models.CreateRecipientRequest{
			{Address: "+1234567890"},
		},
	}

	notification1, err := suite.service.CreateNotification(suite.ctx, req)
	suite.Require().NoError(err)

	notification2, err := suite.service.CreateNotification(suite.ctx, req)
	suite.Require().NoError(err)

	suite.Equal(notification1.ID, notification2.ID)
	suite.Equal(notification1.CreatedAt, notification2.CreatedAt)
}

func (suite *NotificationServiceIntegrationTestSuite) TestUpdateNotificationIntegration() {
	req := models.CreateNotificationRequest{
		IdempotencyKey: "update-test-1",
		Type:           models.NotificationTypeEmail,
		Subject:        "Original Subject",
		Content:        "Original Content",
		Provider:       "ses",
		Recipients: []models.CreateRecipientRequest{
			{Address: "update@example.com"},
		},
	}

	notification, err := suite.service.CreateNotification(suite.ctx, req)
	suite.Require().NoError(err)

	newSubject := "Updated Subject"
	newContent := "Updated Content"
	newStatus := models.NotificationStatusSent

	updateReq := models.UpdateNotificationRequest{
		ID:      notification.ID,
		Subject: &newSubject,
		Content: &newContent,
		Status:  &newStatus,
	}

	updated, err := suite.service.UpdateNotification(suite.ctx, updateReq)
	suite.Require().NoError(err)
	suite.Equal(newSubject, updated.Subject)
	suite.Equal(newContent, updated.Content)
	suite.Equal(newStatus, updated.Status)
	suite.True(updated.UpdatedAt.After(updated.CreatedAt))
}

func (suite *NotificationServiceIntegrationTestSuite) TestListNotificationsIntegration() {
	notifications := []models.CreateNotificationRequest{
		{
			IdempotencyKey: "list-test-email-1",
			Type:           models.NotificationTypeEmail,
			Subject:        "Email 1",
			Content:        "Content 1",
			Provider:       "sendgrid",
			Recipients:     []models.CreateRecipientRequest{{Address: "email1@example.com"}},
		},
		{
			IdempotencyKey: "list-test-sms-1",
			Type:           models.NotificationTypeSMS,
			Content:        "SMS Content",
			Provider:       "twilio",
			Recipients:     []models.CreateRecipientRequest{{Address: "+1111111111"}},
		},
		{
			IdempotencyKey: "list-test-push-1",
			Type:           models.NotificationTypePush,
			Subject:        "Push Notification",
			Content:        "Push Content",
			Provider:       "fcm",
			Recipients:     []models.CreateRecipientRequest{{Address: "device-token-123"}},
		},
	}

	for _, req := range notifications {
		_, err := suite.service.CreateNotification(suite.ctx, req)
		suite.Require().NoError(err)
	}

	filter := models.NotificationFilter{}
	result, total, err := suite.service.ListNotifications(suite.ctx, filter, 1, 10)
	suite.Require().NoError(err)
	suite.Equal(int64(3), total)
	suite.Len(result, 3)

	emailFilter := models.NotificationFilter{
		Type: models.NotificationTypeEmail,
	}
	emailResult, emailTotal, err := suite.service.ListNotifications(suite.ctx, emailFilter, 1, 10)
	suite.Require().NoError(err)
	suite.Equal(int64(1), emailTotal)
	suite.Len(emailResult, 1)
	suite.Equal(models.NotificationTypeEmail, emailResult[0].Type)
}

func (suite *NotificationServiceIntegrationTestSuite) TestStatusTransitionsIntegration() {
	req := models.CreateNotificationRequest{
		IdempotencyKey: "status-transition-test-1",
		Type:           models.NotificationTypeEmail,
		Subject:        "Status Test",
		Content:        "Testing status transitions",
		Provider:       "sendgrid",
		Recipients: []models.CreateRecipientRequest{
			{Address: "status@example.com"},
		},
	}

	notification, err := suite.service.CreateNotification(suite.ctx, req)
	suite.Require().NoError(err)
	suite.Equal(models.NotificationStatusQueued, notification.Status)

	err = suite.service.MarkAsSent(suite.ctx, notification.ID, "provider-123", "provider-123", map[string]interface{}{"response": "success"})
	suite.Require().NoError(err)

	sent, err := suite.service.GetNotification(suite.ctx, notification.ID)
	suite.Require().NoError(err)
	suite.Equal(models.NotificationStatusSent, sent.Status)
	suite.NotNil(sent.ProviderMessageID)
	suite.Equal("provider-123", *sent.ProviderMessageID)
	suite.NotNil(sent.SentAt)

	err = suite.service.MarkAsDelivered(suite.ctx, notification.ID)
	suite.Require().NoError(err)

	delivered, err := suite.service.GetNotification(suite.ctx, notification.ID)
	suite.Require().NoError(err)
	suite.Equal(models.NotificationStatusDelivered, delivered.Status)
	suite.NotNil(delivered.DeliveredAt)
}

func (suite *NotificationServiceIntegrationTestSuite) TestFailureAndRetryIntegration() {
	req := models.CreateNotificationRequest{
		IdempotencyKey: "failure-retry-test-1",
		Type:           models.NotificationTypeSMS,
		Content:        "Retry test SMS",
		Provider:       "twilio",
		Recipients: []models.CreateRecipientRequest{
			{Address: "+1234567890"},
		},
	}

	notification, err := suite.service.CreateNotification(suite.ctx, req)
	suite.Require().NoError(err)

	err = suite.service.MarkAsFailed(suite.ctx, notification.ID, "Provider timeout")
	suite.Require().NoError(err)

	failed, err := suite.service.GetNotification(suite.ctx, notification.ID)
	suite.Require().NoError(err)
	suite.Equal(models.NotificationStatusFailed, failed.Status)
	suite.NotNil(failed.ErrorMessage)
	suite.Equal("Provider timeout", *failed.ErrorMessage)
	suite.Equal(int8(1), failed.RetryCount)
	suite.NotNil(failed.FailedAt)

	_, err = suite.service.RetryNotification(suite.ctx, notification.ID)
	suite.Require().NoError(err)

	retried, err := suite.service.GetNotification(suite.ctx, notification.ID)
	suite.Require().NoError(err)
	suite.Equal(models.NotificationStatusQueued, retried.Status)
	suite.Nil(retried.ErrorMessage)
	suite.Nil(retried.FailedAt)
	suite.Equal(int8(1), retried.RetryCount)
}

func (suite *NotificationServiceIntegrationTestSuite) TestGetNotificationsByStatusIntegration() {
	requests := []models.CreateNotificationRequest{
		{
			IdempotencyKey: "status-test-queued-1",
			Type:           models.NotificationTypeEmail,
			Content:        "Queued email",
			Provider:       "sendgrid",
			Recipients:     []models.CreateRecipientRequest{{Address: "queued@example.com"}},
		},
		{
			IdempotencyKey: "status-test-sent-1",
			Type:           models.NotificationTypeSMS,
			Content:        "Sent SMS",
			Provider:       "twilio",
			Recipients:     []models.CreateRecipientRequest{{Address: "+1111111111"}},
		},
	}

	createdNotifications := make([]*models.Notification, len(requests))
	for i, req := range requests {
		notification, err := suite.service.CreateNotification(suite.ctx, req)
		suite.Require().NoError(err)
		createdNotifications[i] = notification
	}

	err := suite.service.MarkAsSent(suite.ctx, createdNotifications[1].ID, "sent-123", "sent-123", nil)
	suite.Require().NoError(err)

	queuedNotifications, err := suite.service.GetNotificationsByStatus(suite.ctx, models.NotificationStatusQueued)
	suite.Require().NoError(err)
	suite.Len(queuedNotifications, 1)
	suite.Equal(createdNotifications[0].ID, queuedNotifications[0].ID)

	sentNotifications, err := suite.service.GetNotificationsByStatus(suite.ctx, models.NotificationStatusSent)
	suite.Require().NoError(err)
	suite.Len(sentNotifications, 1)
	suite.Equal(createdNotifications[1].ID, sentNotifications[0].ID)
}

func (suite *NotificationServiceIntegrationTestSuite) TestDeleteNotificationIntegration() {
	req := models.CreateNotificationRequest{
		IdempotencyKey: "delete-test-1",
		Type:           models.NotificationTypePush,
		Content:        "Delete test push",
		Provider:       "fcm",
		Recipients: []models.CreateRecipientRequest{
			{Address: "delete-token"},
		},
	}

	notification, err := suite.service.CreateNotification(suite.ctx, req)
	suite.Require().NoError(err)

	err = suite.service.DeleteNotification(suite.ctx, notification.ID)
	suite.Require().NoError(err)

	_, err = suite.service.GetNotification(suite.ctx, notification.ID)
	suite.Error(err)
}

func (suite *NotificationServiceIntegrationTestSuite) TestConcurrentNotificationOperations() {
	const numWorkers = 5
	const notificationsPerWorker = 3

	results := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer func() {
				if r := recover(); r != nil {
					results <- fmt.Errorf("worker %d panicked: %v", workerID, r)
					return
				}
			}()

			for j := 0; j < notificationsPerWorker; j++ {
				req := models.CreateNotificationRequest{
					IdempotencyKey: fmt.Sprintf("conc-%d-%d", workerID, j),
					Type:           models.NotificationTypeEmail,
					Subject:        fmt.Sprintf("Concurrent Email %d-%d", workerID, j),
					Content:        fmt.Sprintf("Content from worker %d", workerID),
					Provider:       "sendgrid",
					Recipients: []models.CreateRecipientRequest{
						{Address: fmt.Sprintf("worker%d-test%d@example.com", workerID, j)},
					},
				}

				_, err := suite.service.CreateNotification(suite.ctx, req)
				if err != nil {
					results <- fmt.Errorf("worker %d, notification %d failed: %w", workerID, j, err)
					return
				}
			}

			results <- nil
		}(i)
	}

	for i := 0; i < numWorkers; i++ {
		result := <-results
		suite.NoError(result)
	}

	filter := models.NotificationFilter{}
	notifications, total, err := suite.service.ListNotifications(suite.ctx, filter, 1, 100)
	suite.Require().NoError(err)
	suite.Equal(int64(numWorkers*notificationsPerWorker), total)
	suite.Len(notifications, numWorkers*notificationsPerWorker)
}

func (suite *NotificationServiceIntegrationTestSuite) TestCreateManyNotifications() {
	requests := []models.CreateNotificationRequest{
		{
			IdempotencyKey: "batch-integration-1",
			Type:           models.NotificationTypeEmail,
			Subject:        "Batch Email 1",
			Content:        "First batch email content",
			Provider:       "sendgrid",
			Recipients:     []models.CreateRecipientRequest{{Address: "batch1@example.com"}},
			Variables:      map[string]string{"user": "Alice"},
		},
		{
			IdempotencyKey: "batch-integration-2",
			Type:           models.NotificationTypeSMS,
			Content:        "Batch SMS content",
			Provider:       "twilio",
			Recipients:     []models.CreateRecipientRequest{{Address: "+1234567890"}},
			Variables:      map[string]string{"code": "123456"},
		},
		{
			IdempotencyKey: "batch-integration-3",
			Type:           models.NotificationTypePush,
			Subject:        "Batch Push Notification",
			Content:        "Push notification content",
			Provider:       "fcm",
			Recipients: []models.CreateRecipientRequest{
				{Address: "device-token-123"},
				{Address: "device-token-456"},
			},
			CC:        []models.CreateRecipientRequest{{Address: "cc@example.com"}},
			Variables: map[string]string{"title": "Alert"},
		},
	}

	notifications, err := suite.service.CreateManyNotifications(suite.ctx, requests)
	suite.Require().NoError(err)
	suite.Len(notifications, 3)

	for i, notification := range notifications {
		suite.NotEmpty(notification.ID)
		suite.Equal(requests[i].IdempotencyKey, notification.IdempotencyKey)
		suite.Equal(requests[i].Type, notification.Type)
		suite.Equal(requests[i].Subject, notification.Subject)
		suite.Equal(requests[i].Content, notification.Content)
		suite.Equal(models.NotificationStatusQueued, notification.Status)
		suite.Equal(requests[i].Variables, notification.Variables)
		suite.NotZero(notification.CreatedAt)
		suite.NotZero(notification.UpdatedAt)

		retrieved, err := suite.service.GetNotification(suite.ctx, notification.ID)
		suite.Require().NoError(err)
		suite.Equal(notification.ID, retrieved.ID)
		suite.Equal(notification.Type, retrieved.Type)
		suite.Equal(notification.Variables, retrieved.Variables)
	}
}

func (suite *NotificationServiceIntegrationTestSuite) TestCreateManyNotificationsWithDuplicates() {
	// First create a notification
	req := models.CreateNotificationRequest{
		IdempotencyKey: "duplicate-batch-test",
		Type:           models.NotificationTypeEmail,
		Subject:        "Original Email",
		Content:        "Original content",
		Provider:       "sendgrid",
		Recipients:     []models.CreateRecipientRequest{{Address: "duplicate@example.com"}},
	}

	original, err := suite.service.CreateNotification(suite.ctx, req)
	suite.Require().NoError(err)

	// Now create a batch that includes the duplicate
	batchReqs := []models.CreateNotificationRequest{
		{
			IdempotencyKey: "batch-new-1",
			Type:           models.NotificationTypeSMS,
			Content:        "New SMS",
			Provider:       "twilio",
			Recipients:     []models.CreateRecipientRequest{{Address: "+1111111111"}},
		},
		req, // This is the duplicate
		{
			IdempotencyKey: "batch-new-2",
			Type:           models.NotificationTypePush,
			Content:        "New Push",
			Provider:       "fcm",
			Recipients:     []models.CreateRecipientRequest{{Address: "new-token"}},
		},
	}

	result, err := suite.service.CreateManyNotifications(suite.ctx, batchReqs)
	suite.Require().NoError(err)
	suite.Len(result, 3)

	// First notification should be new
	suite.NotEqual(original.ID, result[0].ID)
	suite.Equal("batch-new-1", result[0].IdempotencyKey)

	// Second notification should be the duplicate (same ID and created time)
	suite.Equal(original.ID, result[1].ID)
	suite.Equal(original.CreatedAt, result[1].CreatedAt)
	suite.Equal("duplicate-batch-test", result[1].IdempotencyKey)

	// Third notification should be new
	suite.NotEqual(original.ID, result[2].ID)
	suite.Equal("batch-new-2", result[2].IdempotencyKey)
}

func (suite *NotificationServiceIntegrationTestSuite) TestCreateManyNotificationsEmpty() {
	result, err := suite.service.CreateManyNotifications(suite.ctx, []models.CreateNotificationRequest{})
	suite.Require().NoError(err)
	suite.Empty(result)
}

func (suite *NotificationServiceIntegrationTestSuite) TestCreateManyNotificationsLargeBatch() {
	const batchSize = 50
	requests := make([]models.CreateNotificationRequest, batchSize)

	for i := 0; i < batchSize; i++ {
		requests[i] = models.CreateNotificationRequest{
			IdempotencyKey: fmt.Sprintf("large-batch-integration-%d", i),
			Type:           models.NotificationTypeEmail,
			Subject:        fmt.Sprintf("Large Batch Email %d", i),
			Content:        fmt.Sprintf("Content for notification %d", i),
			Provider:       "large-batch-provider",
			Recipients:     []models.CreateRecipientRequest{{Address: fmt.Sprintf("user%d@example.com", i)}},
			Variables:      map[string]string{"index": fmt.Sprintf("%d", i)},
		}
	}

	notifications, err := suite.service.CreateManyNotifications(suite.ctx, requests)
	suite.Require().NoError(err)
	suite.Len(notifications, batchSize)

	// Verify all notifications were created correctly
	for i, notification := range notifications {
		suite.Equal(requests[i].IdempotencyKey, notification.IdempotencyKey)
		suite.Equal(requests[i].Variables, notification.Variables)
		suite.NotZero(notification.CreatedAt)
	}

	// Verify they're all in the database
	filter := models.NotificationFilter{Provider: "large-batch-provider"}
	retrieved, total, err := suite.service.ListNotifications(suite.ctx, filter, 1, batchSize)
	suite.Require().NoError(err)
	suite.Equal(int64(batchSize), total)
	suite.Len(retrieved, batchSize)
}

func TestNotificationServiceIntegrationSuite(t *testing.T) {
	suite.Run(t, new(NotificationServiceIntegrationTestSuite))
}
