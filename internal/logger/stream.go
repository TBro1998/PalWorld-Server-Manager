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

// Msg is one SSE message delivered to a subscriber: an event name plus its data
// payload. Ordinary log lines use Event "log" (see Broadcast); control signals
// such as mod-update completion use their own event name (e.g. "done") so
// subscribers can react to them without parsing log text.
type Msg struct {
	Event string
	Data  string
}

// StreamManager fans out live log lines to any number of SSE subscribers,
// grouped by (server ID, log kind). It is safe for concurrent use.
type StreamManager struct {
	mu      sync.RWMutex
	clients map[streamKey]map[string]chan Msg
	counter atomic.Uint64
}

// NewStreamManager creates an empty StreamManager.
func NewStreamManager() *StreamManager {
	return &StreamManager{
		clients: make(map[streamKey]map[string]chan Msg),
	}
}

// Subscribe registers a new subscriber for a server's log stream of the given
// kind and returns a unique client id plus the channel that will receive
// messages. The caller must Unsubscribe when done.
func (sm *StreamManager) Subscribe(serverID int64, kind string) (string, chan Msg) {
	// Buffered so a slow consumer doesn't block the writer; overflow is dropped.
	ch := make(chan Msg, 256)
	id := strconv.FormatUint(sm.counter.Add(1), 10)
	key := streamKey{serverID: serverID, kind: kind}

	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.clients[key] == nil {
		sm.clients[key] = make(map[string]chan Msg)
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
// for them) rather than blocking. The line is delivered as a "log" event.
func (sm *StreamManager) Broadcast(serverID int64, kind, line string) {
	sm.send(serverID, kind, Msg{Event: "log", Data: line})
}

// BroadcastEvent sends a named control event (e.g. "done") to all subscribers of
// a server's stream of the given kind. Same non-blocking, drop-on-full semantics
// as Broadcast. Use this for out-of-band signals that subscribers should act on
// without treating them as log text.
func (sm *StreamManager) BroadcastEvent(serverID int64, kind, event, data string) {
	sm.send(serverID, kind, Msg{Event: event, Data: data})
}

func (sm *StreamManager) send(serverID int64, kind string, msg Msg) {
	key := streamKey{serverID: serverID, kind: kind}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, ch := range sm.clients[key] {
		select {
		case ch <- msg:
		default:
			// Consumer too slow; drop this message for them.
		}
	}
}
