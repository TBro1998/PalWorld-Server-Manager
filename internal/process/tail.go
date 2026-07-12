package process

import (
	"bytes"
	"io"
	"os"
	"time"
)

// tailPollInterval is how often the tailer checks for new content, or for the
// log file to appear when it does not exist yet.
const tailPollInterval = 1 * time.Second

// tailFile follows the file at path, writing each complete line (newline
// terminated, CR stripped) to w until stop is closed, then closes done. It
// tolerates the file not existing yet — the Palworld server creates its log a
// moment after launch — and handles a new run recreating/truncating the file by
// reopening from the start.
//
// It begins following from the current end of a pre-existing file so a previous
// run's log is not replayed; when the file is truncated (a fresh run), the shrink
// is detected and the new content is read from the beginning.
func tailFile(stop <-chan struct{}, done chan<- struct{}, path string, w io.Writer) {
	defer close(done)

	var f *os.File
	var offset int64
	var pending []byte
	newline := []byte{'\n'}
	buf := make([]byte, 32*1024)

	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	for {
		select {
		case <-stop:
			return
		default:
		}

		if f == nil {
			file, err := os.Open(path)
			if err != nil {
				if !waitOrStop(stop) {
					return
				}
				continue
			}
			f = file
			// Follow from the end so a prior run's log isn't replayed.
			if end, err := f.Seek(0, io.SeekEnd); err == nil {
				offset = end
			} else {
				offset = 0
			}
			pending = pending[:0]
		}

		// A new run recreates/truncates the file; read it from the start.
		if fi, err := f.Stat(); err == nil && fi.Size() < offset {
			f.Close()
			f = nil
			if file, err := os.Open(path); err == nil {
				f = file
				offset = 0
				pending = pending[:0]
			}
			continue
		}

		n, err := f.Read(buf)
		if n > 0 {
			offset += int64(n)
			pending = append(pending, buf[:n]...)
			for {
				i := bytes.IndexByte(pending, '\n')
				if i < 0 {
					break
				}
				line := pending[:i]
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
				// Two writes keep the broadcast splitter happy: it flushes a line
				// only on the terminating newline.
				if _, werr := w.Write(line); werr != nil {
					return
				}
				if _, werr := w.Write(newline); werr != nil {
					return
				}
				pending = pending[i+1:]
			}
		}
		if err == io.EOF || n == 0 {
			if !waitOrStop(stop) {
				return
			}
			continue
		}
		if err != nil {
			return
		}
	}
}

// waitOrStop sleeps for tailPollInterval, returning false if stop was closed
// during the wait (signaling the caller to return).
func waitOrStop(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return false
	case <-time.After(tailPollInterval):
		return true
	}
}
