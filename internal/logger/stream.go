package logger

import (
	"strconv"
	"sync"
	"sync/atomic"
)

// streamKey identifies a single log stream: a server plus a log kind
// (KindServer / KindSteamCMD). Keying by both keeps the game server's runtime
// logs and SteamCMD's install/update logs on independent channels.
type streamKey struct {
	serverID int64
	kind     string
}

// StreamManager fans out live log lines to any number of SSE subscribers,
// grouped by (server ID, log kind). It is safe for concurrent use.
type StreamManager struct {
	mu      sync.RWMutex
	clients map[streamKey]map[string]chan string
	counter atomic.Uint64
}

// NewStreamManager creates an empty StreamManager.
func NewStreamManager() *StreamManager {
	return &StreamManager{
		clients: make(map[streamKey]map[string]chan string),
	}
}

// Subscribe registers a new subscriber for a server's log stream of the given
// kind and returns a unique client id plus the channel that will receive log
// lines. The caller must Unsubscribe when done.
func (sm *StreamManager) Subscribe(serverID int64, kind string) (string, chan string) {
	// Buffered so a slow consumer doesn't block the writer; overflow is dropped.
	ch := make(chan string, 256)
	id := strconv.FormatUint(sm.counter.Add(1), 10)
	key := streamKey{serverID: serverID, kind: kind}

	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.clients[key] == nil {
		sm.clients[key] = make(map[string]chan string)
	}
	sm.clients[key][id] = ch
	return id, ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (sm *StreamManager) Unsubscribe(serverID int64, kind, id string) {
	key := streamKey{serverID: serverID, kind: kind}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if subs, ok := sm.clients[key]; ok {
		if ch, ok := subs[id]; ok {
			close(ch)
			delete(subs, id)
		}
		if len(subs) == 0 {
			delete(sm.clients, key)
		}
	}
}

// Broadcast sends a log line to all subscribers of a server's stream of the
// given kind. Subscribers whose buffer is full are skipped (the line is dropped
// for them) rather than blocking.
func (sm *StreamManager) Broadcast(serverID int64, kind, line string) {
	key := streamKey{serverID: serverID, kind: kind}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, ch := range sm.clients[key] {
		select {
		case ch <- line:
		default:
			// Consumer too slow; drop this line for them.
		}
	}
}
