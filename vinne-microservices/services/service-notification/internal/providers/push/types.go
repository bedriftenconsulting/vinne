package push

// PushNotificationRequest represents a push notification request
type PushNotificationRequest struct {
	DeviceToken string            // FCM device token
	Title       string            // Notification title
	Body        string            // Notification body
	Data        map[string]string // Additional data payload
	Priority    string            // Priority: "normal", "high", "critical"
	ImageURL    string            // Optional image URL
}

// PushNotificationResponse represents a push notification response
type PushNotificationResponse struct {
	MessageID string // Provider message ID
	Success   bool   // Whether the send was successful
	Error     string // Error message if failed
}

// BatchPushNotificationResponse represents a batch push notification response
type BatchPushNotificationResponse struct {
	Responses    []*PushNotificationResponse // Individual responses
	SuccessCount int                         // Count of successful sends
	FailureCount int                         // Count of failed sends
}

// PushProvider defines the interface for push notification providers
type PushProvider interface {
	SendPushNotification(deviceToken, title, body string, data map[string]string) (*PushNotificationResponse, error)
	SendBatchPushNotifications(requests []*PushNotificationRequest) (*BatchPushNotificationResponse, error)
	Close() error
}
