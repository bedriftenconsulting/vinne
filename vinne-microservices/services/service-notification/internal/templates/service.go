package templates

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/models"
)

type NotificationTemplateService struct {
	renderer    *TemplateRenderer
	templateDir string
}

func NewNotificationTemplateService(templateDir string) (*NotificationTemplateService, error) {
	if templateDir == "" {
		templateDir = DefaultTemplatePath
	}

	renderer := NewTemplateRenderer(templateDir)
	if err := renderer.LoadAllTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return &NotificationTemplateService{
		renderer:    renderer,
		templateDir: templateDir,
	}, nil
}

func NewDefaultNotificationTemplateService() (*NotificationTemplateService, error) {
	return NewNotificationTemplateService(DefaultTemplatePath)
}

func (nts *NotificationTemplateService) RenderWelcomeEmail(userData map[string]string) (string, error) {
	defaultData := TemplateData{
		"CompanyName":  "RAND Lottery",
		"CurrentYear":  fmt.Sprintf("%d", time.Now().Year()),
		"SupportUrl":   "https://support.randlottery.com",
		"SupportEmail": "support@randlottery.com",
		"LoginUrl":     "https://app.randlottery.com/login",
	}

	for k, v := range userData {
		defaultData[k] = v
	}

	return nts.renderer.RenderTemplate("welcome", defaultData)
}

func (nts *NotificationTemplateService) RenderPasswordResetEmail(userData map[string]string) (string, error) {
	defaultData := TemplateData{
		"CompanyName":  "RAND Lottery",
		"CurrentYear":  fmt.Sprintf("%d", time.Now().Year()),
		"SupportUrl":   "https://support.randlottery.com",
		"SupportEmail": "support@randlottery.com",
		"ResetUrl":     "https://app.randlottery.com/reset-password",
		"ExpiryHours":  "24",
	}

	for k, v := range userData {
		defaultData[k] = v
	}

	return nts.renderer.RenderTemplate("password_reset", defaultData)
}

func (nts *NotificationTemplateService) RenderSMSTemplate(templateName TemplateName, userData map[string]string) (string, error) {
	templ := SMSTemplates[templateName]
	if templ.Path == "" {
		return "", fmt.Errorf("SMS template %s not found", templateName)
	}

	fullPath := filepath.Join(nts.templateDir, templ.Path)
	content, err := LoadHTMLFromFile(fullPath)
	if err != nil {
		return "", err
	}

	defaultData := TemplateData{
		"CompanyName": "RAND Lottery",
	}

	for k, v := range userData {
		defaultData[k] = v
	}

	return ReplacePlaceholders(content, defaultData), nil
}

func (nts *NotificationTemplateService) RenderPushTemplate(templateName TemplateName, userData map[string]string) (string, error) {
	templatePl := PushTemplates[templateName]
	if templatePl.Path == "" {
		return "", fmt.Errorf("push template %s not found", templateName)
	}

	fullPath := filepath.Join(nts.templateDir, templatePl.Path)
	content, err := LoadHTMLFromFile(fullPath)
	if err != nil {
		return "", err
	}

	defaultData := TemplateData{
		"Timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	for k, v := range userData {
		defaultData[k] = v
	}

	return ReplacePlaceholders(content, defaultData), nil
}

func (nts *NotificationTemplateService) RenderTemplateByType(
	templateType models.NotificationType,
	templateName TemplateName,
	userData map[string]string,
) (string, error) {
	switch templateType {
	case models.NotificationTypeEmail:
		switch templateName {
		case TemplateNameWelcome:
			return nts.RenderWelcomeEmail(userData)
		case TemplateNamePasswordReset:
			return nts.RenderPasswordResetEmail(userData)
		default:
			return "", fmt.Errorf("unknown email template: %s", templateName)
		}
	case models.NotificationTypeSMS:
		return nts.RenderSMSTemplate(templateName, userData)
	case models.NotificationTypePush:
		return nts.RenderPushTemplate(templateName, userData)
	default:
		return "", fmt.Errorf("unsupported notification type: %s", templateType)
	}
}

func (nts *NotificationTemplateService) RenderEmailTemplate(templateName TemplateName, userData map[string]string) (string, error) {
	switch templateName {
	case TemplateNameWelcome:
		return nts.RenderWelcomeEmail(userData)
	case TemplateNamePasswordReset:
		return nts.RenderPasswordResetEmail(userData)
	default:
		return "", fmt.Errorf("unknown email template: %s", templateName)
	}
}
