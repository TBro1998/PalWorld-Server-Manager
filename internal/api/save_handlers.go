package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palsave"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// --- response DTOs ---
//
// palsave's structs carry no json tags (they would serialize as PascalCase), so
// the handler layer maps them to explicit lowerCamel DTOs to match the frontend
// conventions used elsewhere (see PalPlayers etc.). Keeping the mapping in one
// place also isolates the frontend contract from palsave field renames.

type savePlayerDTO struct {
	UID        string `json:"uid"`
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
	Level      int    `json:"level"`
	Exp        int64  `json:"exp"`
	GuildID    string `json:"guildId"`
	GuildName  string `json:"guildName,omitempty"`
}

type saveTalentDTO struct {
	HP      int `json:"hp"`
	Melee   int `json:"melee"`
	Shot    int `json:"shot"`
	Defense int `json:"defense"`
}

type savePalDTO struct {
	InstanceID string        `json:"instanceId"`
	OwnerUID   string        `json:"ownerUid"`
	Species    string        `json:"species"`
	Name       string        `json:"name"`
	Level      int           `json:"level"`
	Exp        int64         `json:"exp"`
	Gender     string        `json:"gender"`
	Rank       int           `json:"rank"`
	Talent     saveTalentDTO `json:"talent"`
	Passives   []string      `json:"passives"`
}

type saveGuildMemberDTO struct {
	UID        string `json:"uid"`
	Name       string `json:"name"`
	Role       int    `json:"role"`
	LastOnline int64  `json:"lastOnline"`
}

type saveGuildDTO struct {
	GuildID       string               `json:"guildId"`
	Name          string               `json:"name"`
	BaseCampLevel int                  `json:"baseCampLevel"`
	AdminUID      string               `json:"adminUid"`
	Members       []saveGuildMemberDTO `json:"members"`
}

type saveItemDTO struct {
	Container  string   `json:"container"`
	Slot       int      `json:"slot"`
	Count      int      `json:"count"`
	StaticID   string   `json:"staticId"`
	ItemType   string   `json:"itemType,omitempty"`
	Durability float64  `json:"durability,omitempty"`
	Passives   []string `json:"passives,omitempty"`
}

// saveContext holds the resolved server plus the parsed level and the world's
// Players directory, produced by saveResolve.
type saveContext struct {
	server     models.Server
	level      *palsave.Level
	playersDir string
}

// saveResolve parses :id, loads the server, locates its world save and returns
// the cached parsed Level. On any failure it writes the appropriate structured
// error response and returns ok=false (the caller must return immediately).
func (r *Router) saveResolve(c *gin.Context) (saveContext, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return saveContext{}, false
	}

	var s models.Server
	if err := r.db.First(&s, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return saveContext{}, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return saveContext{}, false
	}

	levelPath, playersDir, err := palsave.LocateWorld(s.InstallPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Save not found"})
		return saveContext{}, false
	}

	level, err := r.saves.Level(s.ID, levelPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse save"})
		return saveContext{}, false
	}

	return saveContext{server: s, level: level, playersDir: playersDir}, true
}

// guildNames builds a groupID -> guild name lookup for annotating players.
func guildNames(level *palsave.Level) map[string]string {
	m := make(map[string]string, len(level.Guilds))
	for _, g := range level.Guilds {
		m[g.GroupID] = g.GuildName
	}
	return m
}

// SavePlayers returns every player recorded in the level save (online or not).
func (r *Router) SavePlayers(c *gin.Context) {
	ctx, ok := r.saveResolve(c)
	if !ok {
		return
	}
	names := guildNames(ctx.level)
	players := make([]savePlayerDTO, 0, len(ctx.level.Players))
	for _, p := range ctx.level.Players {
		players = append(players, savePlayerDTO{
			UID:        p.UID,
			InstanceID: p.InstanceID,
			Name:       p.NickName,
			Level:      p.Level,
			Exp:        p.Exp,
			GuildID:    p.GroupID,
			GuildName:  names[p.GroupID],
		})
	}
	c.JSON(http.StatusOK, gin.H{"players": players})
}

// SaveGuilds returns all guilds with their rosters.
func (r *Router) SaveGuilds(c *gin.Context) {
	ctx, ok := r.saveResolve(c)
	if !ok {
		return
	}
	guilds := make([]saveGuildDTO, 0, len(ctx.level.Guilds))
	for _, g := range ctx.level.Guilds {
		members := make([]saveGuildMemberDTO, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, saveGuildMemberDTO{
				UID:        m.UID,
				Name:       m.Name,
				Role:       m.Role,
				LastOnline: m.LastOnline,
			})
		}
		guilds = append(guilds, saveGuildDTO{
			GuildID:       g.GroupID,
			Name:          g.GuildName,
			BaseCampLevel: g.BaseCampLevel,
			AdminUID:      g.AdminUID,
			Members:       members,
		})
	}
	c.JSON(http.StatusOK, gin.H{"guilds": guilds})
}

// SavePlayerPals returns the pals owned by a specific player. All pals live in
// the Level.sav, so this needs no per-player file read.
func (r *Router) SavePlayerPals(c *gin.Context) {
	ctx, ok := r.saveResolve(c)
	if !ok {
		return
	}
	uid := c.Param("uid")
	pals := make([]savePalDTO, 0)
	for _, p := range ctx.level.Pals {
		if p.OwnerUID != uid {
			continue
		}
		pals = append(pals, savePalDTO{
			InstanceID: p.InstanceID,
			OwnerUID:   p.OwnerUID,
			Species:    p.CharacterID,
			Name:       p.NickName,
			Level:      p.Level,
			Exp:        p.Exp,
			Gender:     p.Gender,
			Rank:       p.Rank,
			Talent: saveTalentDTO{
				HP:      p.TalentHP,
				Melee:   p.TalentMelee,
				Shot:    p.TalentShot,
				Defense: p.TalentDefense,
			},
			Passives: p.PassiveSkills,
		})
	}
	c.JSON(http.StatusOK, gin.H{"pals": pals})
}

// SavePlayerInventory returns a player's inventory grouped by container role.
// It reads the player's own .sav for container ids, then cross-references the
// level's item containers and dynamic items to resolve item contents.
func (r *Router) SavePlayerInventory(c *gin.Context) {
	ctx, ok := r.saveResolve(c)
	if !ok {
		return
	}
	uid := c.Param("uid")

	playerPath, err := palsave.ResolvePlayerSave(ctx.playersDir, uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player save not found"})
		return
	}
	pl, err := palsave.LoadPlayer(playerPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse player save"})
		return
	}

	ctx.level.AttachInventories([]*palsave.Player{pl})

	inventory := map[string][]saveItemDTO{}
	if pl.Inventory != nil {
		for role, items := range pl.Inventory.Items {
			dtos := make([]saveItemDTO, 0, len(items))
			for _, it := range items {
				dtos = append(dtos, saveItemDTO{
					Container:  it.Container,
					Slot:       it.SlotIndex,
					Count:      it.Count,
					StaticID:   it.StaticID,
					ItemType:   it.ItemType,
					Durability: it.Durability,
					Passives:   it.PassiveSkills,
				})
			}
			inventory[role] = dtos
		}
	}
	c.JSON(http.StatusOK, gin.H{"inventory": inventory})
}
