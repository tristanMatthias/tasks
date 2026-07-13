package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// A live "something changed" push. It's deliberately a small signal, not the
// task itself: the client re-fetches through the SAME endpoints (and slim tree
// projection) it loaded with, so the store shape stays identical and there's no
// partial-merge to keep in sync. IDs name the affected task(s) so the client can
// refresh an open detail pane precisely; Mtime mirrors the /api/meta freshness.
type wsMessage struct {
	Type  string   `json:"type"`
	IDs   []string `json:"ids,omitempty"`
	Mtime float64  `json:"mtime"`
}

// wsHub fans a message out to every connected client on a topic. The topic is
// the tenant/board key (empty in single-tenant), so a mutation in one workspace
// never leaks to another's sockets.
type wsHub struct {
	mu      sync.Mutex
	topics  map[string]map[*wsClient]struct{}
}

type wsClient struct {
	send chan []byte
}

func newWSHub() *wsHub { return &wsHub{topics: map[string]map[*wsClient]struct{}{}} }

func (h *wsHub) add(topic string, c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.topics[topic]
	if set == nil {
		set = map[*wsClient]struct{}{}
		h.topics[topic] = set
	}
	set[c] = struct{}{}
}

func (h *wsHub) remove(topic string, c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.topics[topic]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.topics, topic)
		}
	}
}

// publish delivers payload to every client on topic, dropping it for any client
// whose buffer is full (a slow/stuck socket must never block a mutation).
func (h *wsHub) publish(topic string, payload []byte) {
	h.mu.Lock()
	clients := make([]*wsClient, 0, len(h.topics[topic]))
	for c := range h.topics[topic] {
		clients = append(clients, c)
	}
	h.mu.Unlock()
	for _, c := range clients {
		select {
		case c.send <- payload:
		default: // buffer full — drop; the next change (or a reconnect) resyncs
		}
	}
}

// Publish broadcasts a "changed" signal naming the affected task id(s) to every
// socket subscribed to topic. Wire it to a Core's onChange hook (single-tenant:
// topic ""; multi-tenant: the org/workspace key). Safe to call from any surface.
func (s *Server) Publish(topic string, ids []string) {
	if s.hub == nil {
		return
	}
	payload, err := json.Marshal(wsMessage{Type: "changed", IDs: ids, Mtime: s.mtime()})
	if err != nil {
		return
	}
	s.hub.publish(topic, payload)
}

// handleWS upgrades an (already auth-gated) request to a WebSocket and streams
// change signals for the caller's tenant until the socket closes.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Same-origin only by default (the auth gate already ran; this blocks a
	// cross-site page from opening an authed socket with the user's cookie).
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept already wrote the response
	}
	defer c.CloseNow()

	topic := s.topicForReq(r)
	client := &wsClient{send: make(chan []byte, 32)}
	s.hub.add(topic, client)
	defer s.hub.remove(topic, client)

	// We never expect client messages; CloseRead drains/handles control frames
	// and gives us a context cancelled the moment the peer goes away.
	ctx := c.CloseRead(r.Context())
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-client.send:
			if err := c.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
		}
	}
}

// topicForReq resolves the tenant/board key for a socket (empty single-tenant).
func (s *Server) topicForReq(r *http.Request) string {
	if s.topicFor != nil {
		return s.topicFor(r)
	}
	return ""
}
