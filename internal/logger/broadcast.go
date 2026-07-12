package logger

import (
	"bytes"
	"sync"
)

// broadcastWriter implements io.Writer, splitting incoming output into lines and
// forwarding each complete line to a StreamManager. Partial lines are buffered
// until the terminating newline arrives.
type broadcastWriter struct {
	streams  *StreamManager
	serverID int64
	kind     string

	mu  sync.Mutex
	buf bytes.Buffer
}

// NewBroadcastWriter returns an io.Writer that streams complete lines of output
// to all SSE subscribers of the given server's stream of the given kind.
func NewBroadcastWriter(streams *StreamManager, serverID int64, kind string) *broadcastWriter {
	return &broadcastWriter{streams: streams, serverID: serverID, kind: kind}
}

func (w *broadcastWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// No full line yet; put the remainder back and wait for more.
			w.buf.Reset()
			w.buf.WriteString(line)
			break
		}
		// Trim the trailing newline (and optional CR) before broadcasting.
		trimmed := line[:len(line)-1]
		if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\r' {
			trimmed = trimmed[:len(trimmed)-1]
		}
		w.streams.Broadcast(w.serverID, w.kind, trimmed)
	}
	return len(p), nil
}
