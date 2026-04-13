package server

import (
	"cmp"
	"log"

	"github.com/randco/randco-microservices/shared/idempotency"

	pb "github.com/randco/randco-microservices/proto/notification/v1"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// convertNotificationToProto converts a models.Notification to pb.Notification
func convertNotificationToProto(notification *models.Notification) *pb.Notification {
	protoNotification := &pb.Notification{
		Id:        notification.ID,
		Type:      string(notification.Type),
		Status:    string(notification.Status),
		Subject:   notification.Subject,
		Content:   notification.Content,
		CreatedAt: timestamppb.New(notification.CreatedAt),
		UpdatedAt: timestamppb.New(notification.UpdatedAt),
	}

	// Set the main recipient as "to" field
	if len(notification.Recipient) > 0 {
		protoNotification.To = notification.Recipient[0].Address
	}

	// Set error message if available
	if notification.ErrorMessage != nil {
		protoNotification.ErrorMessage = *notification.ErrorMessage
	}

	if notification.SentAt != nil {
		protoNotification.SentAt = timestamppb.New(*notification.SentAt)
	}

	return protoNotification
}

// convertNotificationStatusToProto converts notification to status response
func convertNotificationStatusToProto(notification *models.Notification) *pb.NotificationStatus {
	statusResponse := &pb.NotificationStatus{
		NotificationId: notification.ID,
		Status:         string(notification.Status),
	}

	// Set error message if available
	if notification.ErrorMessage != nil {
		statusResponse.ErrorMessage = *notification.ErrorMessage
	}

	if notification.SentAt != nil {
		statusResponse.SentAt = timestamppb.New(*notification.SentAt)
	}

	return statusResponse
}

// convertEmailRequestToCreateNotification converts pb.SendEmailRequest to models.CreateNotificationRequest
func convertEmailRequestToCreateNotification(req *pb.SendEmailRequest) *models.CreateNotificationRequest {
	recipients := []models.CreateRecipientRequest{
		{
			Address: req.To,
		},
	}

	// Add CC recipients
	cc := make([]models.CreateRecipientRequest, 0)
	for _, ccAddr := range req.Cc {
		cc = append(cc, models.CreateRecipientRequest{
			Address: ccAddr,
		})
	}

	// Add BCC recipients
	bcc := make([]models.CreateRecipientRequest, 0)
	for _, bccAddr := range req.Bcc {
		bcc = append(bcc, models.CreateRecipientRequest{
			Address: bccAddr,
		})
	}

	createReq := &models.CreateNotificationRequest{
		Type:           models.NotificationTypeEmail,
		Subject:        req.Subject,
		Recipients:     recipients,
		CC:             cc,
		BCC:            bcc,
		TemplateID:     req.TemplateId,
		Variables:      req.Variables,
		IdempotencyKey: req.IdempotencyKey,
	}

	// Use html_content or text_content as content
	if req.HtmlContent != "" {
		createReq.Content = req.HtmlContent
	} else if req.TextContent != "" {
		createReq.Content = req.TextContent
	}

	if !req.ScheduledFor.AsTime().IsZero() {
		t := req.ScheduledFor.AsTime()
		createReq.ScheduledFor = &t
	}

	return createReq
}

// convertSMSRequestToCreateNotification converts pb.SendSMSRequest to models.CreateNotificationRequest
func convertSMSRequestToCreateNotification(req *pb.SendSMSRequest) *models.CreateNotificationRequest {
	recipients := []models.CreateRecipientRequest{
		{
			Address: req.To,
		},
	}

	createReq := &models.CreateNotificationRequest{
		Type:           models.NotificationTypeSMS,
		Content:        req.Content,
		Recipients:     recipients,
		TemplateID:     req.TemplateId,
		Variables:      req.Variables,
		IdempotencyKey: req.IdempotencyKey,
	}

	if !req.ScheduledFor.AsTime().IsZero() {
		t := req.ScheduledFor.AsTime()
		createReq.ScheduledFor = &t
	}

	return createReq
}

// convertPushRequestToCreateNotification converts pb.SendPushNotificationRequest to models.CreateNotificationRequest
func convertPushRequestToCreateNotification(req *pb.SendPushNotificationRequest) *models.CreateNotificationRequest {
	recipients := []models.CreateRecipientRequest{
		{
			Address: req.To,
		},
	}

	createReq := &models.CreateNotificationRequest{
		Type:       models.NotificationTypePush,
		Subject:    req.Title,
		Content:    req.Body,
		Recipients: recipients,
		TemplateID: req.TemplateId,
		Variables:  req.Variables,
	}

	if req.IdempotencyKey != "" {
		createReq.IdempotencyKey = req.IdempotencyKey
	}

	return createReq
}

// convertBulkEmailRequestToCreateNotification converts pb.SendBulkEmailRequest to models.CreateNotificationRequest
func convertBulkEmailRequestToCreateNotification(req *pb.SendBulkEmailRequest, idempotencyStore idempotency.IdempotencyStore) *[]models.CreateNotificationRequest {
	var emRequest []models.CreateNotificationRequest

	// Aggregate all recipients from all email requests
	for index, emailReq := range req.Requests {
		idempKey := idempotencyStore.BuildBulkIdempotencyKey(req.IdempotencyKey, index)

		// Use variables from individual request, fall back to empty map if nil
		variables := emailReq.Variables
		if variables == nil {
			variables = make(map[string]string)
		}

		// DEBUG: Log variables being passed
		log.Printf("[DEBUG convertBulkEmail] Request %d: TemplateID=%s, VariableCount=%d, Variables=%+v",
			index, req.TemplateId, len(variables), variables)

		r := models.CreateNotificationRequest{
			Type:           models.NotificationTypeEmail,
			Subject:        req.Subject,
			Recipients:     []models.CreateRecipientRequest{{Address: emailReq.To}},
			TemplateID:     req.TemplateId,
			Variables:      variables,
			IdempotencyKey: idempKey,
		}

		if !emailReq.ScheduledFor.AsTime().IsZero() || !req.ScheduledFor.AsTime().IsZero() {
			t := cmp.Or(emailReq.ScheduledFor.AsTime(), req.ScheduledFor.AsTime())
			r.ScheduledFor = &t
		}

		emRequest = append(emRequest, r)
	}

	return &emRequest
}

// convertBulkSMSRequestToCreateNotification converts pb.SendBulkSMSRequest to models.CreateNotificationRequest
func convertBulkSMSRequestToCreateNotification(req *pb.SendBulkSMSRequest, idempotencyStore idempotency.IdempotencyStore) *[]models.CreateNotificationRequest {
	var smsRequests []models.CreateNotificationRequest
	// Aggregate all recipients from all SMS requests
	for index, smsReq := range req.Requests {
		idempKey := idempotencyStore.BuildBulkIdempotencyKey(req.IdempotencyKey, index)

		r := models.CreateNotificationRequest{
			Type:           models.NotificationTypeSMS,
			Content:        req.Content,
			Recipients:     []models.CreateRecipientRequest{{Address: smsReq.To}},
			TemplateID:     req.TemplateId,
			Variables:      smsReq.Variables,
			IdempotencyKey: idempKey,
		}

		if !smsReq.ScheduledFor.AsTime().IsZero() || !req.ScheduledFor.AsTime().IsZero() {
			t := cmp.Or(smsReq.ScheduledFor.AsTime(), req.ScheduledFor.AsTime())
			r.ScheduledFor = &t
		}

		smsRequests = append(smsRequests, r)
	}

	return &smsRequests
}

// convertBulkPushRequestToCreateNotification converts pb.SendBulkPushRequest to models.CreateNotificationRequest
func convertBulkPushRequestToCreateNotification(req *pb.SendBulkPushRequest, idempotencyStore idempotency.IdempotencyStore) *[]models.CreateNotificationRequest {
	var pushRequests []models.CreateNotificationRequest

	for index, pushReq := range req.Requests {
		idempKey := idempotencyStore.BuildBulkIdempotencyKey(req.IdempotencyKey, index)

		r := models.CreateNotificationRequest{
			Type:           models.NotificationTypePush,
			Subject:        cmp.Or(req.Title, pushReq.Title),
			Content:        cmp.Or(req.Body, pushReq.Body),
			Recipients:     []models.CreateRecipientRequest{{Address: pushReq.To}},
			TemplateID:     req.TemplateId,
			Variables:      pushReq.Variables,
			IdempotencyKey: idempKey,
		}

		if !pushReq.ScheduledFor.AsTime().IsZero() || !req.ScheduledFor.AsTime().IsZero() {
			t := cmp.Or(pushReq.ScheduledFor.AsTime(), req.ScheduledFor.AsTime())
			r.ScheduledFor = &t
		}

		pushRequests = append(pushRequests, r)
	}

	return &pushRequests
}

// convertRetailerNotificationToProto converts a models.RetailerNotification to pb.RetailerNotification
func convertRetailerNotificationToProto(notification *models.RetailerNotification) *pb.RetailerNotification {
	protoNotification := &pb.RetailerNotification{
		Id:        notification.ID,
		Type:      string(notification.Type),
		Title:     notification.Title,
		Body:      notification.Body,
		IsRead:    notification.IsRead,
		CreatedAt: timestamppb.New(notification.CreatedAt),
	}

	// Set amount if present (convert from int64 to int64, already in pesewas)
	if notification.Amount != nil {
		protoNotification.Amount = *notification.Amount
	}

	// Set transaction ID if present
	if notification.TransactionID != nil {
		protoNotification.TransactionId = *notification.TransactionID
	}

	return protoNotification
}
