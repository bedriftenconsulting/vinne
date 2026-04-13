package models

type SendEmailRequest struct {
	To             string            `json:"to"`
	Subject        string            `json:"subject"`
	Content        string            `json:"content"`
	TemplateID     string            `json:"template_id,omitempty"`
	Variables      map[string]string `json:"variables,omitempty"`
	Priority       int               `json:"priority"`
	IdempotencyKey string            `json:"idempotency_key"`
}

type SendEmailResponse struct {
	NotificationID string `json:"notification_id"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}
