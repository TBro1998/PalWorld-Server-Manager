package palsave

import (
	"testing"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/palsave/gvas"
)

// TestGVASTrailer is gate G2: a full parse of each save must reach a clean
// 4-zero-byte trailer, proving the GVAS reader consumes the stream exactly.
func TestGVASTrailer(t *testing.T) {
	for _, name := range []string{"LevelMeta.sav", "Level.sav"} {
		data := readTestdata(t, name)
		gvasBytes, _, err := DecompressToGVAS(data)
		if err != nil {
			t.Fatalf("%s: decompress: %v", name, err)
		}
		f, err := gvas.ReadFull(gvasBytes, palworldTypeHints, palworldCustomProperties)
		if err != nil {
			t.Fatalf("%s: full parse: %v", name, err)
		}
		if len(f.Trailer) != 4 || f.Trailer[0]|f.Trailer[1]|f.Trailer[2]|f.Trailer[3] != 0 {
			t.Fatalf("%s: trailer = % x, want 00 00 00 00", name, f.Trailer)
		}
		t.Logf("%s: full parse OK, trailer 00 00 00 00", name)
	}
}

// TestLevelMaps is gate G3: the 5 target maps decode with no error and the core
// maps are non-empty.
func TestLevelMaps(t *testing.T) {
	data := readTestdata(t, "Level.sav")
	_, world, err := gvas.ReadWorldSaveData(gvasBytesOf(t, data), palworldTypeHints, palworldCustomProperties, "worldSaveData", worldSaveDataWanted)
	if err != nil {
		t.Fatalf("world parse: %v", err)
	}
	count := func(name string) int {
		p, ok := world[name]
		if !ok {
			return -1
		}
		entries, ok := p.Value.([]gvas.MapEntry)
		if !ok {
			return -1
		}
		return len(entries)
	}
	chars := count("CharacterSaveParameterMap")
	groups := count("GroupSaveDataMap")
	items := count("ItemContainerSaveData")
	t.Logf("CharacterSaveParameterMap=%d GroupSaveDataMap=%d ItemContainerSaveData=%d CharacterContainerSaveData=%d",
		chars, groups, items, count("CharacterContainerSaveData"))
	if chars <= 0 {
		t.Errorf("CharacterSaveParameterMap count = %d, want > 0", chars)
	}
	if groups <= 0 {
		t.Errorf("GroupSaveDataMap count = %d, want > 0", groups)
	}
	if items <= 0 {
		t.Errorf("ItemContainerSaveData count = %d, want > 0", items)
	}
}

// TestSemantic is gate G4: end-to-end LoadLevel yields readable players and
// guilds, and pals when present. It prints a summary for eyeballing.
func TestSemantic(t *testing.T) {
	data := readTestdata(t, "Level.sav")
	lvl, err := LoadLevelBytes(data)
	if err != nil {
		t.Fatalf("LoadLevelBytes: %v", err)
	}

	t.Logf("players=%d pals=%d guilds=%d", len(lvl.Players), len(lvl.Pals), len(lvl.Guilds))

	// Players
	namedPlayers := 0
	for _, p := range lvl.Players {
		t.Logf("player: uid=%s nick=%q level=%d exp=%d group=%s", p.UID, p.NickName, p.Level, p.Exp, p.GroupID)
		if p.NickName != "" {
			namedPlayers++
		}
	}
	if namedPlayers == 0 {
		t.Errorf("expected >= 1 player with a non-empty NickName")
	}

	// Guilds
	namedGuilds := 0
	for _, g := range lvl.Guilds {
		t.Logf("guild: name=%q admin=%s members=%d baseCamp=%d handles=%d", g.GuildName, g.AdminUID, len(g.Members), g.BaseCampLevel, len(g.HandleIDs))
		for _, m := range g.Members {
			t.Logf("    member: uid=%s name=%q role=%d", m.UID, m.Name, m.Role)
		}
		if g.GuildName != "" && len(g.Members) >= 1 {
			namedGuilds++
		}
	}
	if namedGuilds == 0 {
		t.Errorf("expected >= 1 guild with a non-empty name and >= 1 member")
	}

	// Pals: assert only if the sample contains any (this trimmed sample may not).
	if len(lvl.Pals) == 0 {
		t.Logf("NOTE: this sample Level.sav contains no captured pals; Pal extraction is exercised structurally but not asserted here")
	} else {
		withID := 0
		for i, p := range lvl.Pals {
			if i < 8 {
				t.Logf("pal: species=%s nick=%q level=%d rank=%d owner=%s", p.CharacterID, p.NickName, p.Level, p.Rank, p.OwnerUID)
			}
			if p.CharacterID != "" {
				withID++
			}
		}
		if withID == 0 {
			t.Errorf("expected >= 1 pal with a non-empty CharacterID")
		}
	}
}

// TestPlayerInventory exercises LoadPlayer + AttachInventories (R6).
func TestPlayerInventory(t *testing.T) {
	lvl, err := LoadLevelBytes(readTestdata(t, "Level.sav"))
	if err != nil {
		t.Fatalf("LoadLevelBytes: %v", err)
	}
	pl, err := LoadPlayerBytes(readTestdata(t, "Player.sav"))
	if err != nil {
		t.Fatalf("LoadPlayerBytes: %v", err)
	}
	t.Logf("player uid=%s instance=%s", pl.UID, pl.InstanceID)
	t.Logf("containers: common=%s essential=%s weapon=%s armor=%s food=%s drop=%s",
		pl.Inventory.CommonContainerID, pl.Inventory.EssentialContainerID,
		pl.Inventory.WeaponLoadOutContainerID, pl.Inventory.PlayerEquipArmorContainerID,
		pl.Inventory.FoodEquipContainerID, pl.Inventory.DropSlotContainerID)

	if pl.Inventory.CommonContainerID == "" {
		t.Fatalf("player CommonContainerId is empty")
	}

	// Cross-reference integrity (R6): every player container id must resolve to
	// a real container in the level's ItemContainerSaveData.
	playerContainers := []string{
		pl.Inventory.CommonContainerID, pl.Inventory.EssentialContainerID,
		pl.Inventory.WeaponLoadOutContainerID, pl.Inventory.PlayerEquipArmorContainerID,
		pl.Inventory.FoodEquipContainerID, pl.Inventory.DropSlotContainerID,
	}
	for _, id := range playerContainers {
		if id == "" {
			continue
		}
		if _, ok := lvl.itemContainers[id]; !ok {
			t.Errorf("player container %s not found in level ItemContainerSaveData", id)
		}
	}

	// Real assembly. In this trimmed sample the player's personal containers are
	// stored empty (the item-bearing containers are base/storage boxes), so this
	// may legitimately produce 0 items; log it rather than failing.
	lvl.AttachInventories([]*Player{pl})
	total := 0
	for role, items := range pl.Inventory.Items {
		for i, it := range items {
			if i < 5 {
				t.Logf("  %s slot %d x%d %s", role, it.SlotIndex, it.Count, it.StaticID)
			}
			total++
		}
	}
	t.Logf("real player inventory resolved %d items", total)

	// Verify the assembly mechanism against a container that is known to hold
	// items, proving GUID -> slots -> items resolution works end-to-end.
	var nonEmpty string
	for id, slots := range lvl.itemContainers {
		for _, s := range slots {
			if s.StaticID != "" {
				nonEmpty = id
				break
			}
		}
		if nonEmpty != "" {
			break
		}
	}
	if nonEmpty == "" {
		t.Fatalf("no non-empty item container found in level to verify assembly")
	}
	synthetic := &Player{Inventory: &Inventory{CommonContainerID: nonEmpty}}
	lvl.AttachInventories([]*Player{synthetic})
	if len(synthetic.Inventory.Items["CommonContainerId"]) == 0 {
		t.Fatalf("assembly failed: container %s resolved to no items", nonEmpty)
	}
	t.Logf("assembly mechanism OK: container %s -> %d items (e.g. %s x%d)",
		nonEmpty, len(synthetic.Inventory.Items["CommonContainerId"]),
		synthetic.Inventory.Items["CommonContainerId"][0].StaticID,
		synthetic.Inventory.Items["CommonContainerId"][0].Count)
}

// TestLevelPerf records parse time for AC4.
func TestLevelPerf(t *testing.T) {
	data := readTestdata(t, "Level.sav")
	start := time.Now()
	lvl, err := LoadLevelBytes(data)
	if err != nil {
		t.Fatalf("LoadLevelBytes: %v", err)
	}
	t.Logf("parsed Level.sav (%d compressed bytes) in %s: players=%d pals=%d guilds=%d containers=%d",
		len(data), time.Since(start), len(lvl.Players), len(lvl.Pals), len(lvl.Guilds), len(lvl.itemContainers))
}

func gvasBytesOf(t *testing.T, data []byte) []byte {
	t.Helper()
	b, _, err := DecompressToGVAS(data)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	return b
}
