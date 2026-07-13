package palsave

import (
	"fmt"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/palsave/gvas"
)

// This file ports the read-side rawdata decoders from
// temp/palsav/palsav/rawdata/{character,group,item_container,
// item_container_slots,character_container,dynamic_item,common}.py. Only the
// decode paths are ported; encoders are intentionally absent.

// ---------------------------------------------------------------------------
// character.py
// ---------------------------------------------------------------------------

// CharacterData is the decoded RawData of a CharacterSaveParameterMap value.
type CharacterData struct {
	Object               map[string]gvas.Property
	UnknownBytes         []byte
	GroupID              string
	TrailingBytes        []byte
	TrailingUnknownBytes []byte
}

func decodeCharacter(r *gvas.Reader, typeName string, size int, path string) (gvas.Property, error) {
	if typeName != "ArrayProperty" {
		return gvas.Property{}, fmt.Errorf("character: expected ArrayProperty, got %s", typeName)
	}
	p := r.MustProperty(typeName, size, path, path)
	av, ok := p.Value.(gvas.ArrayValue)
	if !ok {
		return gvas.Property{}, fmt.Errorf("character: expected ArrayValue")
	}
	charBytes, _ := av.Values.([]byte)
	sub := r.InternalCopy(charBytes)
	var d CharacterData
	d.Object = sub.MustProperties("")
	d.UnknownBytes = sub.ByteList(4)
	d.GroupID = sub.GUID()
	d.TrailingBytes = sub.ByteList(4)
	if !sub.Eof() {
		d.TrailingUnknownBytes = sub.ReadToEnd()
	}
	p.Value = d
	return p, nil
}

// ---------------------------------------------------------------------------
// group.py
// ---------------------------------------------------------------------------

// GuildPlayer is one member entry of a guild.
type GuildPlayer struct {
	PlayerUID          string
	PlayerName         string
	LastOnlineRealTime int64
	Role               uint8 // EPalGuildRole (0 if not present in this layout)
}

// GroupData is the decoded RawData of a GroupSaveDataMap value.
type GroupData struct {
	GroupType                    string
	GroupID                      string
	GroupName                    string
	IndividualCharacterHandleIDs []map[string]string
	OrgType                      uint8
	BaseIDs                      []string
	BaseCampLevel                int32
	GuildName                    string
	AdminPlayerUID               string
	Players                      []GuildPlayer
}

func decodeGroup(r *gvas.Reader, typeName string, size int, path string) (gvas.Property, error) {
	if typeName != "MapProperty" {
		return gvas.Property{}, fmt.Errorf("group: expected MapProperty, got %s", typeName)
	}
	p := r.MustProperty(typeName, size, path, path)
	entries, ok := p.Value.([]gvas.MapEntry)
	if !ok {
		return gvas.Property{}, fmt.Errorf("group: expected map entries")
	}
	for i := range entries {
		m, ok := entries[i].Value.(map[string]gvas.Property)
		if !ok {
			return gvas.Property{}, fmt.Errorf("group: entry value not a struct")
		}
		groupType := enumString(m["GroupType"])
		rawProp := m["RawData"]
		av, ok := rawProp.Value.(gvas.ArrayValue)
		if !ok {
			return gvas.Property{}, fmt.Errorf("group: RawData not an array")
		}
		groupBytes, _ := av.Values.([]byte)
		gd, err := decodeGroupBytes(r, groupBytes, groupType)
		if err != nil {
			return gvas.Property{}, err
		}
		rawProp.Value = gd
		m["RawData"] = rawProp
	}
	return p, nil
}

func decodeGroupBytes(parent *gvas.Reader, groupBytes []byte, groupType string) (gd GroupData, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			if e, ok := rec.(error); ok {
				err = e
				return
			}
			err = fmt.Errorf("group: %v", rec)
		}
	}()
	r := parent.InternalCopy(groupBytes)
	gd.GroupType = groupType
	gd.GroupID = r.GUID()
	gd.GroupName = r.FString()
	gd.IndividualCharacterHandleIDs = readInstanceIDs(r)

	switch groupType {
	case "EPalGroupType::Guild", "EPalGroupType::IndependentGuild", "EPalGroupType::Organization":
		gd.OrgType = r.Byte()
	}
	if groupType == "EPalGroupType::Organization" {
		r.ByteList(12) // trailing_bytes
	}

	if groupType == "EPalGroupType::Guild" {
		r.ByteList(4)             // leading_bytes
		gd.BaseIDs = readGUIDs(r) // base_ids
		r.I32()                   // unknown_1
		gd.BaseCampLevel = r.I32()
		readGUIDs(r)               // map_object_instance_ids_base_camp_points
		gd.GuildName = r.FString() // guild_name
		r.GUID()                   // last_guild_name_modifier_player_uid
		readGuildMarkers(r)        // guild_markers
		readGuildTail(r, &gd)
	}
	if groupType == "EPalGroupType::IndependentGuild" {
		gd.BaseCampLevel = r.I32()
		readGUIDs(r) // map_object_instance_ids_base_camp_points
		gd.GuildName = r.FString()
		playerUID := r.GUID()
		r.FString() // guild_name_2
		lastOnline := r.I64()
		playerName := r.FString()
		gd.AdminPlayerUID = playerUID
		gd.Players = []GuildPlayer{{PlayerUID: playerUID, PlayerName: playerName, LastOnlineRealTime: lastOnline}}
	}
	if !r.Eof() {
		return gd, fmt.Errorf("group: EOF not reached (%d/%d)", r.Pos(), r.Size())
	}
	return gd, nil
}

func readGuildMarkers(r *gvas.Reader) {
	// FPalGuildMarkerData: marker_id guid, icon_location vector(3 doubles),
	// icon_type i32, owner_player_uid guid.
	count := r.U32()
	for i := uint32(0); i < count; i++ {
		r.GUID()
		r.F64()
		r.F64()
		r.F64()
		r.I32()
		r.GUID()
	}
}

// readGuildTail reproduces group.py::_read_guild_tail: try the v2 layout, and
// if it does not land exactly on EOF, rewind and use the v1 layout.
func readGuildTail(r *gvas.Reader, gd *GroupData) {
	start := r.Pos()
	var v2Admin string
	var v2Players []GuildPlayer
	err := gvas.Attempt(func() {
		readByteArray(r) // guild_chest_allowed_roles
		r.I32()          // unknown_i32
		v2Admin = r.GUID()
		v2Players = readGuildPlayersV2(r)
		// role_permissions
		rpCount := r.U32()
		for i := uint32(0); i < rpCount; i++ {
			r.Byte()         // role
			readByteArray(r) // permissions
		}
		r.ByteList(4) // trailing_bytes
	})
	if err == nil && r.Eof() {
		gd.AdminPlayerUID = v2Admin
		gd.Players = v2Players
		return
	}
	// Not v2: rewind and read the pre-update layout. Errors propagate.
	r.SeekTo(start)
	gd.AdminPlayerUID = r.GUID()
	gd.Players = readGuildPlayersV1(r)
	r.ByteList(4) // trailing_bytes
}

func readGuildPlayersV2(r *gvas.Reader) []GuildPlayer {
	count := r.U32()
	out := make([]GuildPlayer, 0, count)
	for i := uint32(0); i < count; i++ {
		uid := r.GUID()
		last := r.I64()
		name := r.FString()
		role := r.Byte()
		out = append(out, GuildPlayer{PlayerUID: uid, LastOnlineRealTime: last, PlayerName: name, Role: role})
	}
	return out
}

func readGuildPlayersV1(r *gvas.Reader) []GuildPlayer {
	count := r.U32()
	out := make([]GuildPlayer, 0, count)
	for i := uint32(0); i < count; i++ {
		uid := r.GUID()
		last := r.I64()
		name := r.FString()
		out = append(out, GuildPlayer{PlayerUID: uid, LastOnlineRealTime: last, PlayerName: name})
	}
	return out
}

// ---------------------------------------------------------------------------
// item_container.py
// ---------------------------------------------------------------------------

// ItemContainerData is the decoded RawData of an ItemContainerSaveData value.
type ItemContainerData struct {
	TypeA         []uint8
	TypeB         []uint8
	ItemStaticIDs []string
}

func decodeItemContainer(r *gvas.Reader, typeName string, size int, path string) (gvas.Property, error) {
	if typeName != "ArrayProperty" {
		return gvas.Property{}, fmt.Errorf("item_container: expected ArrayProperty, got %s", typeName)
	}
	p := r.MustProperty(typeName, size, path, path)
	av, _ := p.Value.(gvas.ArrayValue)
	dataBytes, _ := av.Values.([]byte)
	if len(dataBytes) == 0 {
		p.Value = nil
		return p, nil
	}
	sub := r.InternalCopy(dataBytes)
	var d ItemContainerData
	d.TypeA = readByteArray(sub)
	d.TypeB = readByteArray(sub)
	d.ItemStaticIDs = readStringArray(sub)
	p.Value = d
	return p, nil
}

// ---------------------------------------------------------------------------
// item_container_slots.py
// ---------------------------------------------------------------------------

// DynamicID identifies a dynamic item instance.
type DynamicID struct {
	CreatedWorldID        string
	LocalIDInCreatedWorld string
}

// ItemContainerSlot is one decoded slot RawData entry.
type ItemContainerSlot struct {
	SlotIndex int32
	Count     int32
	StaticID  string
	DynamicID DynamicID
}

func decodeItemContainerSlots(r *gvas.Reader, typeName string, size int, path string) (gvas.Property, error) {
	if typeName != "ArrayProperty" {
		return gvas.Property{}, fmt.Errorf("item_container_slots: expected ArrayProperty, got %s", typeName)
	}
	p := r.MustProperty(typeName, size, path, path)
	av, _ := p.Value.(gvas.ArrayValue)
	dataBytes, _ := av.Values.([]byte)
	if len(dataBytes) == 0 {
		p.Value = nil
		return p, nil
	}
	sub := r.InternalCopy(dataBytes)
	var s ItemContainerSlot
	s.SlotIndex = sub.I32()
	s.Count = sub.I32()
	s.StaticID = sub.FString()
	s.DynamicID = DynamicID{CreatedWorldID: sub.GUID(), LocalIDInCreatedWorld: sub.GUID()}
	// trailing_bytes consumed implicitly (read_to_end); ignored.
	p.Value = s
	return p, nil
}

// ---------------------------------------------------------------------------
// character_container.py
// ---------------------------------------------------------------------------

// CharacterContainerData is the decoded RawData of a CharacterContainerSaveData
// slot value.
type CharacterContainerData struct {
	PlayerUID         string
	InstanceID        string
	PermissionTribeID uint8
}

func decodeCharacterContainer(r *gvas.Reader, typeName string, size int, path string) (gvas.Property, error) {
	if typeName != "ArrayProperty" {
		return gvas.Property{}, fmt.Errorf("character_container: expected ArrayProperty, got %s", typeName)
	}
	p := r.MustProperty(typeName, size, path, path)
	av, _ := p.Value.(gvas.ArrayValue)
	dataBytes, _ := av.Values.([]byte)
	if len(dataBytes) == 0 {
		p.Value = nil
		return p, nil
	}
	sub := r.InternalCopy(dataBytes)
	var d CharacterContainerData
	d.PlayerUID = sub.GUID()
	d.InstanceID = sub.GUID()
	d.PermissionTribeID = sub.Byte()
	p.Value = d
	return p, nil
}

// ---------------------------------------------------------------------------
// dynamic_item.py
// ---------------------------------------------------------------------------

// DynamicItemData is the decoded RawData of a DynamicItemSaveData entry.
type DynamicItemData struct {
	Type                  string // "unknown" | "egg" | "armor" | "weapon"
	CreatedWorldID        string
	LocalIDInCreatedWorld string
	StaticID              string
	Durability            float64
	RemainingBullets      int32
	PassiveSkillList      []string
	CharacterID           string // egg
}

func decodeDynamicItem(r *gvas.Reader, typeName string, size int, path string) (gvas.Property, error) {
	if typeName != "ArrayProperty" {
		return gvas.Property{}, fmt.Errorf("dynamic_item: expected ArrayProperty, got %s", typeName)
	}
	p := r.MustProperty(typeName, size, path, path)
	av, _ := p.Value.(gvas.ArrayValue)
	dataBytes, _ := av.Values.([]byte)
	if len(dataBytes) == 0 {
		p.Value = nil
		return p, nil
	}
	sub := r.InternalCopy(dataBytes)
	var d DynamicItemData
	d.Type = "unknown"
	d.CreatedWorldID = sub.GUID()
	d.LocalIDInCreatedWorld = sub.GUID()
	d.StaticID = sub.FString()

	if tryReadEgg(sub, &d) {
		// egg fields filled
	} else if sub.Size()-sub.Pos() == 12 {
		d.Type = "armor"
		sub.ByteList(4) // leading_bytes
		d.Durability = sub.F32()
		sub.ByteList(4) // trailing_bytes
	} else {
		cur := sub.Pos()
		err := gvas.Attempt(func() {
			sub.ByteList(4) // leading_bytes
			d.Durability = sub.F32()
			d.RemainingBullets = sub.I32()
			d.PassiveSkillList = readStringArray(sub)
			sub.ByteList(4) // trailing_bytes
			// remaining bytes ignored
		})
		if err != nil {
			// not a weapon; keep as raw unknown
			sub.SeekTo(cur)
			d.Type = "unknown"
			d.Durability = 0
			d.RemainingBullets = 0
			d.PassiveSkillList = nil
		} else {
			d.Type = "weapon"
		}
	}
	p.Value = d
	return p, nil
}

// tryReadEgg mirrors dynamic_item.try_read_egg: attempt the egg layout; on any
// parse failure, rewind and report failure.
func tryReadEgg(r *gvas.Reader, d *DynamicItemData) bool {
	cur := r.Pos()
	var charID string
	err := gvas.Attempt(func() {
		r.ByteList(4) // leading_bytes
		charID = r.FString()
		r.MustProperties("") // object
		r.ByteList(28)       // trailing_bytes
		// remaining bytes ignored
	})
	if err != nil {
		r.SeekTo(cur)
		return false
	}
	d.Type = "egg"
	d.CharacterID = charID
	return true
}

// ---------------------------------------------------------------------------
// shared readers (archive.py helpers)
// ---------------------------------------------------------------------------

func readInstanceIDs(r *gvas.Reader) []map[string]string {
	count := r.U32()
	out := make([]map[string]string, 0, count)
	for i := uint32(0); i < count; i++ {
		out = append(out, r.InstanceID())
	}
	return out
}

func readGUIDs(r *gvas.Reader) []string {
	count := r.U32()
	out := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		out = append(out, r.GUID())
	}
	return out
}

func readByteArray(r *gvas.Reader) []uint8 {
	count := r.U32()
	out := make([]uint8, 0, count)
	for i := uint32(0); i < count; i++ {
		out = append(out, r.Byte())
	}
	return out
}

func readStringArray(r *gvas.Reader) []string {
	count := r.U32()
	out := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		out = append(out, r.FString())
	}
	return out
}

// enumString extracts the enum member string from a GroupType-style property,
// tolerating both EnumProperty and ByteProperty encodings.
func enumString(p gvas.Property) string {
	switch v := p.Value.(type) {
	case gvas.EnumValue:
		return v.Value
	case gvas.ByteValue:
		if s, ok := v.Value.(string); ok {
			return s
		}
	}
	return ""
}
