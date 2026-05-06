package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type EventHub struct {
	mu      sync.Mutex
	clients map[chan serverEvent]struct{}
}

type serverEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewEventHub() *EventHub {
	return &EventHub{clients: make(map[chan serverEvent]struct{})}
}

func (h *EventHub) Publish(eventType string, data interface{}) {
	if h == nil {
		return
	}
	event := serverEvent{Type: eventType, Data: data}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *EventHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "Streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan serverEvent, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}()

	writeEvent := func(event serverEvent) bool {
		payload, err := json.Marshal(event.Data)
		if err != nil {
			payload = []byte("{}")
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !writeEvent(serverEvent{Type: "connected", Data: map[string]interface{}{"time": time.Now().Format(time.RFC3339)}}) {
		return
	}
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			if !writeEvent(event) {
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
