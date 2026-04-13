package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsInterface defines the contract for metrics operations
type MetricsInterface interface {
	// gRPC metrics
	RecordGRPCRequest(method string, duration time.Duration, requestSize, responseSize int, status string)
	IncrementActiveRequests()
	DecrementActiveRequests()

	// Notification metrics
	RecordNotificationSent(notificationType, provider string)
	RecordNotificationFailed(notificationType, provider, reason string)
	RecordNotificationDelivered(notificationType, provider string)
	RecordNotificationOpened(notificationType, provider string)
	RecordNotificationClicked(notificationType, provider string)
	RecordNotificationBounced(notificationType, provider, reason string)

	// Bulk notification metrics
	RecordBulkNotification(notificationType string, batchSize int, duration time.Duration)
	RecordBulkNotificationStart(notificationType string, batchSize int)
	RecordBulkNotificationComplete(notificationType string, duration time.Duration)

	// Queue metrics
	RecordQueueEnqueue(notificationType string, duration time.Duration)
	RecordQueueProcess(notificationType, provider string, duration time.Duration)

	// Template metrics
	RecordTemplateProcessing(templateID, notificationType string, duration time.Duration)
	RecordTemplateError(templateID, notificationType, errorType string)

	// Provider metrics
	RecordProviderRequest(provider, notificationType string, duration time.Duration, status int)
	RecordProviderError(provider, notificationType, errorType string)

	// Idempotency metrics
	RecordIdempotency(notificationType string, isHit bool)
}

// NoOpMetrics implements MetricsInterface with no-op methods for when metrics are disabled
type NoOpMetrics struct{}

func (n *NoOpMetrics) RecordGRPCRequest(method string, duration time.Duration, requestSize, responseSize int, status string) {
}
func (n *NoOpMetrics) IncrementActiveRequests()                                            {}
func (n *NoOpMetrics) DecrementActiveRequests()                                            {}
func (n *NoOpMetrics) RecordNotificationSent(notificationType, provider string)            {}
func (n *NoOpMetrics) RecordNotificationFailed(notificationType, provider, reason string)  {}
func (n *NoOpMetrics) RecordNotificationDelivered(notificationType, provider string)       {}
func (n *NoOpMetrics) RecordNotificationOpened(notificationType, provider string)          {}
func (n *NoOpMetrics) RecordNotificationClicked(notificationType, provider string)         {}
func (n *NoOpMetrics) RecordNotificationBounced(notificationType, provider, reason string) {}
func (n *NoOpMetrics) RecordBulkNotification(notificationType string, batchSize int, duration time.Duration) {
}
func (n *NoOpMetrics) RecordBulkNotificationStart(notificationType string, batchSize int) {}
func (n *NoOpMetrics) RecordBulkNotificationComplete(notificationType string, duration time.Duration) {
}
func (n *NoOpMetrics) RecordQueueEnqueue(notificationType string, duration time.Duration)           {}
func (n *NoOpMetrics) RecordQueueProcess(notificationType, provider string, duration time.Duration) {}
func (n *NoOpMetrics) RecordTemplateProcessing(templateID, notificationType string, duration time.Duration) {
}
func (n *NoOpMetrics) RecordTemplateError(templateID, notificationType, errorType string) {}
func (n *NoOpMetrics) RecordProviderRequest(provider, notificationType string, duration time.Duration, status int) {
}
func (n *NoOpMetrics) RecordProviderError(provider, notificationType, errorType string) {}
func (n *NoOpMetrics) RecordIdempotency(notificationType string, isHit bool)            {}

// Metrics holds all Prometheus metrics for notification service
type Metrics struct {
	env string
	// gRPC metrics
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestSize     *prometheus.HistogramVec
	ResponseSize    *prometheus.HistogramVec
	ActiveRequests  prometheus.Gauge

	// Notification metrics
	NotificationSentTotal      *prometheus.CounterVec
	NotificationFailedTotal    *prometheus.CounterVec
	NotificationDeliveredTotal *prometheus.CounterVec
	NotificationOpenedTotal    *prometheus.CounterVec
	NotificationClickedTotal   *prometheus.CounterVec
	NotificationBouncedTotal   *prometheus.CounterVec

	// Bulk notification metrics
	BulkNotificationRequestsTotal *prometheus.CounterVec
	BulkNotificationDuration      *prometheus.HistogramVec
	BulkNotificationBatchSize     *prometheus.HistogramVec

	// Queue metrics
	QueueSize        *prometheus.GaugeVec
	QueueEnqueueTime *prometheus.HistogramVec
	QueueProcessTime *prometheus.HistogramVec

	// Template metrics
	TemplateProcessingTime *prometheus.HistogramVec
	TemplateErrors         *prometheus.CounterVec

	// Provider metrics
	ProviderRequestsTotal   *prometheus.CounterVec
	ProviderRequestDuration *prometheus.HistogramVec
	ProviderErrors          *prometheus.CounterVec

	// Idempotency metrics
	IdempotencyHits   *prometheus.CounterVec
	IdempotencyMisses *prometheus.CounterVec
}

// NewMetricsInstance creates the appropriate metrics implementation based on enabled flag
func NewMetricsInstance(enabled bool, env string) MetricsInterface {
	if !enabled {
		return &NoOpMetrics{}
	}
	return NewMetrics(env)
}

// NewMetrics creates and registers all metrics
func NewMetrics(env string) *Metrics {
	m := &Metrics{
		env: env,
		// gRPC metrics
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_service_requests_total",
				Help: "Total number of gRPC requests",
			},
			[]string{"method", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_service_request_duration_seconds",
				Help:    "gRPC request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		RequestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_service_request_size_bytes",
				Help:    "gRPC request size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method"},
		),
		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_service_response_size_bytes",
				Help:    "gRPC response size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method"},
		),
		ActiveRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "notification_service_active_requests",
				Help: "Number of active gRPC requests",
			},
		),

		// Notification metrics
		NotificationSentTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_sent_total",
				Help: "Total number of notifications sent",
			},
			[]string{"type", "provider"},
		),
		NotificationFailedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_failed_total",
				Help: "Total number of notifications that failed",
			},
			[]string{"type", "provider", "reason"},
		),
		NotificationDeliveredTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_delivered_total",
				Help: "Total number of notifications delivered",
			},
			[]string{"type", "provider"},
		),
		NotificationOpenedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_opened_total",
				Help: "Total number of notifications opened by recipients",
			},
			[]string{"type", "provider"},
		),
		NotificationClickedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_clicked_total",
				Help: "Total number of notifications clicked by recipients",
			},
			[]string{"type", "provider"},
		),
		NotificationBouncedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_bounced_total",
				Help: "Total number of notifications that bounced",
			},
			[]string{"type", "provider", "reason"},
		),

		// Bulk notification metrics
		BulkNotificationRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bulk_notification_requests_total",
				Help: "Total number of bulk notification requests",
			},
			[]string{"type"},
		),
		BulkNotificationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bulk_notification_duration_seconds",
				Help:    "Time taken to process bulk notifications",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"type"},
		),
		BulkNotificationBatchSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bulk_notification_batch_size",
				Help:    "Number of notifications in bulk requests",
				Buckets: []float64{1, 10, 50, 100, 500, 1000, 5000, 10000},
			},
			[]string{"type"},
		),

		// Queue metrics
		QueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "notification_queue_size",
				Help: "Current size of notification queues",
			},
			[]string{"type", "status"},
		),
		QueueEnqueueTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_queue_enqueue_duration_seconds",
				Help:    "Time taken to enqueue notifications",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"type"},
		),
		QueueProcessTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_queue_process_duration_seconds",
				Help:    "Time taken to process queued notifications",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"type", "provider"},
		),

		// Template metrics
		TemplateProcessingTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_template_processing_duration_seconds",
				Help:    "Time taken to process notification templates",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"template_id", "type"},
		),
		TemplateErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_template_errors_total",
				Help: "Total number of template processing errors",
			},
			[]string{"template_id", "type", "error_type"},
		),

		// Provider metrics
		ProviderRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_provider_requests_total",
				Help: "Total number of requests to notification providers",
			},
			[]string{"provider", "type", "status"},
		),
		ProviderRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "notification_provider_request_duration_seconds",
				Help:    "Time taken for provider requests",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"provider", "type"},
		),
		ProviderErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_provider_errors_total",
				Help: "Total number of provider errors",
			},
			[]string{"provider", "type", "error_type"},
		),

		// Idempotency metrics
		IdempotencyHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_idempotency_hits_total",
				Help: "Total number of idempotency hits (duplicates found)",
			},
			[]string{"type"},
		),
		IdempotencyMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notification_idempotency_misses_total",
				Help: "Total number of idempotency misses (new requests)",
			},
			[]string{"type"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.RequestSize,
		m.ResponseSize,
		m.ActiveRequests,
		m.NotificationSentTotal,
		m.NotificationFailedTotal,
		m.NotificationDeliveredTotal,
		m.NotificationOpenedTotal,
		m.NotificationClickedTotal,
		m.NotificationBouncedTotal,
		m.BulkNotificationRequestsTotal,
		m.BulkNotificationDuration,
		m.BulkNotificationBatchSize,
		m.QueueSize,
		m.QueueEnqueueTime,
		m.QueueProcessTime,
		m.TemplateProcessingTime,
		m.TemplateErrors,
		m.ProviderRequestsTotal,
		m.ProviderRequestDuration,
		m.ProviderErrors,
		m.IdempotencyHits,
		m.IdempotencyMisses,
	)

	return m
}

// RecordGRPCRequest records gRPC request metrics
func (m *Metrics) RecordGRPCRequest(method string, duration time.Duration, requestSize, responseSize int, status string) {
	m.RequestsTotal.WithLabelValues(method, status).Inc()
	m.RequestDuration.WithLabelValues(method).Observe(duration.Seconds())
	m.RequestSize.WithLabelValues(method).Observe(float64(requestSize))
	m.ResponseSize.WithLabelValues(method).Observe(float64(responseSize))
}

// IncrementActiveRequests increments the active requests counter
func (m *Metrics) IncrementActiveRequests() {
	m.ActiveRequests.Inc()
}

// DecrementActiveRequests decrements the active requests counter
func (m *Metrics) DecrementActiveRequests() {
	m.ActiveRequests.Dec()
}

// RecordNotificationSent records a successfully sent notification
func (m *Metrics) RecordNotificationSent(notificationType, provider string) {
	m.NotificationSentTotal.WithLabelValues(notificationType, provider).Inc()
}

// RecordNotificationFailed records a failed notification
func (m *Metrics) RecordNotificationFailed(notificationType, provider, reason string) {
	m.NotificationFailedTotal.WithLabelValues(notificationType, provider, reason).Inc()
}

// RecordNotificationDelivered records a delivered notification
func (m *Metrics) RecordNotificationDelivered(notificationType, provider string) {
	m.NotificationDeliveredTotal.WithLabelValues(notificationType, provider).Inc()
}

// RecordBulkNotification records bulk notification metrics
func (m *Metrics) RecordBulkNotification(notificationType string, batchSize int, duration time.Duration) {
	m.BulkNotificationRequestsTotal.WithLabelValues(notificationType).Inc()
	m.BulkNotificationDuration.WithLabelValues(notificationType).Observe(duration.Seconds())
	m.BulkNotificationBatchSize.WithLabelValues(notificationType).Observe(float64(batchSize))
}

// RecordQueueOperation records queue metrics
func (m *Metrics) RecordQueueEnqueue(notificationType string, duration time.Duration) {
	m.QueueEnqueueTime.WithLabelValues(notificationType).Observe(duration.Seconds())
}

// RecordQueueProcess records queue processing metrics
func (m *Metrics) RecordQueueProcess(notificationType, provider string, duration time.Duration) {
	m.QueueProcessTime.WithLabelValues(notificationType, provider).Observe(duration.Seconds())
}

// RecordTemplateProcessing records template processing metrics
func (m *Metrics) RecordTemplateProcessing(templateID, notificationType string, duration time.Duration) {
	m.TemplateProcessingTime.WithLabelValues(templateID, notificationType).Observe(duration.Seconds())
}

// RecordTemplateError records template processing errors
func (m *Metrics) RecordTemplateError(templateID, notificationType, errorType string) {
	m.TemplateErrors.WithLabelValues(templateID, notificationType, errorType).Inc()
}

// RecordProviderRequest records provider request metrics
func (m *Metrics) RecordProviderRequest(provider, notificationType string, duration time.Duration, status int) {
	m.ProviderRequestsTotal.WithLabelValues(provider, notificationType, strconv.Itoa(status)).Inc()
	m.ProviderRequestDuration.WithLabelValues(provider, notificationType).Observe(duration.Seconds())
}

// RecordProviderError records provider errors
func (m *Metrics) RecordProviderError(provider, notificationType, errorType string) {
	m.ProviderErrors.WithLabelValues(provider, notificationType, errorType).Inc()
}

// RecordIdempotency records idempotency hits/misses
func (m *Metrics) RecordIdempotency(notificationType string, isHit bool) {
	if isHit {
		m.IdempotencyHits.WithLabelValues(notificationType).Inc()
	} else {
		m.IdempotencyMisses.WithLabelValues(notificationType).Inc()
	}
}

// UpdateQueueSize updates the current queue size
func (m *Metrics) UpdateQueueSize(notificationType, status string, size int) {
	m.QueueSize.WithLabelValues(notificationType, status).Set(float64(size))
}

// RecordNotificationOpened records when a notification is opened by recipient
func (m *Metrics) RecordNotificationOpened(notificationType, provider string) {
	m.NotificationOpenedTotal.WithLabelValues(notificationType, provider).Inc()
}

// RecordNotificationClicked records when a notification is clicked by recipient
func (m *Metrics) RecordNotificationClicked(notificationType, provider string) {
	m.NotificationClickedTotal.WithLabelValues(notificationType, provider).Inc()
}

// RecordNotificationBounced records when a notification bounces
func (m *Metrics) RecordNotificationBounced(notificationType, provider, reason string) {
	m.NotificationBouncedTotal.WithLabelValues(notificationType, provider, reason).Inc()
}

// GetEnvironment returns the environment this metrics instance is for
func (m *Metrics) GetEnvironment() string {
	return m.env
}

// RecordBulkNotificationStart records the start of a bulk notification operation
func (m *Metrics) RecordBulkNotificationStart(notificationType string, batchSize int) {
	m.BulkNotificationRequestsTotal.WithLabelValues(notificationType).Inc()
	m.BulkNotificationBatchSize.WithLabelValues(notificationType).Observe(float64(batchSize))
}

// RecordBulkNotificationComplete records the completion time of a bulk notification operation
func (m *Metrics) RecordBulkNotificationComplete(notificationType string, duration time.Duration) {
	m.BulkNotificationDuration.WithLabelValues(notificationType).Observe(duration.Seconds())
}
