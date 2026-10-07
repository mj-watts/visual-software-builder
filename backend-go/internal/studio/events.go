package studio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func eventCursor(r *http.Request) (int, error) {
	after := 0
	var err error
	if raw := r.URL.Query().Get("after"); raw != "" {
		after, err = strconv.Atoi(raw)
	}
	if err != nil || after < 0 {
		return 0, problem(422, "Invalid event cursor")
	}
	if v, err := strconv.Atoi(r.Header.Get("Last-Event-ID")); err == nil {
		after = max(after, v)
	}
	return after, nil
}
func (a *App) publishEvents(w http.ResponseWriter, after int) (int, error) {
	a.mu.Lock()
	events, err := a.store.events(after, "")
	a.mu.Unlock()
	if err != nil {
		return after, err
	}
	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return after, err
		}
		if _, err = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.ID, data); err != nil {
			return after, err
		}
		after = e.ID
	}
	return after, nil
}
func (a *App) eventStream(w http.ResponseWriter, r *http.Request) {
	after, err := eventCursor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, problem(500, "Streaming unavailable"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		after, err = a.publishEvents(w, after)
		if err != nil {
			logError(err)
			return
		}
		if _, err = fmt.Fprint(w, ": keepalive\n\n"); err != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
