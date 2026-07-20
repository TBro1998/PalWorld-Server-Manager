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
	req.Header.Set("Authorization", "Bearer "+testToken(t))
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)
	return w
}

// TestServerModsLinkToggleUnlink exercises the current mod model: a global mod
// library (POST /api/mods) plus per-server references (POST /api/servers/:id/mods
// with a modId). Mods are no longer owned by a server; a ServerMod row links a
// global mod to a server and carries the enabled flag.
func TestServerModsLinkToggleUnlink(t *testing.T) {
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

	// Empty server-mod list initially.
	w := doJSON(t, eng, http.MethodGet, "/api/servers/1/mods", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d body=%s", w.Code, w.Body.String())
	}
	var listResp struct {
		Mods []ServerModDetail `json:"mods"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Mods) != 0 {
		t.Errorf("expected empty mod list, got %d", len(listResp.Mods))
	}

	// Register a mod in the global library.
	w = doJSON(t, eng, http.MethodPost, "/api/mods", map[string]string{"workshopId": "12345", "name": "My Mod"})
	if w.Code != http.StatusCreated {
		t.Fatalf("add global: got %d body=%s", w.Code, w.Body.String())
	}
	var gm models.Mod
	if err := json.Unmarshal(w.Body.Bytes(), &gm); err != nil {
		t.Fatal(err)
	}
	if gm.WorkshopID != "12345" || gm.Name != "My Mod" {
		t.Errorf("unexpected global mod: %+v", gm)
	}

	// Link the global mod to server 1.
	w = doJSON(t, eng, http.MethodPost, "/api/servers/1/mods", map[string]any{"modId": gm.ID})
	if w.Code != http.StatusCreated {
		t.Fatalf("link: got %d body=%s", w.Code, w.Body.String())
	}
	var sm models.ServerMod
	if err := json.Unmarshal(w.Body.Bytes(), &sm); err != nil {
		t.Fatal(err)
	}
	if sm.ServerID != s1.ID || sm.ModID != gm.ID || !sm.Enabled {
		t.Errorf("unexpected linked server mod: %+v", sm)
	}

	// Linking without modId/workshopId is a 400.
	w = doJSON(t, eng, http.MethodPost, "/api/servers/1/mods", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing modId, got %d", w.Code)
	}

	// The link belongs to server 1, not server 2.
	w = doJSON(t, eng, http.MethodGet, "/api/servers/2/mods", nil)
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp.Mods) != 0 {
		t.Errorf("server 2 should have no mods, got %d", len(listResp.Mods))
	}

	// Server 1 lists exactly the one linked mod, with a (empty) dependencies array.
	w = doJSON(t, eng, http.MethodGet, "/api/servers/1/mods", nil)
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp.Mods) != 1 {
		t.Fatalf("server 1 should have 1 mod, got %d", len(listResp.Mods))
	}
	if listResp.Mods[0].Dependencies == nil {
		t.Errorf("dependencies should be a non-nil array, got nil")
	}

	// Toggle from server 2 must be rejected (cross-server).
	w = doJSON(t, eng, http.MethodPut, "/api/servers/2/mods/"+itoa(sm.ID)+"/toggle", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-server toggle should 404, got %d", w.Code)
	}

	// Toggle from the owning server flips enabled to false.
	w = doJSON(t, eng, http.MethodPut, "/api/servers/1/mods/"+itoa(sm.ID)+"/toggle", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("toggle: got %d body=%s", w.Code, w.Body.String())
	}
	var toggled models.ServerMod
	json.Unmarshal(w.Body.Bytes(), &toggled)
	if toggled.Enabled {
		t.Errorf("expected enabled=false after toggle, got %+v", toggled)
	}

	// Unlink returns 204 and removes the server_mods row (global mod remains).
	w = doJSON(t, eng, http.MethodDelete, "/api/servers/1/mods/"+itoa(sm.ID), nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("unlink: got %d body=%s", w.Code, w.Body.String())
	}
	var smCount, gmCount int64
	r.db.Model(&models.ServerMod{}).Where("server_id = ?", s1.ID).Count(&smCount)
	if smCount != 0 {
		t.Errorf("expected 0 server mods after unlink, got %d", smCount)
	}
	r.db.Model(&models.Mod{}).Count(&gmCount)
	if gmCount != 1 {
		t.Errorf("global mod should survive unlink, got %d", gmCount)
	}
}

// TestGlobalModDependencies verifies the runtime dependency-satisfaction
// computation: a dependency name is satisfied when a downloaded global mod has
// that PackageName.
func TestGlobalModDependencies(t *testing.T) {
	eng, r := newTestRouter(t)

	// Dependency provider: downloaded, PackageName "DepPkg".
	dep := models.Mod{WorkshopID: "111", Name: "Dep", Downloaded: true, PackageName: "DepPkg"}
	if err := r.db.Create(&dep).Error; err != nil {
		t.Fatal(err)
	}
	// Consumer depends on "DepPkg" (satisfied) and "MissingPkg" (not).
	consumer := models.Mod{WorkshopID: "222", Name: "Consumer", Dependencies: []string{"DepPkg", "MissingPkg"}}
	if err := r.db.Create(&consumer).Error; err != nil {
		t.Fatal(err)
	}

	w := doJSON(t, eng, http.MethodGet, "/api/mods", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list global: got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Mods []ModWithStatus `json:"mods"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	var got *ModWithStatus
	for i := range resp.Mods {
		if resp.Mods[i].WorkshopID == "222" {
			got = &resp.Mods[i]
			break
		}
	}
	if got == nil {
		t.Fatal("consumer mod not found in list")
	}
	if len(got.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %#v", got.Dependencies)
	}
	byName := map[string]bool{}
	for _, d := range got.Dependencies {
		byName[d.Name] = d.Satisfied
	}
	if !byName["DepPkg"] {
		t.Errorf("DepPkg should be satisfied (a downloaded mod has that PackageName)")
	}
	if byName["MissingPkg"] {
		t.Errorf("MissingPkg should not be satisfied")
	}
}

func TestModsServerNotFound(t *testing.T) {
	eng, _ := newTestRouter(t)
	w := doJSON(t, eng, http.MethodGet, "/api/servers/999/mods", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing server, got %d", w.Code)
	}
}
