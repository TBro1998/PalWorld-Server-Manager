package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/auth"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/config"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/database"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palsave"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/update"
	"github.com/gin-gonic/gin"
)

// copyFile copies src to dst, failing the test on error.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("fixture %s unavailable: %v", src, err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// layoutSave builds a <install>/Pal/Saved/SaveGames/0/<world>/ tree containing
// the testdata Level.sav and returns the install root. If playerUID != "" a copy
// of Player.sav is placed under Players/ named for that UID.
func layoutSave(t *testing.T, playerUID string) string {
	t.Helper()
	root := t.TempDir()
	world := filepath.Join(root, "Pal", "Saved", "SaveGames", "0", "TESTWORLD")
	if err := os.MkdirAll(filepath.Join(world, "Players"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join("..", "palsave", "testdata", "Level.sav"), filepath.Join(world, "Level.sav"))
	if playerUID != "" {
		copyFile(t, filepath.Join("..", "palsave", "testdata", "Player.sav"),
			filepath.Join(world, "Players", palsave.PlayerSaveFile(playerUID)))
	}
	return root
}

// newTestRouter spins up a Router backed by a fresh temp SQLite DB and returns a
// mounted gin engine plus the DB for inserting fixtures.
func newTestRouter(t *testing.T) (*gin.Engine, *Router) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := database.Initialize(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db init: %v", err)
	}
	// Close the SQLite handle before the temp dir is removed; on Windows an open
	// file cannot be unlinked, which would fail t.TempDir cleanup.
	t.Cleanup(func() {
		if sqlDB, derr := db.DB(); derr == nil {
			_ = sqlDB.Close()
		}
	})
	cfg := &config.Config{LogDir: t.TempDir(), JWTSecret: testJWTSecret}
	r := NewRouter(db, cfg, update.BuildInfo{})
	eng := gin.New()
	r.RegisterRoutes(eng.Group("/api"))
	return eng, r
}

// testJWTSecret signs the token used by test requests. RegisterRoutes mounts the
// protected group behind JWT auth, so every test request must carry a valid
// bearer token (see testToken / doJSON / doGET).
const testJWTSecret = "test-secret"

// testToken returns a bearer token valid for the test router's JWT secret.
func testToken(t *testing.T) string {
	t.Helper()
	tok, err := auth.GenerateToken(testJWTSecret)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

func doGET(t *testing.T, eng *gin.Engine, path string) (int, map[string]json.RawMessage) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+testToken(t))
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)
	var body map[string]json.RawMessage
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func TestSaveEndpoints(t *testing.T) {
	// Discover a player UID from the level so the inventory fixture can be named.
	level, err := palsave.LoadLevel(filepath.Join("..", "palsave", "testdata", "Level.sav"))
	if err != nil {
		t.Skipf("cannot load testdata Level.sav: %v", err)
	}
	if len(level.Players) == 0 {
		t.Skip("testdata Level.sav has no players")
	}
	uid := level.Players[0].UID

	eng, r := newTestRouter(t)
	install := layoutSave(t, uid)
	srv := models.Server{Name: "t", InstallPath: install}
	if err := r.db.Create(&srv).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}
	base := "/api/servers/" + itoa(srv.ID)

	t.Run("players", func(t *testing.T) {
		code, body := doGET(t, eng, base+"/save/players")
		if code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", code, body)
		}
		var players []savePlayerDTO
		if err := json.Unmarshal(body["players"], &players); err != nil {
			t.Fatalf("decode players: %v", err)
		}
		if len(players) == 0 {
			t.Fatal("expected at least one player")
		}
	})

	t.Run("guilds", func(t *testing.T) {
		code, body := doGET(t, eng, base+"/save/guilds")
		if code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", code, body)
		}
		if _, ok := body["guilds"]; !ok {
			t.Fatal("missing guilds key")
		}
	})

	t.Run("pals", func(t *testing.T) {
		code, body := doGET(t, eng, base+"/save/players/"+uid+"/pals")
		if code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", code, body)
		}
		if _, ok := body["pals"]; !ok {
			t.Fatal("missing pals key")
		}
	})

	t.Run("inventory", func(t *testing.T) {
		code, body := doGET(t, eng, base+"/save/players/"+uid+"/inventory")
		if code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", code, body)
		}
		if _, ok := body["inventory"]; !ok {
			t.Fatal("missing inventory key")
		}
	})
}

func TestSaveEndpointsErrors(t *testing.T) {
	eng, r := newTestRouter(t)

	// Server with no save on disk -> 404.
	srv := models.Server{Name: "empty", InstallPath: t.TempDir()}
	if err := r.db.Create(&srv).Error; err != nil {
		t.Fatal(err)
	}
	if code, _ := doGET(t, eng, "/api/servers/"+itoa(srv.ID)+"/save/players"); code != http.StatusNotFound {
		t.Errorf("no-save code = %d, want 404", code)
	}

	// Unknown server id -> 404.
	if code, _ := doGET(t, eng, "/api/servers/999999/save/players"); code != http.StatusNotFound {
		t.Errorf("unknown-server code = %d, want 404", code)
	}

	// Non-numeric id -> 400.
	if code, _ := doGET(t, eng, "/api/servers/abc/save/players"); code != http.StatusBadRequest {
		t.Errorf("bad-id code = %d, want 400", code)
	}
}

func TestSaveCacheReuseAndInvalidate(t *testing.T) {
	dir := t.TempDir()
	levelPath := filepath.Join(dir, "Level.sav")
	copyFile(t, filepath.Join("..", "palsave", "testdata", "Level.sav"), levelPath)

	c := newSaveCache()
	first, err := c.Level(1, levelPath)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	second, err := c.Level(1, levelPath)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("unchanged file should return the cached *Level (same pointer)")
	}

	// Bump mtime -> cache must re-parse (different pointer).
	future := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(levelPath, future, future); err != nil {
		t.Fatal(err)
	}
	third, err := c.Level(1, levelPath)
	if err != nil {
		t.Fatal(err)
	}
	if third == second {
		t.Error("changed mtime should invalidate the cache (expected re-parse)")
	}
}

// itoa avoids importing strconv just for the tiny id-to-path conversions above.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
