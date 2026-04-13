package metrics

import (
	"testing"
	"time"
)

func TestNoOpMetrics(t *testing.T) {
	// Test that NoOpMetrics doesn't panic when called
	noOp := &NoOpMetrics{}

	// Test all methods to ensure they don't panic or error
	noOp.RecordGRPCRequest("test", time.Second, 100, 200, "success")
	noOp.IncrementActiveRequests()
	noOp.DecrementActiveRequests()
	noOp.RecordNotificationSent("email", "mailgun")
	noOp.RecordNotificationFailed("email", "mailgun", "test error")
	noOp.RecordNotificationDelivered("email", "mailgun")
	noOp.RecordNotificationOpened("email", "mailgun")
	noOp.RecordNotificationClicked("email", "mailgun")
	noOp.RecordNotificationBounced("email", "mailgun", "bounce reason")
	noOp.RecordBulkNotification("email", 100, time.Second)
	noOp.RecordBulkNotificationStart("email", 100)
	noOp.RecordBulkNotificationComplete("email", time.Second)
	noOp.RecordQueueEnqueue("email", time.Millisecond)
	noOp.RecordQueueProcess("email", "mailgun", time.Millisecond)
	noOp.RecordTemplateProcessing("welcome", "email", time.Millisecond)
	noOp.RecordTemplateError("welcome", "email", "parse error")
	noOp.RecordProviderRequest("mailgun", "email", time.Second, 200)
	noOp.RecordProviderError("mailgun", "email", "network error")
	noOp.RecordIdempotency("email", true)

	t.Log("NoOpMetrics successfully handled all method calls without errors")
}

func TestNewMetricsInstance(t *testing.T) {
	// Test disabled metrics returns NoOpMetrics
	disabledMetrics := NewMetricsInstance(false, "test")
	if _, ok := disabledMetrics.(*NoOpMetrics); !ok {
		t.Error("Expected NoOpMetrics when disabled, got different type")
	}

	// Test enabled metrics returns real Metrics
	enabledMetrics := NewMetricsInstance(true, "test")
	if _, ok := enabledMetrics.(*Metrics); !ok {
		t.Error("Expected *Metrics when enabled, got different type")
	}
}
