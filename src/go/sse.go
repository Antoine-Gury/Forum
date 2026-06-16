package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
)

var sseClients = make(map[chan Discussion]struct{})
var sseClientsMu sync.Mutex

func broadcastNewDiscussion(d Discussion) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()
	for ch := range sseClients {
		select {
		case ch <- d:
		default:
		}
	}
}

func addSSEClient(ch chan Discussion) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()
	sseClients[ch] = struct{}{}
}

func removeSSEClient(ch chan Discussion) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()
	delete(sseClients, ch)
}

func Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan Discussion, 1)
	addSSEClient(ch)
	defer removeSSEClient(ch)

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case d := <-ch:
			data, err := json.Marshal(d)
			if err != nil {
				continue
			}
			_, _ = w.Write([]byte("event: discussion\n"))
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
			flusher.Flush()
		}
	}
}
