package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/gin-gonic/gin"
)

func doJSON(t *testing.T, eng *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)
	return w
}

func TestModsCRUD(t *testing.T) {
	eng, r := newTestRouter(t)

	// Two servers to exercise cross-server isolation.
	s1 := models.Server{Name: "s1", InstallPath: t.TempDir()}
	s2 := models.Server{Name: "s2", InstallPath: t.TempDir()}
	if err := r.db.Create(&s1).Error; err != nil {
		t.Fatal(err)
	}
	if err := r.db.Create(&s2).Error; err != nil {
		t.Fatal(err)
	}

	// Empty list initially.
	w := doJSON(t, eng, http.MethodGet, "/api/servers/1/mods", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d body=%s", w.Code, w.Body.String())
	}
	var listResp struct {
		Mods []models.Mod `json:"mods"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Mods) != 0 {
		t.Errorf("expected empty mod list, got %d", len(listResp.Mods))
	}

	// Add a mod (list entry only, no download).
	w = doJSON(t, eng, http.MethodPost, "/api/servers/1/mods", map[string]string{"workshopId": "12345", "name": "My Mod"})
	if w.Code != http.StatusCreated {
		t.Fatalf("add: got %d body=%s", w.Code, w.Body.String())
	}
	var created models.Mod
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.WorkshopID != "12345" || created.Name != "My Mod" || !created.Enabled {
		t.Errorf("unexpected created mod: %+v", created)
	}

	// Missing workshopId is a 400.
	w = doJSON(t, eng, http.MethodPost, "/api/servers/1/mods", map[string]string{"name": "x"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing workshopId, got %d", w.Code)
	}

	// The mod belongs to server 1, not server 2.
	w = doJSON(t, eng, http.MethodGet, "/api/servers/2/mods", nil)
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp.Mods) != 0 {
		t.Errorf("server 2 should have no mods, got %d", len(listResp.Mods))
	}

	// Toggle from server 2 must be rejected (cross-server).
	w = doJSON(t, eng, http.MethodPut, "/api/servers/2/mods/1/toggle", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-server toggle should 404, got %d", w.Code)
	}

	// Toggle from the owning server flips enabled to false.
	w = doJSON(t, eng, http.MethodPut, "/api/servers/1/mods/1/toggle", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("toggle: got %d body=%s", w.Code, w.Body.String())
	}
	var toggled models.Mod
	json.Unmarshal(w.Body.Bytes(), &toggled)
	if toggled.Enabled {
		t.Errorf("expected enabled=false after toggle, got %+v", toggled)
	}

	// Delete returns 204 and removes the row.
	w = doJSON(t, eng, http.MethodDelete, "/api/servers/1/mods/1", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d body=%s", w.Code, w.Body.String())
	}
	var count int64
	r.db.Model(&models.Mod{}).Where("server_id = ?", 1).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 mods after delete, got %d", count)
	}
}

func TestModsServerNotFound(t *testing.T) {
	eng, _ := newTestRouter(t)
	w := doJSON(t, eng, http.MethodGet, "/api/servers/999/mods", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing server, got %d", w.Code)
	}
}
