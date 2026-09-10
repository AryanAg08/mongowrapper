package mongowrapper

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/event"
)

const metricsNamespace = "mongowrapper"

var (
	// CommandsTotal counts MongoDB commands executed through the wrapper, labeled by
	// command name (e.g. "find", "insert") and outcome ("success" or "failure").
	CommandsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "commands_total",
			Help:      "Total number of MongoDB commands executed, labeled by command and status.",
		},
		[]string{"command", "status"},
	)

	// CommandDuration observes how long MongoDB commands take, labeled by command name.
	CommandDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "command_duration_seconds",
			Help:      "Duration of MongoDB commands in seconds, labeled by command.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"command"},
	)

	// PoolConnectionsOpen is the current number of open connections in the pool.
	PoolConnectionsOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "pool_connections_open",
			Help:      "Current number of open connections in the MongoDB connection pool.",
		},
	)

	// PoolConnectionsInUse is the current number of connections checked out of the pool.
	PoolConnectionsInUse = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Name:      "pool_connections_in_use",
			Help:      "Current number of connections checked out of the MongoDB connection pool.",
		},
	)

	// PoolEventsTotal counts connection pool events, labeled by event type.
	PoolEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "pool_events_total",
			Help:      "Total number of MongoDB connection pool events, labeled by type.",
		},
		[]string{"type"},
	)

	registerMetricsOnce sync.Once
)

// registerMetrics registers the wrapper's collectors with reg. Safe to call from
// multiple Connect calls; registration only happens once.
func registerMetrics(reg prometheus.Registerer) {
	registerMetricsOnce.Do(func() {
		reg.MustRegister(CommandsTotal, CommandDuration, PoolConnectionsOpen, PoolConnectionsInUse, PoolEventsTotal)
	})
}

// commandMonitor records command counts and durations for every command the driver sends.
func commandMonitor() *event.CommandMonitor {
	return &event.CommandMonitor{
		Succeeded: func(_ context.Context, evt *event.CommandSucceededEvent) {
			CommandsTotal.WithLabelValues(evt.CommandName, "success").Inc()
			CommandDuration.WithLabelValues(evt.CommandName).Observe(evt.Duration.Seconds())
		},
		Failed: func(_ context.Context, evt *event.CommandFailedEvent) {
			CommandsTotal.WithLabelValues(evt.CommandName, "failure").Inc()
			CommandDuration.WithLabelValues(evt.CommandName).Observe(evt.Duration.Seconds())
		},
	}
}

// poolMonitor tracks connection pool size and checkout activity.
func poolMonitor() *event.PoolMonitor {
	return &event.PoolMonitor{
		Event: func(evt *event.PoolEvent) {
			PoolEventsTotal.WithLabelValues(evt.Type).Inc()
			switch evt.Type {
			case event.ConnectionCreated:
				PoolConnectionsOpen.Inc()
			case event.ConnectionClosed:
				PoolConnectionsOpen.Dec()
			case event.GetSucceeded:
				PoolConnectionsInUse.Inc()
			case event.ConnectionReturned:
				PoolConnectionsInUse.Dec()
			}
		},
	}
}
