package mongowrapper

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
)

func mustRaw(t *testing.T, d bson.D) bson.Raw {
	t.Helper()
	b, err := bson.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bson.Raw(b)
}

// Most commands name their collection as the value of the command key itself.
func TestCollectionFromCommand_NamedAfterTheCommand(t *testing.T) {
	cases := []struct {
		name, command string
		doc           bson.D
		want          string
	}{
		{"find", "find", bson.D{{Key: "find", Value: "users"}, {Key: "filter", Value: bson.D{}}}, "users"},
		{"insert", "insert", bson.D{{Key: "insert", Value: "logs"}}, "logs"},
		{"update", "update", bson.D{{Key: "update", Value: "orgs"}}, "orgs"},
		{"delete", "delete", bson.D{{Key: "delete", Value: "sessions"}}, "sessions"},
		{"aggregate", "aggregate", bson.D{{Key: "aggregate", Value: "logentries"}}, "logentries"},
		{"findAndModify", "findAndModify", bson.D{{Key: "findAndModify", Value: "apikeys"}}, "apikeys"},
		{"createIndexes", "createIndexes", bson.D{{Key: "createIndexes", Value: "users"}}, "users"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := collectionFromCommand(mustRaw(t, tc.doc), tc.command); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// getMore is the exception: its command value is a cursor ID, not a name, so the
// collection has to come from the separate "collection" field.
func TestCollectionFromCommand_GetMore(t *testing.T) {
	doc := mustRaw(t, bson.D{
		{Key: "getMore", Value: int64(7788)},
		{Key: "collection", Value: "logentries"},
	})
	if got := collectionFromCommand(doc, "getMore"); got != "logentries" {
		t.Errorf("got %q, want %q", got, "logentries")
	}
}

// Commands with no collection get the placeholder rather than an empty label.
func TestCollectionFromCommand_NoCollection(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		doc           bson.D
	}{
		{"ping", "ping", bson.D{{Key: "ping", Value: 1}}},
		{"hello", "hello", bson.D{{Key: "hello", Value: 1}}},
		{"endSessions", "endSessions", bson.D{{Key: "endSessions", Value: bson.A{}}}},
		{"buildInfo", "buildInfo", bson.D{{Key: "buildInfo", Value: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := collectionFromCommand(mustRaw(t, tc.doc), tc.command); got != UnknownCollection {
				t.Errorf("got %q, want %q", got, UnknownCollection)
			}
		})
	}
}

// A remembered collection is handed out once and then forgotten, so the map does
// not accumulate entries across commands.
func TestInFlightCommands_TakeIsOneShot(t *testing.T) {
	rememberCollection(11, "users")
	if got := takeCollection(11); got != "users" {
		t.Fatalf("got %q, want %q", got, "users")
	}
	if got := takeCollection(11); got != UnknownCollection {
		t.Errorf("second take returned %q; the entry should be gone", got)
	}
}

// An unrecognised request ID degrades to the placeholder instead of panicking.
func TestInFlightCommands_UnknownRequestID(t *testing.T) {
	if got := takeCollection(-99); got != UnknownCollection {
		t.Errorf("got %q, want %q", got, UnknownCollection)
	}
}

// Past the cap the map stops growing. The cost is a lost label, not memory.
func TestInFlightCommands_Bounded(t *testing.T) {
	inFlightCommands.Lock()
	inFlightCommands.m = make(map[int64]string)
	inFlightCommands.Unlock()
	t.Cleanup(func() {
		inFlightCommands.Lock()
		inFlightCommands.m = make(map[int64]string)
		inFlightCommands.Unlock()
	})

	for i := 0; i < maxInFlightCommands+25; i++ {
		rememberCollection(int64(i), "users")
	}

	inFlightCommands.Lock()
	size := len(inFlightCommands.m)
	inFlightCommands.Unlock()
	if size != maxInFlightCommands {
		t.Errorf("map holds %d entries, want the cap of %d", size, maxInFlightCommands)
	}
	if got := takeCollection(int64(maxInFlightCommands + 5)); got != UnknownCollection {
		t.Errorf("entry past the cap was stored: got %q", got)
	}
}

// A started+succeeded pair lands on the right command/database/collection series.
func TestCommandMonitor_LabelsSuccess(t *testing.T) {
	m := commandMonitor()
	ctx := context.Background()

	before := testutil.ToFloat64(CommandsTotal.WithLabelValues("find", "appdb", "users", "success"))

	m.Started(ctx, &event.CommandStartedEvent{
		CommandName: "find", DatabaseName: "appdb", RequestID: 501,
		Command: mustRaw(t, bson.D{{Key: "find", Value: "users"}}),
	})
	m.Succeeded(ctx, &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName: "find", DatabaseName: "appdb", RequestID: 501,
			Duration: 7 * time.Millisecond,
		},
	})

	if after := testutil.ToFloat64(CommandsTotal.WithLabelValues("find", "appdb", "users", "success")); after != before+1 {
		t.Errorf("counter went %v -> %v, want +1", before, after)
	}
}

// Failures are counted separately so the error rate is not diluted by successes.
func TestCommandMonitor_LabelsFailure(t *testing.T) {
	m := commandMonitor()
	ctx := context.Background()

	before := testutil.ToFloat64(CommandsTotal.WithLabelValues("update", "appdb", "logs", "failure"))

	m.Started(ctx, &event.CommandStartedEvent{
		CommandName: "update", DatabaseName: "appdb", RequestID: 502,
		Command: mustRaw(t, bson.D{{Key: "update", Value: "logs"}}),
	})
	m.Failed(ctx, &event.CommandFailedEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName: "update", DatabaseName: "appdb", RequestID: 502,
			Duration: 2 * time.Millisecond,
		},
		Failure: "connection reset",
	})

	if after := testutil.ToFloat64(CommandsTotal.WithLabelValues("update", "appdb", "logs", "failure")); after != before+1 {
		t.Errorf("counter went %v -> %v, want +1", before, after)
	}
}

// The same collection name in two databases must stay two series -- that is the
// whole point of carrying the database label.
func TestCommandMonitor_SameCollectionDifferentDatabases(t *testing.T) {
	m := commandMonitor()
	ctx := context.Background()

	for i, db := range []string{"appdb", "analytics"} {
		id := int64(600 + i)
		m.Started(ctx, &event.CommandStartedEvent{
			CommandName: "find", DatabaseName: db, RequestID: id,
			Command: mustRaw(t, bson.D{{Key: "find", Value: "events"}}),
		})
		m.Succeeded(ctx, &event.CommandSucceededEvent{
			CommandFinishedEvent: event.CommandFinishedEvent{
				CommandName: "find", DatabaseName: db, RequestID: id, Duration: time.Millisecond,
			},
		})
	}

	for _, db := range []string{"appdb", "analytics"} {
		if got := testutil.ToFloat64(CommandsTotal.WithLabelValues("find", db, "events", "success")); got < 1 {
			t.Errorf("database %q lost its own series (got %v)", db, got)
		}
	}
}

// A finished event with no matching start still records, just without the
// collection -- it must never drop the observation entirely.
func TestCommandMonitor_FinishedWithoutStart(t *testing.T) {
	m := commandMonitor()
	before := testutil.ToFloat64(CommandsTotal.WithLabelValues("ping", "admin", UnknownCollection, "success"))
	m.Succeeded(context.Background(), &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{
			CommandName: "ping", DatabaseName: "admin", RequestID: 999999, Duration: time.Millisecond,
		},
	})
	if after := testutil.ToFloat64(CommandsTotal.WithLabelValues("ping", "admin", UnknownCollection, "success")); after != before+1 {
		t.Errorf("counter went %v -> %v, want +1", before, after)
	}
}
