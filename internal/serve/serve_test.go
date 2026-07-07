package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// drive feeds the given request lines to a Server and returns the emitted
// events, parsed one per output line.
func drive(t *testing.T, lines ...string) []Event {
	t.Helper()
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	var out bytes.Buffer
	srv := New(Options{In: in, Out: &out, Version: "test"})
	if err := srv.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var events []Event
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal event %q: %v", line, err)
		}
		events = append(events, e)
	}
	return events
}

func TestServeEmitsReadyFirst(t *testing.T) {
	events := drive(t, `{"type":"ping","request_id":"p1"}`)
	if len(events) < 2 {
		t.Fatalf("expected at least ready+pong, got %d", len(events))
	}
	if events[0].Type != EvtReady {
		t.Fatalf("first event must be ready, got %q", events[0].Type)
	}
	if events[0].SchemaVersion != SchemaVersion {
		t.Fatalf("ready must carry schema version %d, got %d", SchemaVersion, events[0].SchemaVersion)
	}
}

func TestServePingPong(t *testing.T) {
	events := drive(t, `{"type":"ping","request_id":"p1"}`)
	var got *Event
	for i := range events {
		if events[i].Type == EvtPong {
			got = &events[i]
		}
	}
	if got == nil {
		t.Fatal("no pong emitted")
	}
	if got.RequestID != "p1" {
		t.Fatalf("pong request_id = %q, want p1", got.RequestID)
	}
}

func TestServeUnknownRequestErrors(t *testing.T) {
	events := drive(t, `{"type":"nonsense","request_id":"x"}`)
	found := false
	for _, e := range events {
		if e.Type == EvtError && e.RequestID == "x" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an error event for unknown request type")
	}
}

func TestServeInvalidJSONErrors(t *testing.T) {
	events := drive(t, `{not json}`)
	found := false
	for _, e := range events {
		if e.Type == EvtError {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an error event for invalid JSON")
	}
}

func TestServeApprovalResponseForUnknownIDErrors(t *testing.T) {
	events := drive(t, `{"type":"approval_response","approval_id":"nope","approved":true}`)
	found := false
	for _, e := range events {
		if e.Type == EvtError && strings.Contains(e.Message, "nope") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an error for approval response to unknown id")
	}
}
