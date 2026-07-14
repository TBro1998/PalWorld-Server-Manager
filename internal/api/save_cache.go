package api

import (
	"os"
	"sync"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/palsave"
)

// saveCache memoizes parsed Level.sav results per server. Parsing a Level.sav is
// expensive (it can be hundreds of KB of GVAS), so we keep the parsed *Level and
// re-parse only when the file's modification time or size changes (R3).
//
// Player .sav files are small and are NOT cached here; handlers parse them on
// demand via palsave.LoadPlayer.
type saveCache struct {
	mu      sync.Mutex
	entries map[int64]*saveEntry // keyed by server ID
}

type saveEntry struct {
	levelPath string
	modTime   time.Time
	size      int64
	level     *palsave.Level
}

func newSaveCache() *saveCache {
	return &saveCache{entries: map[int64]*saveEntry{}}
}

// Level returns the parsed Level for a server, using the cached value when the
// file at levelPath is unchanged (same mtime and size) since it was parsed.
// Otherwise it re-parses and updates the cache. A stat failure or parse error is
// returned to the caller; the cache is left untouched on error.
func (c *saveCache) Level(serverID int64, levelPath string) (*palsave.Level, error) {
	fi, err := os.Stat(levelPath)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[serverID]; ok &&
		e.levelPath == levelPath &&
		e.modTime.Equal(fi.ModTime()) &&
		e.size == fi.Size() {
		return e.level, nil
	}

	level, err := palsave.LoadLevel(levelPath)
	if err != nil {
		return nil, err
	}
	c.entries[serverID] = &saveEntry{
		levelPath: levelPath,
		modTime:   fi.ModTime(),
		size:      fi.Size(),
		level:     level,
	}
	return level, nil
}
