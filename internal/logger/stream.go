package logger

import (
	"strconv"
	"sync"
	"sync/atomic"
)

// StreamManager fans out live log lines to any number of SSE subscribers,
// grouped by server ID. It is safe for concurrent use.
type StreamManager struct {
	mu      sync.RWMutex
	clients map[int64]map[string]chan string
	counter atomic.Uint64
}

// NewStreamManager creates an empty StreamManager.
func NewStreamManager() *StreamManager {
	return &StreamManager{
		clients: make(map[int64]map[string]chan string),
	}
}

// Subscribe registers a new subscriber for a server's log stream and returns a
// unique client id plus the channel that will receive log lines. The caller
// must Unsubscribe when done.
func (sm *StreamManager) Subscribe(serverID int64) (string, chan string) {
	// Buffered so a slow consumer doesn't block the writer; overflow is dropped.
	ch := make(chan string, 256)
	id := strconv.FormatUint(sm.counter.Add(1), 10)

	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.clients[serverID] == nil {
		sm.clients[serverID] = make(map[string]chan string)
	}
	sm.clients[serverID][id] = ch
	return id, ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (sm *StreamManager) Unsubscribe(serverID int64, id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if subs, ok := sm.clients[serverID]; ok {
		if ch, ok := subs[id]; ok {
			close(ch)
			delete(subs, id)
		}
		if len(subs) == 0 {
			delete(sm.clients, serverID)
		}
	}
}

// Broadcast sends a log line to all subscribers of a server. Subscribers whose
// buffer is full are skipped (the line is dropped for them) rather than blocking.
func (sm *StreamManager) Broadcast(serverID int64, line string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, ch := range sm.clients[serverID] {
		select {
		case ch <- line:
		default:
			// Consumer too slow; drop this line for them.
		}
	}
}
