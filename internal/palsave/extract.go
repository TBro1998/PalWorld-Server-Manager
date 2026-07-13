package palsave

import (
	"fmt"
	"os"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/palsave/gvas"
)

// LoadLevel reads and parses a Level.sav at path into a semantic Level.
func LoadLevel(path string) (*Level, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadLevelBytes(data)
}

// LoadLevelBytes parses raw Level.sav bytes into a semantic Level.
func LoadLevelBytes(data []byte) (*Level, error) {
	gvasBytes, _, err := DecompressToGVAS(data)
	if err != nil {
		return nil, err
	}
	h, world, err := gvas.ReadWorldSaveData(gvasBytes, palworldTypeHints, palworldCustomProperties, "worldSaveData", worldSaveDataWanted)
	if err != nil {
		return nil, fmt.Errorf("palsave: parse level: %w", err)
	}
	return extractLevel(h, world)
}

func extractLevel(h gvas.GvasHeader, world map[string]gvas.Property) (*Level, error) {
	l := &Level{
		Header:         h,
		itemContainers: map[string][]ItemContainerSlot{},
		dynamicItems:   map[string]DynamicItemData{},
	}

	extractCharacters(l, world)
	extractGuilds(l, world)
	extractItemContainers(l, world)
	extractDynamicItems(l, world)
	return l, nil
}

func extractCharacters(l *Level, world map[string]gvas.Property) {
	p, ok := world["CharacterSaveParameterMap"]
	if !ok {
		return
	}
	entries, ok := p.Value.([]gvas.MapEntry)
	if !ok {
		return
	}
	for _, e := range entries {
		key, _ := e.Key.(map[string]gvas.Property)
		val, _ := e.Value.(map[string]gvas.Property)
		if key == nil || val == nil {
			continue
		}
		instanceID := fieldString(key, "InstanceId")
		keyPlayerUID := fieldString(key, "PlayerUId")

		cd, ok := propCharacterData(val, "RawData")
		if !ok {
			continue
		}
		sp, ok := propStruct(cd.Object, "SaveParameter")
		if !ok {
			continue
		}

		if fieldBool(sp, "IsPlayer") {
			l.Players = append(l.Players, Player{
				UID:        keyPlayerUID,
				InstanceID: instanceID,
				NickName:   fieldString(sp, "NickName"),
				Level:      fieldInt(sp, "Level"),
				Exp:        fieldInt64(sp, "Exp"),
				GroupID:    cd.GroupID,
			})
			continue
		}

		l.Pals = append(l.Pals, Pal{
			InstanceID:    instanceID,
			OwnerUID:      fieldString(sp, "OwnerPlayerUId"),
			CharacterID:   fieldString(sp, "CharacterID"),
			NickName:      fieldString(sp, "NickName"),
			Level:         fieldInt(sp, "Level"),
			Exp:           fieldInt64(sp, "Exp"),
			Gender:        fieldString(sp, "Gender"),
			Rank:          fieldInt(sp, "Rank"),
			TalentHP:      fieldInt(sp, "Talent_HP"),
			TalentMelee:   fieldInt(sp, "Talent_Melee"),
			TalentShot:    fieldInt(sp, "Talent_Shot"),
			TalentDefense: fieldInt(sp, "Talent_Defense"),
			PassiveSkills: fieldStrings(sp, "PassiveSkillList"),
		})
	}
}

func extractGuilds(l *Level, world map[string]gvas.Property) {
	p, ok := world["GroupSaveDataMap"]
	if !ok {
		return
	}
	entries, ok := p.Value.([]gvas.MapEntry)
	if !ok {
		return
	}
	for _, e := range entries {
		val, _ := e.Value.(map[string]gvas.Property)
		if val == nil {
			continue
		}
		rd, ok := val["RawData"]
		if !ok {
			continue
		}
		gd, ok := rd.Value.(GroupData)
		if !ok || gd.GroupType != "EPalGroupType::Guild" {
			continue
		}
		g := Guild{
			GroupID:       gd.GroupID,
			GuildName:     gd.GuildName,
			BaseCampLevel: int(gd.BaseCampLevel),
			AdminUID:      gd.AdminPlayerUID,
		}
		for _, m := range gd.Players {
			g.Members = append(g.Members, GuildMember{
				UID:        m.PlayerUID,
				Name:       m.PlayerName,
				LastOnline: m.LastOnlineRealTime,
				Role:       int(m.Role),
			})
		}
		for _, h := range gd.IndividualCharacterHandleIDs {
			g.HandleIDs = append(g.HandleIDs, h["instance_id"])
		}
		l.Guilds = append(l.Guilds, g)
	}
}

func extractItemContainers(l *Level, world map[string]gvas.Property) {
	p, ok := world["ItemContainerSaveData"]
	if !ok {
		return
	}
	entries, ok := p.Value.([]gvas.MapEntry)
	if !ok {
		return
	}
	for _, e := range entries {
		key, _ := e.Key.(map[string]gvas.Property)
		val, _ := e.Value.(map[string]gvas.Property)
		if key == nil || val == nil {
			continue
		}
		containerID := fieldString(key, "ID")
		if containerID == "" {
			continue
		}
		slotsProp, ok := val["Slots"]
		if !ok {
			continue
		}
		av, ok := slotsProp.Value.(gvas.ArrayValue)
		if !ok {
			continue
		}
		vals, _ := av.Values.([]any)
		var slots []ItemContainerSlot
		for _, sv := range vals {
			sm, ok := sv.(map[string]gvas.Property)
			if !ok {
				continue
			}
			rd, ok := sm["RawData"]
			if !ok {
				continue
			}
			if slot, ok := rd.Value.(ItemContainerSlot); ok {
				slots = append(slots, slot)
			}
		}
		l.itemContainers[containerID] = slots
	}
}

// extractDynamicItems walks the DynamicItemSaveData structure (when present)
// and indexes decoded dynamic items by their local id. The layout mirrors the
// custom path .worldSaveData.DynamicItemSaveData.DynamicItemSaveData.RawData.
func extractDynamicItems(l *Level, world map[string]gvas.Property) {
	p, ok := world["DynamicItemSaveData"]
	if !ok {
		return
	}
	inner, ok := p.Value.(map[string]gvas.Property)
	if !ok {
		return
	}
	arrProp, ok := inner["DynamicItemSaveData"]
	if !ok {
		return
	}
	av, ok := arrProp.Value.(gvas.ArrayValue)
	if !ok {
		return
	}
	vals, _ := av.Values.([]any)
	for _, sv := range vals {
		sm, ok := sv.(map[string]gvas.Property)
		if !ok {
			continue
		}
		rd, ok := sm["RawData"]
		if !ok {
			continue
		}
		if di, ok := rd.Value.(DynamicItemData); ok && di.LocalIDInCreatedWorld != "" {
			l.dynamicItems[di.LocalIDInCreatedWorld] = di
		}
	}
}

// LoadPlayer reads a Players/<uid>.sav file and returns its Player with the
// inventory container ids populated (but not yet the item contents).
func LoadPlayer(path string) (*Player, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadPlayerBytes(data)
}

// LoadPlayerBytes parses raw Players/<uid>.sav bytes.
func LoadPlayerBytes(data []byte) (*Player, error) {
	gvasBytes, _, err := DecompressToGVAS(data)
	if err != nil {
		return nil, err
	}
	f, err := gvas.ReadFull(gvasBytes, palworldTypeHints, palworldCustomProperties)
	if err != nil {
		return nil, fmt.Errorf("palsave: parse player: %w", err)
	}
	sd, ok := propStruct(f.Properties, "SaveData")
	if !ok {
		return nil, fmt.Errorf("palsave: player save has no SaveData")
	}

	pl := &Player{
		UID:       fieldString(sd, "PlayerUId"),
		Inventory: &Inventory{Items: map[string][]Item{}},
	}
	if ind, ok := propStruct(sd, "IndividualId"); ok {
		pl.InstanceID = fieldString(ind, "InstanceId")
		if pl.UID == "" {
			pl.UID = fieldString(ind, "PlayerUId")
		}
	}
	if inv, ok := propStruct(sd, "InventoryInfo"); ok {
		pl.Inventory.CommonContainerID = containerID(inv, "CommonContainerId")
		pl.Inventory.DropSlotContainerID = containerID(inv, "DropSlotContainerId")
		pl.Inventory.EssentialContainerID = containerID(inv, "EssentialContainerId")
		pl.Inventory.WeaponLoadOutContainerID = containerID(inv, "WeaponLoadOutContainerId")
		pl.Inventory.PlayerEquipArmorContainerID = containerID(inv, "PlayerEquipArmorContainerId")
		pl.Inventory.FoodEquipContainerID = containerID(inv, "FoodEquipContainerId")
	}
	pl.Inventory.OtomoCharacterContainerID = containerID(sd, "OtomoCharacterContainerId")
	pl.Inventory.PalStorageContainerID = containerID(sd, "PalStorageContainerId")
	return pl, nil
}

// AttachInventories fills each player's Inventory.Items by cross-referencing
// its container ids against this level's item containers and dynamic items (R6).
func (l *Level) AttachInventories(players []*Player) {
	for _, pl := range players {
		if pl == nil || pl.Inventory == nil {
			continue
		}
		inv := pl.Inventory
		if inv.Items == nil {
			inv.Items = map[string][]Item{}
		}
		roles := map[string]string{
			"CommonContainerId":           inv.CommonContainerID,
			"DropSlotContainerId":         inv.DropSlotContainerID,
			"EssentialContainerId":        inv.EssentialContainerID,
			"WeaponLoadOutContainerId":    inv.WeaponLoadOutContainerID,
			"PlayerEquipArmorContainerId": inv.PlayerEquipArmorContainerID,
			"FoodEquipContainerId":        inv.FoodEquipContainerID,
		}
		for role, id := range roles {
			if id == "" {
				continue
			}
			slots := l.itemContainers[id]
			for _, s := range slots {
				if s.StaticID == "" {
					continue
				}
				item := Item{
					Container: role,
					SlotIndex: int(s.SlotIndex),
					Count:     int(s.Count),
					StaticID:  s.StaticID,
					DynamicID: s.DynamicID,
				}
				if di, ok := l.dynamicItems[s.DynamicID.LocalIDInCreatedWorld]; ok {
					item.ItemType = di.Type
					item.Durability = di.Durability
					item.RemainingBullets = int(di.RemainingBullets)
					item.PassiveSkills = di.PassiveSkillList
				}
				inv.Items[role] = append(inv.Items[role], item)
			}
		}
	}
}

// --- field accessors ---

func propStruct(m map[string]gvas.Property, key string) (map[string]gvas.Property, bool) {
	p, ok := m[key]
	if !ok {
		return nil, false
	}
	sm, ok := p.Value.(map[string]gvas.Property)
	return sm, ok
}

func propCharacterData(m map[string]gvas.Property, key string) (CharacterData, bool) {
	p, ok := m[key]
	if !ok {
		return CharacterData{}, false
	}
	cd, ok := p.Value.(CharacterData)
	return cd, ok
}

// containerID reads a nested {ID: guid} struct's guid string.
func containerID(m map[string]gvas.Property, key string) string {
	sub, ok := propStruct(m, key)
	if !ok {
		return ""
	}
	return fieldString(sub, "ID")
}

// fieldString extracts a string-like value: StrProperty/NameProperty, a struct
// Guid value, or an enum member name.
func fieldString(m map[string]gvas.Property, key string) string {
	p, ok := m[key]
	if !ok {
		return ""
	}
	switch v := p.Value.(type) {
	case string:
		return v
	case gvas.EnumValue:
		return v.Value
	case gvas.ByteValue:
		if s, ok := v.Value.(string); ok {
			return s
		}
	}
	return ""
}

func fieldInt(m map[string]gvas.Property, key string) int {
	return int(fieldInt64(m, key))
}

func fieldInt64(m map[string]gvas.Property, key string) int64 {
	p, ok := m[key]
	if !ok {
		return 0
	}
	switch v := p.Value.(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	}
	return 0
}

func fieldBool(m map[string]gvas.Property, key string) bool {
	p, ok := m[key]
	if !ok {
		return false
	}
	b, _ := p.Value.(bool)
	return b
}

func fieldStrings(m map[string]gvas.Property, key string) []string {
	p, ok := m[key]
	if !ok {
		return nil
	}
	av, ok := p.Value.(gvas.ArrayValue)
	if !ok {
		return nil
	}
	ss, _ := av.Values.([]string)
	return ss
}
