package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Prometheus counters.
type Metrics struct {
	blocksProcessed prometheus.Counter
	alertsSent      prometheus.Counter
	alertsDropped   prometheus.Counter
	errors          prometheus.Counter
}

var (
	once    sync.Once
	metrics *Metrics
)

// Init initializes global metrics (idempotent).
func Init() *Metrics {
	once.Do(func() {
		metrics = &Metrics{
			blocksProcessed: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "watch_tower_blocks_processed_total",
				Help: "Total number of blocks processed",
			}),
			alertsSent: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "watch_tower_alerts_sent_total",
				Help: "Total number of alerts sent to sinks",
			}),
			alertsDropped: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "watch_tower_alerts_dropped_total",
				Help: "Total number of alerts dropped (dedupe/rate-limit)",
			}),
			errors: prometheus.NewCounter(prometheus.CounterOpts{
				Name: "watch_tower_errors_total",
				Help: "Total number of errors encountered",
			}),
		}
		prometheus.MustRegister(
			metrics.blocksProcessed,
			metrics.alertsSent,
			metrics.alertsDropped,
			metrics.errors,
		)
	})
	return metrics
}

// BlocksProcessed increments the blocks processed counter.
func (m *Metrics) BlocksProcessed() {
	if m != nil {
		m.blocksProcessed.Inc()
	}
}

// AlertsSent increments the alerts sent counter.
func (m *Metrics) AlertsSent() {
	if m != nil {
		m.alertsSent.Inc()
	}
}

// AlertsDropped increments the alerts dropped counter.
func (m *Metrics) AlertsDropped() {
	if m != nil {
		m.alertsDropped.Inc()
	}
}

// Errors increments the errors counter.
func (m *Metrics) Errors() {
	if m != nil {
		m.errors.Inc()
	}
}

// Handler returns an HTTP handler for /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
