package mongowrapper

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
)

const metricsNamespace = "mongowrapper"

var (
	// CommandsTotal counts MongoDB commands executed through the wrapper, labeled by
	// command name (e.g. "find", "insert"), the database and collection it targeted,
	// and outcome ("success" or "failure").
	CommandsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Name:      "commands_total",
			Help:      "Total number of MongoDB commands executed, labeled by command, database, collection and status.",
		},
		[]string{"command", "database", "collection", "status"},
	)

	// CommandDuration observes how long MongoDB commands take, labeled by command,
	// database and collection.
	CommandDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Name:      "command_duration_seconds",
			Help:      "Duration of MongoDB commands in seconds, labeled by command, database and collection.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"command", "database", "collection"},
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

// UnknownCollection labels commands that target no collection: hello, ping,
// endSessions, the authentication handshake. An explicit placeholder beats an
// empty string, which reads as a bug when it shows up in a dashboard legend.
const UnknownCollection = "-"

// maxInFlightCommands bounds the correlation map below. The driver emits exactly
// one finished event per started event, so in practice the map only ever holds
// genuinely in-flight commands. The cap means that if a finished event is ever
// dropped, the cost is a missing collection label rather than memory that grows
// for the life of the process.
const maxInFlightCommands = 4096

// The collection is only available on the *started* event, which carries the
// command document; the succeeded and failed events carry the duration but not
// the document. Correlating the two on RequestID is what makes it possible to
// label latency by collection.
//
// The database needs no correlation -- CommandFinishedEvent carries
// DatabaseName directly.
var inFlightCommands = struct {
	sync.Mutex
	m map[int64]string
}{m: make(map[int64]string)}

func rememberCollection(requestID int64, collection string) {
	inFlightCommands.Lock()
	defer inFlightCommands.Unlock()
	if len(inFlightCommands.m) >= maxInFlightCommands {
		return
	}
	inFlightCommands.m[requestID] = collection
}

func takeCollection(requestID int64) string {
	inFlightCommands.Lock()
	defer inFlightCommands.Unlock()
	c, ok := inFlightCommands.m[requestID]
	if !ok {
		return UnknownCollection
	}
	delete(inFlightCommands.m, requestID)
	return c
}

// collectionFromCommand extracts the collection name from a command document.
//
// For nearly every command the first key is the command name and its value is
// the collection: {find: "users", filter: {...}}. getMore is the exception --
// its command value is a cursor ID, and the collection sits in a separate
// "collection" field, so that is checked first. Commands that name no
// collection at all fall through to the placeholder.
func collectionFromCommand(cmd bson.Raw, commandName string) string {
	if v, err := cmd.LookupErr("collection"); err == nil {
		if s, ok := v.StringValueOK(); ok && s != "" {
			return s
		}
	}
	if v, err := cmd.LookupErr(commandName); err == nil {
		if s, ok := v.StringValueOK(); ok && s != "" {
			return s
		}
	}
	return UnknownCollection
}

// commandMonitor records command counts and durations for every command the driver sends.
func commandMonitor() *event.CommandMonitor {
	return &event.CommandMonitor{
		Started: func(_ context.Context, evt *event.CommandStartedEvent) {
			rememberCollection(evt.RequestID, collectionFromCommand(evt.Command, evt.CommandName))
		},
		Succeeded: func(_ context.Context, evt *event.CommandSucceededEvent) {
			observeCommand(evt.CommandName, evt.DatabaseName, takeCollection(evt.RequestID),
				"success", evt.Duration.Seconds())
		},
		Failed: func(_ context.Context, evt *event.CommandFailedEvent) {
			observeCommand(evt.CommandName, evt.DatabaseName, takeCollection(evt.RequestID),
				"failure", evt.Duration.Seconds())
		},
	}
}

func observeCommand(command, database, collection, status string, seconds float64) {
	CommandsTotal.WithLabelValues(command, database, collection, status).Inc()
	CommandDuration.WithLabelValues(command, database, collection).Observe(seconds)
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
