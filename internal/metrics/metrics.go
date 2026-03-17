package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds all Prometheus collectors for the Quorum service.
type Metrics struct {
	// HTTP layer
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// Request service (business)
	RequestsTotal             *prometheus.CounterVec
	RequestResolutionDuration prometheus.Observer
	PendingRequestsGauge      prometheus.Gauge

	// Webhook dispatcher
	WebhookDeliveriesTotal  *prometheus.CounterVec
	WebhookDeliveryDuration prometheus.Observer

	// Authorization hook
	AuthHookTotal    *prometheus.CounterVec
	AuthHookDuration prometheus.Observer

	// Expiry worker
	ExpiryErrorsTotal prometheus.Counter
}

// New creates a Metrics instance and registers all collectors with the given registerer.
func New(reg prometheus.Registerer) *Metrics {
	requestResolution := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "quorum",
		Subsystem: "requests",
		Name:      "resolution_duration_seconds",
		Help:      "Time from request creation to terminal state.",
		Buckets:   []float64{1, 5, 15, 30, 60, 300, 900, 1800, 3600, 7200, 86400},
	})

	authHookDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "quorum",
		Subsystem: "auth_hook",
		Name:      "duration_seconds",
		Help:      "Authorization hook call duration.",
		Buckets:   prometheus.DefBuckets,
	})

	webhookDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "quorum",
		Subsystem: "webhook",
		Name:      "delivery_duration_seconds",
		Help:      "Webhook delivery duration including retries.",
		Buckets:   prometheus.DefBuckets,
	})

	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "quorum",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests processed.",
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "quorum",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path", "status_code"},
		),

		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "quorum",
				Subsystem: "requests",
				Name:      "total",
				Help:      "Total maker-checker requests by outcome.",
			},
			[]string{"status"},
		),
		RequestResolutionDuration: requestResolution,
		PendingRequestsGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "quorum",
				Subsystem: "requests",
				Name:      "pending",
				Help:      "Current number of pending approval requests.",
			},
		),

		WebhookDeliveriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "quorum",
				Subsystem: "webhook",
				Name:      "deliveries_total",
				Help:      "Total webhook deliveries by outcome.",
			},
			[]string{"outcome"},
		),
		WebhookDeliveryDuration: webhookDuration,

		AuthHookTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "quorum",
				Subsystem: "auth_hook",
				Name:      "total",
				Help:      "Total authorization hook calls by outcome.",
			},
			[]string{"outcome"},
		),
		AuthHookDuration: authHookDuration,

		ExpiryErrorsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "quorum",
				Subsystem: "expiry",
				Name:      "errors_total",
				Help:      "Total errors during expiry processing.",
			},
		),
	}

	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.RequestsTotal,
		requestResolution,
		m.PendingRequestsGauge,
		m.WebhookDeliveriesTotal,
		webhookDuration,
		m.AuthHookTotal,
		authHookDuration,
		m.ExpiryErrorsTotal,
	)

	return m
}
