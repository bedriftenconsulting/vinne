package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/config"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockEmailProvider struct {
	mock.Mock
}

func (m *MockEmailProvider) SendEmail(ctx context.Context, req *EmailRequest) (*EmailResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmailResponse), args.Error(1)
}

func (m *MockEmailProvider) GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DeliveryStatus), args.Error(1)
}

func (m *MockEmailProvider) ValidateAddress(ctx context.Context, email string) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *MockEmailProvider) GetProviderName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockEmailProvider) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type MockSMSProvider struct {
	mock.Mock
}

func (m *MockSMSProvider) SendSMS(ctx context.Context, req *SMSRequest) (*SMSResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SMSResponse), args.Error(1)
}

func (m *MockSMSProvider) GetStatus(ctx context.Context, messageID string) (*DeliveryStatus, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DeliveryStatus), args.Error(1)
}

func (m *MockSMSProvider) ValidateNumber(ctx context.Context, phone string) error {
	args := m.Called(ctx, phone)
	return args.Error(0)
}

func (m *MockSMSProvider) GetProviderName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSMSProvider) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestNewProviderManager(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.ProviderConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration with mailgun enabled",
			config: &config.ProviderConfig{
				Email: config.EmailProviderConfig{
					DefaultProvider: "mailgun",
					Mailgun: config.MailgunProviderConfig{
						Enabled: true,
						APIKey:  "test-api-key",
						Domain:  "test-domain.com",
						BaseURL: "https://api.mailgun.net/v3",
					},
				},
				SMS: config.SMSProviderConfig{
					DefaultProvider: "hubtel",
					Hubtel: config.HubtelProviderConfig{
						Enabled:      false,
						ClientID:     "",
						ClientSecret: "",
						BaseURL:      "",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid configuration with hubtel enabled",
			config: &config.ProviderConfig{
				Email: config.EmailProviderConfig{
					DefaultProvider: "mailgun",
					Mailgun: config.MailgunProviderConfig{
						Enabled: false,
						APIKey:  "",
						Domain:  "",
						BaseURL: "",
					},
				},
				SMS: config.SMSProviderConfig{
					DefaultProvider: "hubtel",
					Hubtel: config.HubtelProviderConfig{
						Enabled:      true,
						ClientID:     "test-client-id",
						ClientSecret: "test-client-secret",
						BaseURL:      "https://smsc.hubtel.com/v1/messages/send",
					},
				},
			},
			expectError: false,
		},
		{
			name: "mailgun enabled but missing api key",
			config: &config.ProviderConfig{
				Email: config.EmailProviderConfig{
					DefaultProvider: "mailgun",
					Mailgun: config.MailgunProviderConfig{
						Enabled: true,
						APIKey:  "",
						Domain:  "test-domain.com",
						BaseURL: "https://api.mailgun.net/v3",
					},
				},
				SMS: config.SMSProviderConfig{
					DefaultProvider: "hubtel",
					Hubtel: config.HubtelProviderConfig{
						Enabled: false,
					},
				},
			},
			expectError: true,
			errorMsg:    "mailgun configuration incomplete: missing api_key or domain",
		},
		{
			name: "mailgun enabled but missing domain",
			config: &config.ProviderConfig{
				Email: config.EmailProviderConfig{
					DefaultProvider: "mailgun",
					Mailgun: config.MailgunProviderConfig{
						Enabled: true,
						APIKey:  "test-api-key",
						Domain:  "",
						BaseURL: "https://api.mailgun.net/v3",
					},
				},
				SMS: config.SMSProviderConfig{
					DefaultProvider: "hubtel",
					Hubtel: config.HubtelProviderConfig{
						Enabled: false,
					},
				},
			},
			expectError: true,
			errorMsg:    "mailgun configuration incomplete: missing api_key or domain",
		},
		{
			name: "hubtel enabled but missing client id",
			config: &config.ProviderConfig{
				Email: config.EmailProviderConfig{
					DefaultProvider: "mailgun",
					Mailgun: config.MailgunProviderConfig{
						Enabled: false,
					},
				},
				SMS: config.SMSProviderConfig{
					DefaultProvider: "hubtel",
					Hubtel: config.HubtelProviderConfig{
						Enabled:      true,
						ClientID:     "",
						ClientSecret: "test-client-secret",
						BaseURL:      "https://smsc.hubtel.com/v1/messages/send",
					},
				},
			},
			expectError: true,
			errorMsg:    "hubtel configuration incomplete: missing client_id or client_secret",
		},
		{
			name: "hubtel enabled but missing client secret",
			config: &config.ProviderConfig{
				Email: config.EmailProviderConfig{
					DefaultProvider: "mailgun",
					Mailgun: config.MailgunProviderConfig{
						Enabled: false,
					},
				},
				SMS: config.SMSProviderConfig{
					DefaultProvider: "hubtel",
					Hubtel: config.HubtelProviderConfig{
						Enabled:      true,
						ClientID:     "test-client-id",
						ClientSecret: "",
						BaseURL:      "https://smsc.hubtel.com/v1/messages/send",
					},
				},
			},
			expectError: true,
			errorMsg:    "hubtel configuration incomplete: missing client_id or client_secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testLogger := logger.NewLogger(logger.Config{
				Level:       "info",
				Format:      "text",
				ServiceName: "test",
			})
			rateLimitCfg := config.RateLimitConfig{
				EmailRatePerHour: 100,
				SMSRatePerMinute: 60,
			}
			// Create a test Redis client (nil is fine for these tests as they don't test rate limiting)
			testRedis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
			manager, err := NewProviderManager(tt.config, rateLimitCfg, testRedis, testLogger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, manager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)
				assert.NotNil(t, manager.factory)
				assert.NotNil(t, manager.healthChecker)
				assert.Equal(t, tt.config, manager.config)
			}
		})
	}
}

func TestProviderManager_GetEmailProvider(t *testing.T) {
	cfg := &config.ProviderConfig{
		Email: config.EmailProviderConfig{
			DefaultProvider: "mailgun",
			Mailgun: config.MailgunProviderConfig{
				Enabled: true,
				APIKey:  "test-api-key",
				Domain:  "test-domain.com",
				BaseURL: "https://api.mailgun.net/v3",
			},
		},
		SMS: config.SMSProviderConfig{
			DefaultProvider: "hubtel",
			Hubtel: config.HubtelProviderConfig{
				Enabled: false,
			},
		},
	}

	testLogger := logger.NewLogger(logger.Config{
		Level:       "info",
		Format:      "text",
		ServiceName: "test",
	})
	rateLimitCfg := config.RateLimitConfig{
		EmailRatePerHour: 100,
		SMSRatePerMinute: 60,
	}
	testRedis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	manager, err := NewProviderManager(cfg, rateLimitCfg, testRedis, testLogger)
	assert.NoError(t, err)

	tests := []struct {
		name         string
		providerName string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "get default provider",
			providerName: "",
			expectError:  false,
		},
		{
			name:         "get mailgun provider",
			providerName: "mailgun",
			expectError:  false,
		},
		{
			name:         "get non-existent provider",
			providerName: "nonexistent",
			expectError:  true,
			errorMsg:     "Email provider not found: nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := manager.GetEmailProvider(tt.providerName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestProviderManager_GetSMSProvider(t *testing.T) {
	cfg := &config.ProviderConfig{
		Email: config.EmailProviderConfig{
			DefaultProvider: "mailgun",
			Mailgun: config.MailgunProviderConfig{
				Enabled: false,
			},
		},
		SMS: config.SMSProviderConfig{
			DefaultProvider: "hubtel",
			Hubtel: config.HubtelProviderConfig{
				Enabled:      true,
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				BaseURL:      "https://smsc.hubtel.com/v1/messages/send",
			},
		},
	}

	testLogger := logger.NewLogger(logger.Config{
		Level:       "info",
		Format:      "text",
		ServiceName: "test",
	})
	rateLimitCfg := config.RateLimitConfig{
		EmailRatePerHour: 100,
		SMSRatePerMinute: 60,
	}
	testRedis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	manager, err := NewProviderManager(cfg, rateLimitCfg, testRedis, testLogger)
	assert.NoError(t, err)

	tests := []struct {
		name         string
		providerName string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "get default provider",
			providerName: "",
			expectError:  false,
		},
		{
			name:         "get hubtel provider",
			providerName: "hubtel",
			expectError:  false,
		},
		{
			name:         "get non-existent provider",
			providerName: "nonexistent",
			expectError:  true,
			errorMsg:     "SMS provider not found: nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := manager.GetSMSProvider(tt.providerName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestProviderManager_SendEmail(t *testing.T) {
	mockProvider := &MockEmailProvider{}
	mockProvider.On("GetProviderName").Return("test-email")
	mockProvider.On("SendEmail", mock.Anything, mock.Anything).Return(&EmailResponse{
		MessageID: "test-message-id",
		Status:    StatusDelivered,
		Provider:  "test-email",
		SentAt:    time.Now(),
	}, nil)

	factory := NewProviderFactory()
	factory.RegisterEmailProvider("test-email", mockProvider)

	config := &config.ProviderConfig{
		Email: config.EmailProviderConfig{
			DefaultProvider: "test-email",
		},
		SMS: config.SMSProviderConfig{
			DefaultProvider: "hubtel",
		},
	}

	manager := &ProviderManager{
		factory: factory,
		config:  config,
		healthChecker: &HealthChecker{
			providers: make(map[string]ProviderHealth),
		},
	}

	req := &EmailRequest{
		To:          "test@example.com",
		Subject:     "Test Subject",
		HTMLContent: "Test Content",
	}

	ctx := context.Background()
	response, err := manager.SendEmail(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-message-id", response.MessageID)
	assert.Equal(t, StatusDelivered, response.Status)
	assert.Equal(t, "test-email", response.Provider)

	mockProvider.AssertExpectations(t)
}

func TestProviderManager_SendSMS(t *testing.T) {
	mockProvider := &MockSMSProvider{}
	mockProvider.On("GetProviderName").Return("test-sms")
	mockProvider.On("SendSMS", mock.Anything, mock.Anything).Return(&SMSResponse{
		MessageID: "test-sms-id",
		Status:    StatusDelivered,
		Provider:  "test-sms",
		SentAt:    time.Now(),
	}, nil)

	factory := NewProviderFactory()
	factory.RegisterSMSProvider("test-sms", mockProvider)

	config := &config.ProviderConfig{
		Email: config.EmailProviderConfig{
			DefaultProvider: "mailgun",
		},
		SMS: config.SMSProviderConfig{
			DefaultProvider: "test-sms",
		},
	}

	manager := &ProviderManager{
		factory: factory,
		config:  config,
		healthChecker: &HealthChecker{
			providers: make(map[string]ProviderHealth),
		},
	}

	req := &SMSRequest{
		To:      "+1234567890",
		Content: "Test SMS",
	}

	ctx := context.Background()
	response, err := manager.SendSMS(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-sms-id", response.MessageID)
	assert.Equal(t, StatusDelivered, response.Status)
	assert.Equal(t, "test-sms", response.Provider)

	mockProvider.AssertExpectations(t)
}

func TestProviderManager_SendEmail_UnhealthyProvider(t *testing.T) {
	mockProvider := &MockEmailProvider{}
	mockProvider.On("GetProviderName").Return("test-email")

	factory := NewProviderFactory()
	factory.RegisterEmailProvider("test-email", mockProvider)

	config := &config.ProviderConfig{
		Email: config.EmailProviderConfig{
			DefaultProvider: "test-email",
		},
		SMS: config.SMSProviderConfig{
			DefaultProvider: "hubtel",
		},
	}

	manager := &ProviderManager{
		factory: factory,
		config:  config,
		healthChecker: &HealthChecker{
			providers: map[string]ProviderHealth{
				"test-email": {
					IsHealthy: false,
					LastError: errors.New("provider unhealthy"),
				},
			},
		},
	}

	req := &EmailRequest{
		To:          "test@example.com",
		Subject:     "Test Subject",
		HTMLContent: "Test Content",
	}

	ctx := context.Background()
	response, err := manager.SendEmail(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "Email provider test-email is not healthy")

	mockProvider.AssertExpectations(t)
}

func TestProviderManager_ListProviders(t *testing.T) {
	mockEmailProvider := &MockEmailProvider{}
	mockSMSProvider := &MockSMSProvider{}

	factory := NewProviderFactory()
	factory.RegisterEmailProvider("mailgun", mockEmailProvider)
	factory.RegisterSMSProvider("hubtel", mockSMSProvider)

	config := &config.ProviderConfig{
		Email: config.EmailProviderConfig{
			DefaultProvider: "mailgun",
		},
		SMS: config.SMSProviderConfig{
			DefaultProvider: "hubtel",
		},
	}

	manager := &ProviderManager{
		factory: factory,
		config:  config,
		healthChecker: &HealthChecker{
			providers: make(map[string]ProviderHealth),
		},
	}

	emailProviders := manager.ListEmailProviders()
	smsProviders := manager.ListSMSProviders()

	assert.Len(t, emailProviders, 1)
	assert.Contains(t, emailProviders, "mailgun")

	assert.Len(t, smsProviders, 1)
	assert.Contains(t, smsProviders, "hubtel")
}

func TestProviderManager_Shutdown(t *testing.T) {
	cfg := &config.ProviderConfig{
		Email: config.EmailProviderConfig{
			DefaultProvider: "mailgun",
			Mailgun: config.MailgunProviderConfig{
				Enabled: false,
			},
		},
		SMS: config.SMSProviderConfig{
			DefaultProvider: "hubtel",
			Hubtel: config.HubtelProviderConfig{
				Enabled: false,
			},
		},
	}

	testLogger := logger.NewLogger(logger.Config{
		Level:       "info",
		Format:      "text",
		ServiceName: "test",
	})
	rateLimitCfg := config.RateLimitConfig{
		EmailRatePerHour: 100,
		SMSRatePerMinute: 60,
	}
	testRedis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	manager, err := NewProviderManager(cfg, rateLimitCfg, testRedis, testLogger)
	assert.NoError(t, err)

	// Shutdown should not panic
	assert.NotPanics(t, func() {
		manager.Shutdown()
	})
}

func TestProviderFactory(t *testing.T) {
	factory := NewProviderFactory()

	mockEmailProvider := &MockEmailProvider{}
	mockSMSProvider := &MockSMSProvider{}

	// Test email provider registration
	factory.RegisterEmailProvider("test-email", mockEmailProvider)
	emailProvider, err := factory.GetEmailProvider("test-email")
	assert.NoError(t, err)
	assert.Equal(t, mockEmailProvider, emailProvider)

	// Test SMS provider registration
	factory.RegisterSMSProvider("test-sms", mockSMSProvider)
	smsProvider, err := factory.GetSMSProvider("test-sms")
	assert.NoError(t, err)
	assert.Equal(t, mockSMSProvider, smsProvider)

	// Test listing providers
	emailProviders := factory.ListEmailProviders()
	smsProviders := factory.ListSMSProviders()

	assert.Len(t, emailProviders, 1)
	assert.Contains(t, emailProviders, "test-email")

	assert.Len(t, smsProviders, 1)
	assert.Contains(t, smsProviders, "test-sms")

	// Test getting non-existent providers
	_, err = factory.GetEmailProvider("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Email provider not found: nonexistent")

	_, err = factory.GetSMSProvider("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SMS provider not found: nonexistent")
}

func TestHealthChecker(t *testing.T) {
	hc := &HealthChecker{
		providers: make(map[string]ProviderHealth),
		interval:  100 * time.Millisecond,
		stopCh:    make(chan struct{}),
	}

	// Test initial state
	health := hc.getAllHealth()
	assert.Empty(t, health)

	// Test updateHealth
	hc.updateHealth("test-provider", nil)
	health = hc.getAllHealth()
	assert.Len(t, health, 1)
	assert.True(t, health["test-provider"].IsHealthy)
	assert.Nil(t, health["test-provider"].LastError)

	// Test updateHealth with error
	testError := errors.New("test error")
	hc.updateHealth("test-provider", testError)
	health = hc.getAllHealth()
	assert.Len(t, health, 1)
	assert.False(t, health["test-provider"].IsHealthy)
	assert.Equal(t, testError, health["test-provider"].LastError)

	// Test recordError
	hc.recordError("test-provider", testError)
	health = hc.getAllHealth()
	assert.False(t, health["test-provider"].IsHealthy)
	assert.Equal(t, testError, health["test-provider"].LastError)

	// Test recordSuccess
	hc.recordSuccess("test-provider")
	health = hc.getAllHealth()
	assert.True(t, health["test-provider"].IsHealthy)
	assert.Nil(t, health["test-provider"].LastError)

	// Test isHealthy
	assert.True(t, hc.isHealthy("test-provider"))
	assert.True(t, hc.isHealthy("nonexistent")) // Should return true for non-existent providers

	// Test shutdown
	close(hc.stopCh)
}
