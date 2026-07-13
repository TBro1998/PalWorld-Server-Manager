package palsave

import "github.com/TBro1998/PalWorld-Server-Manager/internal/palsave/gvas"

// Player is a semantic view of a human player character.
type Player struct {
	UID        string
	InstanceID string
	NickName   string
	Level      int
	Exp        int64
	GroupID    string // guild/group id from the character RawData

	// Container ids from Players/<uid>.sav (populated by LoadPlayer).
	Inventory *Inventory
}

// Pal is a semantic view of a captured/owned pal.
type Pal struct {
	InstanceID    string
	OwnerUID      string
	CharacterID   string // species, e.g. "SheepBall"
	NickName      string
	Level         int
	Exp           int64
	Gender        string
	Rank          int
	TalentHP      int
	TalentMelee   int
	TalentShot    int
	TalentDefense int
	PassiveSkills []string
}

// GuildMember is one player in a guild's roster.
type GuildMember struct {
	UID        string
	Name       string
	LastOnline int64
	Role       int
}

// Guild is a semantic view of a player guild.
type Guild struct {
	GroupID       string
	GuildName     string
	BaseCampLevel int
	AdminUID      string
	Members       []GuildMember
	HandleIDs     []string // instance ids of characters owned by the guild
}

// Item is one inventory slot's resolved item.
type Item struct {
	Container        string // container role (e.g. "CommonContainerId")
	SlotIndex        int
	Count            int
	StaticID         string
	DynamicID        DynamicID
	ItemType         string // "" if not a dynamic item; else egg/armor/weapon/unknown
	Durability       float64
	RemainingBullets int
	PassiveSkills    []string
}

// Inventory holds a player's container ids and, once assembled, their items.
type Inventory struct {
	CommonContainerID           string
	DropSlotContainerID         string
	EssentialContainerID        string
	WeaponLoadOutContainerID    string
	PlayerEquipArmorContainerID string
	FoodEquipContainerID        string
	OtomoCharacterContainerID   string
	PalStorageContainerID       string

	// Items grouped by container role, assembled by AttachInventories.
	Items map[string][]Item
}

// Level is the parsed, semantic view of a Level.sav.
type Level struct {
	Header  gvas.GvasHeader
	Players []Player
	Pals    []Pal
	Guilds  []Guild

	// Internal indexes for cross-file inventory assembly.
	itemContainers map[string][]ItemContainerSlot // container GUID -> slots
	dynamicItems   map[string]DynamicItemData     // local_id_in_created_world -> data
}

// ItemContainers exposes a read-only view of the container GUID -> slots index.
func (l *Level) ItemContainers() map[string][]ItemContainerSlot { return l.itemContainers }
