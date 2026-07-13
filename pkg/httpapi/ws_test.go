package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// dial opens a WS to srv's /api/ws with the given token, keyed to topic via the
// Host header (the test TopicFor derives the topic from Host).
func wsDial(t *testing.T, base, token, host string) *websocket.Conn {
	t.Helper()
	url := strings.Replace(base, "http://", "ws://", 1) + "/api/ws"
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	if token != "" {
		opts.HTTPHeader.Set("Authorization", "Bearer "+token)
	}
	if host != "" {
		opts.Host = host // sets the request Host, which our test TopicFor reads
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, opts)
	if err != nil {
		t.Fatalf("dial (%s): %v", host, err)
	}
	return c
}

func readMsg(t *testing.T, c *websocket.Conn) wsMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var m wsMessage
	if err := wsjson.Read(ctx, c, &m); err != nil {
		t.Fatalf("read: %v", err)
	}
	return m
}

func TestWS_AuthGated(t *testing.T) {
	srv := New(Config{Token: "secret", Static: fstest.MapFS{}})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// No token -> the auth gate rejects the upgrade (no 101).
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	url := strings.Replace(ts.URL, "http://", "ws://", 1) + "/api/ws"
	if _, _, err := websocket.Dial(ctx, url, nil); err == nil {
		t.Fatal("expected unauthenticated WS dial to fail")
	}
}

func TestWS_DeliversAndIsolatesByTopic(t *testing.T) {
	// Topic = request Host, so two "tenants" are simulated by two Hosts.
	srv := New(Config{
		Token:    "secret",
		Static:   fstest.MapFS{},
		TopicFor: func(r *http.Request) string { return r.Host },
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	a := wsDial(t, ts.URL, "secret", "team-a")
	defer a.CloseNow()
	b := wsDial(t, ts.URL, "secret", "team-b")
	defer b.CloseNow()

	// Give both registrations time to land in the hub.
	waitFor(t, func() bool { return srv.hubTopicCount("team-a") == 1 && srv.hubTopicCount("team-b") == 1 })

	srv.Publish("team-a", []string{"x1"})

	got := readMsg(t, a)
	if got.Type != "changed" || len(got.IDs) != 1 || got.IDs[0] != "x1" {
		t.Fatalf("team-a got unexpected message: %+v", got)
	}

	// team-b must NOT receive team-a's change.
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	var leaked wsMessage
	if err := wsjson.Read(ctx, b, &leaked); err == nil {
		t.Fatalf("team-b leaked another tenant's message: %+v", leaked)
	}
}

func TestWS_UnsubscribeOnClose(t *testing.T) {
	srv := New(Config{Token: "secret", Static: fstest.MapFS{}})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	c := wsDial(t, ts.URL, "secret", "")
	waitFor(t, func() bool { return srv.hubTopicCount("") == 1 })
	c.CloseNow()
	waitFor(t, func() bool { return srv.hubTopicCount("") == 0 })
}

// hubTopicCount reports how many clients are registered on a topic (test helper).
func (s *Server) hubTopicCount(topic string) int {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	return len(s.hub.topics[topic])
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
