package templates

import (
	"fmt"

	"github.com/randco/randco-microservices/services/service-notification/internal/models"
)

type TemplateName string

const (
	TemplateNameWelcome       TemplateName = "welcome"
	TemplateNamePasswordReset TemplateName = "password_reset"
	TemplateNameVerification  TemplateName = "verification"
	TemplateNameGameEnd       TemplateName = "game_end"
	TemplateNameSalesCutoff   TemplateName = "sales_cutoff"
)

type TemplateObj struct {
	Path    string
	Subject string
}

type Template struct {
	Content      string
	Subject      string
	TemplateData TemplateData
}

var EmailTemplates = map[TemplateName]TemplateObj{
	TemplateNameWelcome:       {Path: "welcome/welcome_email.html", Subject: "Welcome to RAND Lottery!"},
	TemplateNamePasswordReset: {Path: "password_reset/password_reset.html", Subject: "Password Reset Request"},
	TemplateNameGameEnd:       {Path: "game_end/game_end_email.html", Subject: "Game Draw Completed"},
	TemplateNameSalesCutoff:   {Path: "sales_cutoff/sales_cutoff_email.html", Subject: "Game Sales Closed"},
}

var SMSTemplates = map[TemplateName]TemplateObj{
	TemplateNameWelcome:      {Path: "welcome/welcome_sms.txt"},
	TemplateNameVerification: {Path: "password_reset/verification_sms.txt"},
}

var PushTemplates = map[TemplateName]TemplateObj{
	TemplateNameWelcome: {Path: "welcome/welcome_push.json"},
}

const DefaultTemplatePath = "./internal/templates/public"

func GetTemplatePath(templateType models.NotificationType, templateName TemplateName) string {
	switch templateType {
	case models.NotificationTypeEmail:
		if template, exists := EmailTemplates[templateName]; exists {
			return template.Path
		}
	case models.NotificationTypeSMS:
		if templ, exists := SMSTemplates[templateName]; exists {
			return templ.Path
		}
	case models.NotificationTypePush:
		if templ, exists := PushTemplates[templateName]; exists {
			return templ.Path
		}
	}
	return ""
}

func ProcessTemplate(templateType models.NotificationType, templateName TemplateName, data TemplateData, templateDir ...string) (Template, error) {
	var renderer *TemplateRenderer
	var result Template

	if len(templateDir) > 0 {
		renderer = NewTemplateRenderer(templateDir[0])
	} else {
		renderer = NewDefaultTemplateRenderer()
	}

	var templateMap map[TemplateName]TemplateObj
	switch templateType {
	case models.NotificationTypeEmail:
		templateMap = EmailTemplates
	case models.NotificationTypeSMS:
		templateMap = SMSTemplates
	case models.NotificationTypePush:
		templateMap = PushTemplates
	default:
		return result, fmt.Errorf("invalid template type: %s", templateType)
	}

	template, exists := templateMap[templateName]
	if !exists {
		return result, fmt.Errorf("template %s not found for type %s", templateName, templateType)
	}

	if err := renderer.LoadTemplate(string(templateName), template.Path); err != nil {
		return result, err
	}

	renderedContent, err := renderer.RenderTemplate(string(templateName), data)
	if err != nil {
		return result, err
	}

	result = Template{
		Content:      renderedContent,
		TemplateData: data,
		Subject:      template.Subject,
	}

	return result, nil
}
