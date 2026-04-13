package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type NotificationRepositoryTestSuite struct {
	suite.Suite
	db        *sql.DB
	repo      NotificationRepository
	container testcontainers.Container
	ctx       context.Context
}

func TestNotificationRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationRepositoryTestSuite))
}

func (s *NotificationRepositoryTestSuite) SetupSuite() {
	s.ctx = context.Background()

	postgresContainer, err := postgres.Run(s.ctx, "postgres:17-alpine",
		postgres.WithDatabase("service_notification_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(s.T(), err)
	s.container = postgresContainer

	connStr, err := postgresContainer.ConnectionString(s.ctx, "sslmode=disable")
	require.NoError(s.T(), err)

	s.db, err = sql.Open("postgres", connStr)
	require.NoError(s.T(), err)

	err = s.db.PingContext(s.ctx)
	require.NoError(s.T(), err)

	err = s.runMigrations()
	require.NoError(s.T(), err)

	s.repo = NewNotificationRepository(s.db)
}

func (s *NotificationRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(s.T(), err)
	}
}

func (s *NotificationRepositoryTestSuite) SetupTest() {
	_, err := s.db.Exec("TRUNCATE notifications, recipients, notification_events RESTART IDENTITY CASCADE")
	require.NoError(s.T(), err)
}

func (s *NotificationRepositoryTestSuite) runMigrations() error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	migrationsDir := filepath.Join("..", "..", "migrations")

	if err := goose.Up(s.db, migrationsDir); err != nil {
		return err
	}

	return nil
}

func (s *NotificationRepositoryTestSuite) TestCreate() {
	notification := &models.Notification{
		ID:             uuid.New().String(),
		IdempotencyKey: "test-key-123",
		Type:           models.NotificationTypeEmail,
		Subject:        "Test Subject",
		Content:        "Test notification content",
		Status:         models.NotificationStatusQueued,
		Provider:       "test-provider",
		RetryCount:     0,
		Recipient: []models.Recipient{
			{
				Type:    models.RecipientTypeTo,
				Address: "test@example.com",
			},
		},
		CC: []models.Recipient{
			{
				Type:    models.RecipientTypeCC,
				Address: "cc@example.com",
			},
		},
		BCC: []models.Recipient{
			{
				Type:    models.RecipientTypeBCC,
				Address: "bcc@example.com",
			},
		},
	}

	err := s.repo.Create(s.ctx, notification)
	require.NoError(s.T(), err)

	assert.NotZero(s.T(), notification.CreatedAt)

	retrieved, err := s.repo.GetByID(s.ctx, notification.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), notification.ID, retrieved.ID)
	assert.Equal(s.T(), notification.Type, retrieved.Type)
	assert.Equal(s.T(), notification.Subject, retrieved.Subject)
	assert.Equal(s.T(), notification.Content, retrieved.Content)
	assert.Equal(s.T(), notification.Status, retrieved.Status)
	assert.Equal(s.T(), notification.IdempotencyKey, retrieved.IdempotencyKey)
	assert.Len(s.T(), retrieved.Recipient, 1)
	assert.Len(s.T(), retrieved.CC, 1)
	assert.Len(s.T(), retrieved.BCC, 1)
}

func (s *NotificationRepositoryTestSuite) TestGetByID() {
	notification := &models.Notification{
		ID:             uuid.New().String(),
		IdempotencyKey: "get-by-id-test",
		Type:           models.NotificationTypeSMS,
		Subject:        "SMS Test",
		Content:        "SMS test content",
		Status:         models.NotificationStatusSent,
		Provider:       "sms-provider",
		RetryCount:     1,
		Recipient: []models.Recipient{
			{
				Type:    models.RecipientTypeTo,
				Address: "+1234567890",
			},
		},
	}
	err := s.repo.Create(s.ctx, notification)
	require.NoError(s.T(), err)

	retrieved, err := s.repo.GetByID(s.ctx, notification.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), notification.ID, retrieved.ID)
	assert.Equal(s.T(), notification.Type, retrieved.Type)
	assert.Equal(s.T(), notification.Status, retrieved.Status)

	nonExistentID := uuid.New().String()
	_, err = s.repo.GetByID(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *NotificationRepositoryTestSuite) TestGetByIdempotencyKey() {
	idempotencyKey := "unique-key-12345"
	notification := &models.Notification{
		ID:             uuid.New().String(),
		IdempotencyKey: idempotencyKey,
		Type:           models.NotificationTypeEmail,
		Subject:        "Idempotency Test",
		Content:        "Test content",
		Status:         models.NotificationStatusQueued,
		Recipient: []models.Recipient{
			{
				Type:    models.RecipientTypeTo,
				Address: "test@example.com",
			},
		},
	}
	err := s.repo.Create(s.ctx, notification)
	require.NoError(s.T(), err)

	retrieved, err := s.repo.GetByIdempotencyKey(s.ctx, idempotencyKey)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), notification.ID, retrieved.ID)
	assert.Equal(s.T(), idempotencyKey, retrieved.IdempotencyKey)

	_, err = s.repo.GetByIdempotencyKey(s.ctx, "non-existent-key")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *NotificationRepositoryTestSuite) TestUpdate() {
	notification := &models.Notification{
		ID:             uuid.New().String(),
		IdempotencyKey: "update-test",
		Type:           models.NotificationTypeEmail,
		Subject:        "Original Subject",
		Content:        "Original content",
		Status:         models.NotificationStatusQueued,
		Provider:       "original-provider",
		RetryCount:     0,
		Recipient: []models.Recipient{
			{
				Type:    models.RecipientTypeTo,
				Address: "original@example.com",
			},
		},
	}
	err := s.repo.Create(s.ctx, notification)
	require.NoError(s.T(), err)

	originalUpdatedAt := notification.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	notification.Subject = "Updated Subject"
	notification.Content = "Updated content"
	notification.Status = models.NotificationStatusSent
	notification.RetryCount = 1
	providerMsgID := "provider-msg-123"
	notification.ProviderMessageID = &providerMsgID
	notification.ProviderResponse = map[string]interface{}{
		"status": "success",
		"id":     "123",
	}

	err = s.repo.Update(s.ctx, notification)
	require.NoError(s.T(), err)
	assert.True(s.T(), notification.UpdatedAt.After(originalUpdatedAt))

	updated, err := s.repo.GetByID(s.ctx, notification.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Subject", updated.Subject)
	assert.Equal(s.T(), "Updated content", updated.Content)
	assert.Equal(s.T(), models.NotificationStatusSent, updated.Status)
	assert.Equal(s.T(), int8(1), updated.RetryCount)
	assert.Equal(s.T(), "provider-msg-123", *updated.ProviderMessageID)
	assert.NotNil(s.T(), updated.ProviderResponse)
}

func (s *NotificationRepositoryTestSuite) TestDelete() {
	notification := &models.Notification{
		ID:             uuid.New().String(),
		IdempotencyKey: "delete-test",
		Type:           models.NotificationTypeEmail,
		Subject:        "Delete Test",
		Content:        "Test content",
		Status:         models.NotificationStatusQueued,
		Recipient: []models.Recipient{
			{
				Type:    models.RecipientTypeTo,
				Address: "delete@example.com",
			},
		},
	}
	err := s.repo.Create(s.ctx, notification)
	require.NoError(s.T(), err)

	err = s.repo.Delete(s.ctx, notification.ID)
	require.NoError(s.T(), err)

	_, err = s.repo.GetByID(s.ctx, notification.ID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")

	nonExistentID := uuid.New().String()
	err = s.repo.Delete(s.ctx, nonExistentID)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
}

func (s *NotificationRepositoryTestSuite) TestList() {
	notifications := []*models.Notification{
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "list-test-1",
			Type:           models.NotificationTypeEmail,
			Subject:        "Email Test 1",
			Content:        "Email content 1",
			Status:         models.NotificationStatusQueued,
			Provider:       "email-provider",
			RetryCount:     0,
			Recipient: []models.Recipient{
				{
					Type:    models.RecipientTypeTo,
					Address: "email1@example.com",
				},
			},
		},
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "list-test-2",
			Type:           models.NotificationTypeSMS,
			Subject:        "SMS Test 1",
			Content:        "SMS content 1",
			Status:         models.NotificationStatusSent,
			Provider:       "sms-provider",
			RetryCount:     0,
			Recipient: []models.Recipient{
				{
					Type:    models.RecipientTypeTo,
					Address: "+1234567890",
				},
			},
		},
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "list-test-3",
			Type:           models.NotificationTypeEmail,
			Subject:        "Email Test 2",
			Content:        "Email content 2",
			Status:         models.NotificationStatusFailed,
			Provider:       "email-provider",
			RetryCount:     2,
			Recipient: []models.Recipient{
				{
					Type:    models.RecipientTypeTo,
					Address: "email2@example.com",
				},
			},
		},
	}

	for _, notification := range notifications {
		err := s.repo.Create(s.ctx, notification)
		require.NoError(s.T(), err)
	}

	filter := models.NotificationFilter{}
	retrieved, total, err := s.repo.List(s.ctx, filter, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), retrieved, 3)

	emailType := models.NotificationTypeEmail
	typeFilter := models.NotificationFilter{Type: emailType}
	emailNotifications, emailTotal, err := s.repo.List(s.ctx, typeFilter, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), emailTotal)
	assert.Len(s.T(), emailNotifications, 2)

	sentStatus := models.NotificationStatusSent
	statusFilter := models.NotificationFilter{Status: sentStatus}
	sentNotifications, sentTotal, err := s.repo.List(s.ctx, statusFilter, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), sentTotal)
	assert.Len(s.T(), sentNotifications, 1)
	assert.Equal(s.T(), models.NotificationStatusSent, sentNotifications[0].Status)

	providerFilter := models.NotificationFilter{Provider: "email-provider"}
	providerNotifications, providerTotal, err := s.repo.List(s.ctx, providerFilter, 1, 10)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), providerTotal)
	assert.Len(s.T(), providerNotifications, 2)

	paginatedResults, paginatedTotal, err := s.repo.List(s.ctx, filter, 1, 2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), paginatedTotal)
	assert.Len(s.T(), paginatedResults, 2)

	secondPageResults, secondPageTotal, err := s.repo.List(s.ctx, filter, 2, 2)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), secondPageTotal)
	assert.Len(s.T(), secondPageResults, 1)
}

func (s *NotificationRepositoryTestSuite) TestGetByStatus() {
	notifications := []*models.Notification{
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "status-test-1",
			Type:           models.NotificationTypeEmail,
			Subject:        "Queued Email 1",
			Content:        "Content 1",
			Status:         models.NotificationStatusQueued,
			Recipient: []models.Recipient{
				{
					Type:    models.RecipientTypeTo,
					Address: "queued1@example.com",
				},
			},
		},
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "status-test-2",
			Type:           models.NotificationTypeSMS,
			Subject:        "Sent SMS",
			Content:        "Content 2",
			Status:         models.NotificationStatusSent,
			Recipient: []models.Recipient{
				{
					Type:    models.RecipientTypeTo,
					Address: "+1234567890",
				},
			},
		},
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "status-test-3",
			Type:           models.NotificationTypeEmail,
			Subject:        "Queued Email 2",
			Content:        "Content 3",
			Status:         models.NotificationStatusQueued,
			Recipient: []models.Recipient{
				{
					Type:    models.RecipientTypeTo,
					Address: "queued2@example.com",
				},
			},
		},
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "status-test-4",
			Type:           models.NotificationTypePush,
			Subject:        "Failed Push",
			Content:        "Content 4",
			Status:         models.NotificationStatusFailed,
			Recipient: []models.Recipient{
				{
					Type:    models.RecipientTypeTo,
					Address: "device-token-123",
				},
			},
		},
	}

	for _, notification := range notifications {
		err := s.repo.Create(s.ctx, notification)
		require.NoError(s.T(), err)
	}

	queuedNotifications, err := s.repo.GetByStatus(s.ctx, models.NotificationStatusQueued)
	require.NoError(s.T(), err)
	assert.Len(s.T(), queuedNotifications, 2)

	for _, notification := range queuedNotifications {
		assert.Equal(s.T(), models.NotificationStatusQueued, notification.Status)
	}

	assert.True(s.T(), queuedNotifications[0].CreatedAt.Before(queuedNotifications[1].CreatedAt) ||
		queuedNotifications[0].CreatedAt.Equal(queuedNotifications[1].CreatedAt))

	sentNotifications, err := s.repo.GetByStatus(s.ctx, models.NotificationStatusSent)
	require.NoError(s.T(), err)
	assert.Len(s.T(), sentNotifications, 1)
	assert.Equal(s.T(), models.NotificationStatusSent, sentNotifications[0].Status)

	failedNotifications, err := s.repo.GetByStatus(s.ctx, models.NotificationStatusFailed)
	require.NoError(s.T(), err)
	assert.Len(s.T(), failedNotifications, 1)
	assert.Equal(s.T(), models.NotificationStatusFailed, failedNotifications[0].Status)
}

func (s *NotificationRepositoryTestSuite) TestProviderResponseJSONHandling() {
	providerResponse := map[string]interface{}{
		"messageId": "msg-12345",
		"status":    "delivered",
		"timestamp": "2023-01-01T10:00:00Z",
		"attempts":  3,
		"metadata": map[string]interface{}{
			"carrier": "Verizon",
			"region":  "US-East",
		},
	}

	notification := &models.Notification{
		ID:             uuid.New().String(),
		IdempotencyKey: "json-test",
		Type:           models.NotificationTypeSMS,
		Subject:        "JSON Test",
		Content:        "Testing JSON handling",
		Status:         models.NotificationStatusQueued,
		Provider:       "json-provider",
		RetryCount:     0,
		Recipient: []models.Recipient{
			{
				Type:    models.RecipientTypeTo,
				Address: "+1234567890",
			},
		},
	}

	err := s.repo.Create(s.ctx, notification)
	require.NoError(s.T(), err)

	notification.Status = models.NotificationStatusDelivered
	notification.ProviderResponse = providerResponse
	err = s.repo.Update(s.ctx, notification)
	require.NoError(s.T(), err)

	retrieved, err := s.repo.GetByID(s.ctx, notification.ID)
	require.NoError(s.T(), err)

	assert.NotNil(s.T(), retrieved.ProviderResponse)
	responseMap, ok := retrieved.ProviderResponse.(map[string]interface{})
	assert.True(s.T(), ok)
	assert.Equal(s.T(), "msg-12345", responseMap["messageId"])
	assert.Equal(s.T(), "delivered", responseMap["status"])
	assert.Equal(s.T(), float64(3), responseMap["attempts"])

	metadataMap, ok := responseMap["metadata"].(map[string]interface{})
	assert.True(s.T(), ok)
	assert.Equal(s.T(), "Verizon", metadataMap["carrier"])
	assert.Equal(s.T(), "US-East", metadataMap["region"])
}

func (s *NotificationRepositoryTestSuite) TestCreateMany() {
	notifications := []*models.Notification{
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "batch-test-1",
			Type:           models.NotificationTypeEmail,
			Subject:        "Batch Email 1",
			Content:        "Batch content 1",
			Status:         models.NotificationStatusQueued,
			Provider:       "batch-provider",
			Variables:      map[string]string{"name": "Alice"},
			Recipient: []models.Recipient{
				{Type: models.RecipientTypeTo, Address: "alice@example.com"},
			},
			CC: []models.Recipient{
				{Type: models.RecipientTypeCC, Address: "cc1@example.com"},
			},
		},
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "batch-test-2",
			Type:           models.NotificationTypeSMS,
			Content:        "Batch SMS content",
			Status:         models.NotificationStatusQueued,
			Provider:       "sms-batch-provider",
			Variables:      map[string]string{"code": "123456"},
			Recipient: []models.Recipient{
				{Type: models.RecipientTypeTo, Address: "+1234567890"},
			},
			BCC: []models.Recipient{
				{Type: models.RecipientTypeBCC, Address: "bcc@example.com"},
			},
		},
		{
			ID:             uuid.New().String(),
			IdempotencyKey: "batch-test-3",
			Type:           models.NotificationTypePush,
			Subject:        "Batch Push",
			Content:        "Push notification content",
			Status:         models.NotificationStatusQueued,
			Provider:       "push-batch-provider",
			Variables:      map[string]string{"title": "Alert"},
			Recipient: []models.Recipient{
				{Type: models.RecipientTypeTo, Address: "device-token-123"},
				{Type: models.RecipientTypeTo, Address: "device-token-456"},
			},
		},
	}

	err := s.repo.CreateMany(s.ctx, notifications)
	require.NoError(s.T(), err)

	for _, notification := range notifications {
		assert.NotZero(s.T(), notification.CreatedAt)

		retrieved, err := s.repo.GetByID(s.ctx, notification.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), notification.ID, retrieved.ID)
		assert.Equal(s.T(), notification.Type, retrieved.Type)
		assert.Equal(s.T(), notification.Subject, retrieved.Subject)
		assert.Equal(s.T(), notification.Content, retrieved.Content)
		assert.Equal(s.T(), notification.Status, retrieved.Status)
		assert.Equal(s.T(), notification.Provider, retrieved.Provider)
		assert.Equal(s.T(), notification.Variables, retrieved.Variables)
		assert.Len(s.T(), retrieved.Recipient, len(notification.Recipient))
		assert.Len(s.T(), retrieved.CC, len(notification.CC))
		assert.Len(s.T(), retrieved.BCC, len(notification.BCC))
	}
}

func (s *NotificationRepositoryTestSuite) TestCreateManyEmpty() {
	err := s.repo.CreateMany(s.ctx, []*models.Notification{})
	require.NoError(s.T(), err)
}

func (s *NotificationRepositoryTestSuite) TestCreateManyLarge() {
	const batchSize = 100
	notifications := make([]*models.Notification, batchSize)

	for i := 0; i < batchSize; i++ {
		notifications[i] = &models.Notification{
			ID:             uuid.New().String(),
			IdempotencyKey: fmt.Sprintf("large-batch-%d", i),
			Type:           models.NotificationTypeEmail,
			Subject:        fmt.Sprintf("Large Batch Email %d", i),
			Content:        fmt.Sprintf("Content for notification %d", i),
			Status:         models.NotificationStatusQueued,
			Provider:       "large-batch-provider",
			Variables:      map[string]string{"index": fmt.Sprintf("%d", i)},
			Recipient: []models.Recipient{
				{Type: models.RecipientTypeTo, Address: fmt.Sprintf("user%d@example.com", i)},
			},
		}
	}

	err := s.repo.CreateMany(s.ctx, notifications)
	require.NoError(s.T(), err)

	filter := models.NotificationFilter{Provider: "large-batch-provider"}
	retrieved, total, err := s.repo.List(s.ctx, filter, 1, batchSize)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(batchSize), total)
	assert.Len(s.T(), retrieved, batchSize)
}

func (s *NotificationRepositoryTestSuite) TestCreateManyMarketingEmails1000() {
	const batchSize = 1000
	notifications := make([]*models.Notification, batchSize)

	startTime := time.Now()

	emailTemplates := []struct {
		subject  string
		content  string
		campaign string
	}{
		{"� Lottery Results Are In!", "Check your numbers! The winning lottery numbers for draw #{{draw_id}} are: {{winning_numbers}}", "lottery_results"},
		{"🎟️ Your Lottery Ticket Confirmation", "Your lottery ticket has been purchased successfully. Ticket ID: {{ticket_id}} for draw #{{draw_id}}", "ticket_confirmation"},
		{"💰 Congratulations - You Won!", "Amazing news! Your ticket {{ticket_id}} won {{prize_amount}} in draw #{{draw_id}}. Claim your prize now!", "winner_notification"},
		{"⏰ Draw Starting Soon", "The lottery draw #{{draw_id}} starts in {{time_remaining}}. Last chance to buy tickets!", "draw_reminder"},
		{"🔔 Weekly Lottery Newsletter", "This week's lottery updates: upcoming draws, winner stories, and jackpot amounts.", "newsletter"},
	}

	for i := 0; i < batchSize; i++ {
		template := emailTemplates[i%len(emailTemplates)]

		notifications[i] = &models.Notification{
			ID:             uuid.New().String(),
			IdempotencyKey: fmt.Sprintf("lottery-email-%d-%d", time.Now().Unix(), i),
			Type:           models.NotificationTypeEmail,
			Subject:        template.subject,
			Content:        template.content,
			Status:         models.NotificationStatusQueued,
			Provider:       "sendgrid",
			Variables: map[string]string{
				"user_id":         fmt.Sprintf("player_%d", i),
				"campaign":        template.campaign,
				"draw_id":         fmt.Sprintf("DRAW-%d", 1000+i),
				"ticket_id":       fmt.Sprintf("TKT-%08d", 10000000+i),
				"winning_numbers": fmt.Sprintf("%02d-%02d-%02d-%02d-%02d", (i%45)+1, ((i+7)%45)+1, ((i+14)%45)+1, ((i+21)%45)+1, ((i+28)%45)+1),
				"prize_amount":    fmt.Sprintf("$%s", []string{"100", "1,000", "10,000", "100,000", "1,000,000"}[i%5]),
				"time_remaining":  fmt.Sprintf("%d hours", (i%24)+1),
			},
			TemplateID: fmt.Sprintf("template_%s", template.campaign),
			Recipient: []models.Recipient{
				{Type: models.RecipientTypeTo, Address: fmt.Sprintf("player%d@lottery.com", i)},
			},
		}
	}

	// Perform bulk insert
	err := s.repo.CreateMany(s.ctx, notifications)
	require.NoError(s.T(), err)

	insertDuration := time.Since(startTime)
	s.T().Logf("CreateMany for 1000 marketing emails took: %v", insertDuration)

	for _, notification := range notifications {
		assert.NotZero(s.T(), notification.CreatedAt)
		assert.NotZero(s.T(), notification.UpdatedAt)
	}

	sampleIndices := []int{0, 249, 500, 750, 999}
	for _, idx := range sampleIndices {
		retrieved, err := s.repo.GetByID(s.ctx, notifications[idx].ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), notifications[idx].ID, retrieved.ID)
		assert.Equal(s.T(), models.NotificationTypeEmail, retrieved.Type)
		assert.NotEmpty(s.T(), retrieved.Subject)
		assert.Equal(s.T(), "sendgrid", retrieved.Provider)
		assert.Equal(s.T(), notifications[idx].Variables, retrieved.Variables)
		assert.NotEmpty(s.T(), retrieved.TemplateID)
		assert.Len(s.T(), retrieved.Recipient, 1)
		assert.Empty(s.T(), retrieved.CC)
		assert.Empty(s.T(), retrieved.BCC)
	}

	filter := models.NotificationFilter{Provider: "sendgrid"}
	retrieved, total, err := s.repo.List(s.ctx, filter, 1, batchSize)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(batchSize), total)
	assert.Len(s.T(), retrieved, batchSize)

	campaignCounts := make(map[string]int)
	templateCounts := make(map[string]int)
	for _, notification := range retrieved {
		if campaign, exists := notification.Variables["campaign"]; exists && campaign != "" {
			campaignCounts[campaign]++
		}
		if notification.TemplateID != "" {
			templateCounts[notification.TemplateID]++
		}
	}
	s.T().Logf("Found %d different campaigns and %d different templates", len(campaignCounts), len(templateCounts))
	assert.GreaterOrEqual(s.T(), len(templateCounts), 1)
}

func (s *NotificationRepositoryTestSuite) TestCreateManyTransactionalSMS1000() {
	const batchSize = 1000
	notifications := make([]*models.Notification, batchSize)

	startTime := time.Now()

	smsTypes := []struct {
		content    string
		smsType    string
		templateID string
	}{
		{"Your lottery verification code is {{code}}. It expires in 5 minutes.", "verification", "sms_verification"},
		{"Your lottery ticket #{{ticket_id}} has been purchased. Draw: {{draw_id}}", "ticket_purchase", "sms_ticket_confirmed"},
		{"Congratulations! Your ticket {{ticket_id}} won {{prize_amount}}!", "winner_alert", "sms_winner_notification"},
		{"Lottery draw #{{draw_id}} results: {{winning_numbers}}. Check your tickets!", "draw_results", "sms_results_alert"},
		{"Security alert: New login detected from {{location}}. Secure your lottery account.", "security_alert", "sms_security_alert"},
		{"Reminder: Lottery draw #{{draw_id}} starts at {{time}}. Last chance to play!", "draw_reminder", "sms_draw_reminder"},
		{"Your lottery wallet balance is low: ${{balance}}. Add funds to play more games.", "low_balance", "sms_balance_alert"},
	}

	for i := 0; i < batchSize; i++ {
		smsTemplate := smsTypes[i%len(smsTypes)]

		areaCode := 200 + (i % 800)
		phoneNumber := fmt.Sprintf("+1%03d%03d%04d", areaCode, (i%900)+100, i%10000)

		notifications[i] = &models.Notification{
			ID:             uuid.New().String(),
			IdempotencyKey: fmt.Sprintf("lottery-sms-%d-%d", time.Now().Unix(), i),
			Type:           models.NotificationTypeSMS,
			Content:        smsTemplate.content,
			Status:         models.NotificationStatusQueued,
			Provider:       "twilio",
			Variables: map[string]string{
				"code":            fmt.Sprintf("%06d", (i*7+123456)%1000000),
				"ticket_id":       fmt.Sprintf("TKT-%08d", 20000000+i),
				"draw_id":         fmt.Sprintf("DRAW-%d", 2000+i),
				"prize_amount":    fmt.Sprintf("$%s", []string{"50", "500", "5,000", "50,000", "500,000"}[i%5]),
				"winning_numbers": fmt.Sprintf("%02d-%02d-%02d-%02d-%02d", (i%49)+1, ((i+11)%49)+1, ((i+22)%49)+1, ((i+33)%49)+1, ((i+44)%49)+1),
				"location":        []string{"New York, NY", "Los Angeles, CA", "Chicago, IL", "Houston, TX", "Phoenix, AZ"}[i%5],
				"time":            "8:00 PM",
				"balance":         fmt.Sprintf("%.2f", float64(i%500)/1.0),
				"user_id":         fmt.Sprintf("player_%d", i),
				"sms_type":        smsTemplate.smsType,
			},
			TemplateID: smsTemplate.templateID,
			Recipient: []models.Recipient{
				{Type: models.RecipientTypeTo, Address: phoneNumber},
			},
		}
	}

	// Perform bulk insert
	err := s.repo.CreateMany(s.ctx, notifications)
	require.NoError(s.T(), err)

	insertDuration := time.Since(startTime)
	s.T().Logf("CreateMany for 1000 transactional SMS took: %v", insertDuration)

	// Verify all notifications were created with timestamps
	for _, notification := range notifications {
		assert.NotZero(s.T(), notification.CreatedAt)
		assert.NotZero(s.T(), notification.UpdatedAt)
	}

	// Verify data integrity by sampling some records
	sampleIndices := []int{0, 199, 400, 650, 999}
	for _, idx := range sampleIndices {
		retrieved, err := s.repo.GetByID(s.ctx, notifications[idx].ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), notifications[idx].ID, retrieved.ID)
		assert.Equal(s.T(), models.NotificationTypeSMS, retrieved.Type)
		assert.NotEmpty(s.T(), retrieved.Content)
		assert.Equal(s.T(), "twilio", retrieved.Provider)
		assert.Equal(s.T(), notifications[idx].Variables, retrieved.Variables)
		assert.NotEmpty(s.T(), retrieved.TemplateID)
		assert.Len(s.T(), retrieved.Recipient, 1)
		assert.Regexp(s.T(), regexp.MustCompile(`^\+1\d{10}$`), retrieved.Recipient[0].Address)
		assert.Empty(s.T(), retrieved.CC)
		assert.Empty(s.T(), retrieved.BCC)
	}

	// Verify total count
	filter := models.NotificationFilter{Provider: "twilio"}
	retrieved, total, err := s.repo.List(s.ctx, filter, 1, batchSize)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(batchSize), total)
	assert.Len(s.T(), retrieved, batchSize)

	smsTypeCounts := make(map[string]int)
	templateCounts := make(map[string]int)
	for _, notification := range retrieved {
		if smsType, exists := notification.Variables["sms_type"]; exists && smsType != "" {
			smsTypeCounts[smsType]++
		}
		if notification.TemplateID != "" {
			templateCounts[notification.TemplateID]++
		}
	}
	s.T().Logf("Found %d different SMS types and %d different templates", len(smsTypeCounts), len(templateCounts))
	assert.GreaterOrEqual(s.T(), len(templateCounts), 1)
}

func (s *NotificationRepositoryTestSuite) TestCreateManyMobilePush1000() {
	const batchSize = 1000
	notifications := make([]*models.Notification, batchSize)

	startTime := time.Now()

	pushTypes := []struct {
		title      string
		content    string
		pushType   string
		templateID string
	}{
		{"💰 Jackpot Winner!", "Congratulations! You won {{prize_amount}} with ticket {{ticket_id}}!", "jackpot_winner", "push_jackpot_winner"},
		{"🎟️ Ticket Confirmed", "Your lottery ticket {{ticket_id}} for draw {{draw_id}} is confirmed", "ticket_confirmed", "push_ticket_confirmed"},
		{"🎰 Draw Results", "Draw {{draw_id}} results are in! Winning numbers: {{winning_numbers}}", "draw_results", "push_draw_results"},
		{"⏰ Draw Starting Soon", "Lottery draw {{draw_id}} starts in {{time_left}}. Last chance to play!", "draw_reminder", "push_draw_reminder"},
		{"🏆 Prize Claim", "You have {{days_left}} days left to claim your {{prize_amount}} prize!", "prize_claim", "push_prize_claim"},
		{"🎯 Lucky Numbers", "Your lucky numbers for today: {{lucky_numbers}}", "lucky_numbers", "push_lucky_numbers"},
		{"💳 Wallet Update", "Your lottery wallet was credited with {{amount}}", "wallet_update", "push_wallet_update"},
		{"🚨 Security Alert", "New login to your lottery account from {{device}} in {{location}}", "security", "push_security_alert"},
	}

	for i := 0; i < batchSize; i++ {
		pushTemplate := pushTypes[i%len(pushTypes)]

		var deviceToken string
		if i%2 == 0 {
			deviceToken = fmt.Sprintf("fcm_%060x", i*12345)
		} else {
			deviceToken = fmt.Sprintf("apns_%028x", i*67890)
		}

		notifications[i] = &models.Notification{
			ID:             uuid.New().String(),
			IdempotencyKey: fmt.Sprintf("lottery-push-%d-%d", time.Now().Unix(), i),
			Type:           models.NotificationTypePush,
			Subject:        pushTemplate.title,
			Content:        pushTemplate.content,
			Status:         models.NotificationStatusQueued,
			Provider:       "fcm",
			Variables: map[string]string{
				"prize_amount":    fmt.Sprintf("$%s", []string{"100", "1,000", "10,000", "100,000", "1,000,000", "10,000,000"}[i%6]),
				"ticket_id":       fmt.Sprintf("TKT-%08d", 30000000+i),
				"draw_id":         fmt.Sprintf("DRAW-%d", 3000+i),
				"winning_numbers": fmt.Sprintf("%02d-%02d-%02d-%02d-%02d-%02d", (i%49)+1, ((i+8)%49)+1, ((i+16)%49)+1, ((i+24)%49)+1, ((i+32)%49)+1, ((i+40)%49)+1),
				"lucky_numbers":   fmt.Sprintf("%d, %d, %d, %d, %d", (i%20)+1, ((i+5)%20)+1, ((i+10)%20)+1, ((i+15)%20)+1, ((i+18)%20)+1),
				"time_left":       fmt.Sprintf("%d minutes", []int{15, 30, 45, 60, 120}[i%5]),
				"days_left":       fmt.Sprintf("%d", []int{1, 7, 15, 30, 90}[i%5]),
				"amount":          fmt.Sprintf("$%.2f", float64(50+i%950)/1.0),
				"device":          []string{"iPhone 15", "Samsung Galaxy S24", "Google Pixel 8", "iPad Pro"}[i%4],
				"location":        []string{"Las Vegas, NV", "Atlantic City, NJ", "Reno, NV", "Biloxi, MS"}[i%4],
				"user_id":         fmt.Sprintf("player_%d", i),
				"push_type":       pushTemplate.pushType,
				"platform":        []string{"ios", "android"}[i%2],
			},
			TemplateID: pushTemplate.templateID,
			Recipient: []models.Recipient{
				{Type: models.RecipientTypeTo, Address: deviceToken},
			},
		}
	}

	// Perform bulk insert
	err := s.repo.CreateMany(s.ctx, notifications)
	require.NoError(s.T(), err)

	insertDuration := time.Since(startTime)
	s.T().Logf("CreateMany for 1000 mobile push notifications took: %v", insertDuration)

	// Verify all notifications were created with timestamps
	for _, notification := range notifications {
		assert.NotZero(s.T(), notification.CreatedAt)
		assert.NotZero(s.T(), notification.UpdatedAt)
	}

	// Verify data integrity by sampling some records
	sampleIndices := []int{0, 100, 300, 500, 700, 990, 999}
	for _, idx := range sampleIndices {
		retrieved, err := s.repo.GetByID(s.ctx, notifications[idx].ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), notifications[idx].ID, retrieved.ID)
		assert.Equal(s.T(), models.NotificationTypePush, retrieved.Type)
		assert.NotEmpty(s.T(), retrieved.Subject)
		assert.Equal(s.T(), "fcm", retrieved.Provider)
		assert.Equal(s.T(), notifications[idx].Variables, retrieved.Variables)
		assert.NotEmpty(s.T(), retrieved.TemplateID)
		assert.Len(s.T(), retrieved.Recipient, 1)

		deviceToken := retrieved.Recipient[0].Address
		assert.True(s.T(),
			regexp.MustCompile(`^(fcm_[0-9a-f]{60}|apns_[0-9a-f]{28})$`).MatchString(deviceToken),
			"Device token should match FCM or APNs format: %s", deviceToken)

		assert.Empty(s.T(), retrieved.CC)
		assert.Empty(s.T(), retrieved.BCC)
	}

	// Verify total count
	filter := models.NotificationFilter{Provider: "fcm"}
	retrieved, total, err := s.repo.List(s.ctx, filter, 1, batchSize)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(batchSize), total)
	assert.Len(s.T(), retrieved, batchSize)

	pushTypeCounts := make(map[string]int)
	platformCounts := make(map[string]int)
	templateCounts := make(map[string]int)
	for _, notification := range retrieved {
		if pushType, exists := notification.Variables["push_type"]; exists && pushType != "" {
			pushTypeCounts[pushType]++
		}
		if platform, exists := notification.Variables["platform"]; exists && platform != "" {
			platformCounts[platform]++
		}
		if notification.TemplateID != "" {
			templateCounts[notification.TemplateID]++
		}
	}
	s.T().Logf("Found %d different push types, %d platforms, and %d different templates",
		len(pushTypeCounts), len(platformCounts), len(templateCounts))
	assert.GreaterOrEqual(s.T(), len(templateCounts), 1)

	fcmCount := 0
	apnsCount := 0
	for _, notification := range retrieved[:100] {
		deviceToken := notification.Recipient[0].Address
		if regexp.MustCompile(`^fcm_`).MatchString(deviceToken) {
			fcmCount++
		} else if regexp.MustCompile(`^apns_`).MatchString(deviceToken) {
			apnsCount++
		}
	}
	assert.Greater(s.T(), fcmCount+apnsCount, 90)
}

func (s *NotificationRepositoryTestSuite) TestNotificationWithAllFields() {
	scheduledTime := time.Date(2023, 1, 1, 15, 0, 0, 0, time.UTC)
	sentTime := time.Date(2023, 1, 1, 15, 5, 0, 0, time.UTC)
	deliveredTime := time.Date(2023, 1, 1, 15, 6, 0, 0, time.UTC)
	providerMsgID := "provider-message-12345"
	errorMsg := "Temporary failure, will retry"

	notification := &models.Notification{
		ID:             uuid.New().String(),
		IdempotencyKey: "complete-test",
		Type:           models.NotificationTypeEmail,
		Subject:        "Complete Test Email",
		Content:        "This is a complete test with all fields populated",
		Status:         models.NotificationStatusQueued,
		Provider:       "complete-provider",
		ScheduledFor:   &scheduledTime,
		Recipient: []models.Recipient{
			{
				Type:    models.RecipientTypeTo,
				Address: "primary@example.com",
			},
		},
		CC: []models.Recipient{
			{
				Type:    models.RecipientTypeCC,
				Address: "cc1@example.com",
			},
			{
				Type:    models.RecipientTypeCC,
				Address: "cc2@example.com",
			},
		},
		BCC: []models.Recipient{
			{
				Type:    models.RecipientTypeBCC,
				Address: "bcc@example.com",
			},
		},
	}

	err := s.repo.Create(s.ctx, notification)
	require.NoError(s.T(), err)

	notification.Status = models.NotificationStatusDelivered
	notification.ProviderMessageID = &providerMsgID
	notification.ProviderResponse = map[string]interface{}{
		"status":    "success",
		"messageId": providerMsgID,
		"cost":      0.05,
	}
	notification.RetryCount = 2
	notification.SentAt = &sentTime
	notification.DeliveredAt = &deliveredTime
	notification.ErrorMessage = &errorMsg
	err = s.repo.Update(s.ctx, notification)
	require.NoError(s.T(), err)

	retrieved, err := s.repo.GetByID(s.ctx, notification.ID)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), notification.ID, retrieved.ID)
	assert.Equal(s.T(), notification.IdempotencyKey, retrieved.IdempotencyKey)
	assert.Equal(s.T(), notification.Type, retrieved.Type)
	assert.Equal(s.T(), notification.Subject, retrieved.Subject)
	assert.Equal(s.T(), notification.Content, retrieved.Content)
	assert.Equal(s.T(), notification.Status, retrieved.Status)
	assert.Equal(s.T(), notification.Provider, retrieved.Provider)
	assert.Equal(s.T(), *notification.ProviderMessageID, *retrieved.ProviderMessageID)
	assert.Equal(s.T(), notification.RetryCount, retrieved.RetryCount)
	assert.Equal(s.T(), *notification.ErrorMessage, *retrieved.ErrorMessage)
	assert.True(s.T(), notification.ScheduledFor.Equal(*retrieved.ScheduledFor))
	assert.True(s.T(), notification.SentAt.Equal(*retrieved.SentAt))
	assert.True(s.T(), notification.DeliveredAt.Equal(*retrieved.DeliveredAt))

	assert.Len(s.T(), retrieved.Recipient, 1)
	assert.Equal(s.T(), "primary@example.com", retrieved.Recipient[0].Address)

	assert.Len(s.T(), retrieved.CC, 2)
	assert.Equal(s.T(), "cc1@example.com", retrieved.CC[0].Address)
	assert.Equal(s.T(), "cc2@example.com", retrieved.CC[1].Address)

	assert.Len(s.T(), retrieved.BCC, 1)
	assert.Equal(s.T(), "bcc@example.com", retrieved.BCC[0].Address)

	assert.NotNil(s.T(), retrieved.ProviderResponse)
	responseMap, ok := retrieved.ProviderResponse.(map[string]interface{})
	assert.True(s.T(), ok)
	assert.Equal(s.T(), "success", responseMap["status"])
	assert.Equal(s.T(), providerMsgID, responseMap["messageId"])
	assert.Equal(s.T(), 0.05, responseMap["cost"])
}
