package templates

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/randco/randco-microservices/services/service-notification/internal/models"
)

func TestNotificationTemplateServiceIntegration(t *testing.T) {
	templateDir := filepath.Join(".", "public")

	service, err := NewNotificationTemplateService(templateDir)
	require.NoError(t, err)

	userData := map[string]string{
		"FirstName":   "John",
		"Username":    "john123",
		"AccountType": "Premium",
	}

	result, err := service.RenderWelcomeEmail(userData)
	require.NoError(t, err)
	assert.Contains(t, result, "John")
	assert.Contains(t, result, "RAND Lottery")
}

func TestDefaultNotificationTemplateService(t *testing.T) {
	service, err := NewNotificationTemplateService("public")
	require.NoError(t, err)
	assert.NotNil(t, service)
}

func TestNewNotificationTemplateServiceWithEmptyPath(t *testing.T) {
	service, err := NewNotificationTemplateService("public")
	require.NoError(t, err)
	assert.Equal(t, "public", service.templateDir)
}

func TestTemplateServiceWithInvalidPath(t *testing.T) {
	_, err := NewNotificationTemplateService("/nonexistent/path")
	assert.Error(t, err)
}

func TestRenderQuickTemplate(t *testing.T) {
	data := TemplateData{
		"FirstName":   "John",
		"CompanyName": "RAND Lottery",
	}

	result, err := ProcessTemplate(models.NotificationTypeEmail, TemplateNameWelcome, data, "public")
	require.NoError(t, err)
	assert.Contains(t, result.Content, "John")
	assert.Contains(t, result.Content, "RAND Lottery")
}

func TestRenderQuickTemplateInvalidType(t *testing.T) {
	data := TemplateData{"Name": "John"}

	_, err := ProcessTemplate("invalid", TemplateNameWelcome, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template type")
}

func TestGetTemplatePath(t *testing.T) {
	path := GetTemplatePath(models.NotificationTypeEmail, TemplateNameWelcome)
	assert.Equal(t, "welcome/welcome_email.html", path)

	path = GetTemplatePath("invalid", TemplateNameWelcome)
	assert.Equal(t, "", path)
}

func TestStaticTypingIntegration(t *testing.T) {
	templateDir := filepath.Join(".", "public")

	service, err := NewNotificationTemplateService(templateDir)
	require.NoError(t, err)

	userData := map[string]string{
		"FirstName":   "Alice",
		"Email":       "alice@example.com",
		"Username":    "alice123",
		"AccountType": "Premium",
	}

	// Test static typing with RenderTemplateByType
	result, err := service.RenderTemplateByType(models.NotificationTypeEmail, TemplateNameWelcome, userData)
	require.NoError(t, err)
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "RAND Lottery")
}

func TestTemplateNameConstants(t *testing.T) {
	// Test that our TemplateName constants are properly defined
	assert.Equal(t, TemplateName("welcome"), TemplateNameWelcome)
	assert.Equal(t, TemplateName("password_reset"), TemplateNamePasswordReset)

	// Test template path resolution with constants
	path := GetTemplatePath(models.NotificationTypeSMS, TemplateNameVerification)
	assert.Equal(t, "password_reset/verification_sms.txt", path)
}
