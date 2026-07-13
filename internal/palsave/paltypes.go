package palsave

import "github.com/TBro1998/PalWorld-Server-Manager/internal/palsave/gvas"

// palworldTypeHints is the full PALWORLD_TYPE_HINTS map ported verbatim from
// temp/palsav/palsav/paltypes.py. It resolves struct types for map keys/values
// so the generic reader parses Palworld saves correctly.
var palworldTypeHints = map[string]string{
	".worldSaveData.CharacterContainerSaveData.Key":                                                                    "StructProperty",
	".worldSaveData.CharacterSaveParameterMap.Key":                                                                     "StructProperty",
	".worldSaveData.CharacterSaveParameterMap.Value":                                                                   "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Key":                                                                        "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value":                                                       "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Key":                                   "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Value":                                 "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value":                                                                      "StructProperty",
	".worldSaveData.ItemContainerSaveData.Key":                                                                         "StructProperty",
	".worldSaveData.MapObjectSaveData.MapObjectSaveData.ConcreteModel.ModuleMap.Value":                                 "StructProperty",
	".worldSaveData.MapObjectSaveData.MapObjectSaveData.Model.EffectMap.Value":                                         "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Key":                                                               "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value":                                                             "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Key":                   "Guid",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Value":                 "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Value.ItemMap.Value":   "StructProperty",
	".worldSaveData.WorkSaveData.WorkSaveData.WorkAssignMap.Value":                                                     "StructProperty",
	".worldSaveData.BaseCampSaveData.Key":                                                                              "Guid",
	".worldSaveData.BaseCampSaveData.Value":                                                                            "StructProperty",
	".worldSaveData.BaseCampSaveData.Value.ModuleMap.Value":                                                            "StructProperty",
	".worldSaveData.ItemContainerSaveData.Value":                                                                       "StructProperty",
	".worldSaveData.CharacterContainerSaveData.Value":                                                                  "StructProperty",
	".worldSaveData.GroupSaveDataMap.Key":                                                                              "Guid",
	".worldSaveData.GroupSaveDataMap.Value":                                                                            "StructProperty",
	".worldSaveData.EnemyCampSaveData.EnemyCampStatusMap.Value":                                                        "StructProperty",
	".worldSaveData.DungeonSaveData.DungeonSaveData.MapObjectSaveData.MapObjectSaveData.Model.EffectMap.Value":         "StructProperty",
	".worldSaveData.DungeonSaveData.DungeonSaveData.MapObjectSaveData.MapObjectSaveData.ConcreteModel.ModuleMap.Value": "StructProperty",
	".worldSaveData.InvaderSaveData.Key":                                                                               "Guid",
	".worldSaveData.InvaderSaveData.Value":                                                                             "StructProperty",
	".worldSaveData.OilrigSaveData.OilrigMap.Value":                                                                    "StructProperty",
	".worldSaveData.SupplySaveData.SupplyInfos.Key":                                                                    "Guid",
	".worldSaveData.SupplySaveData.SupplyInfos.Value":                                                                  "StructProperty",
	".worldSaveData.GuildExtraSaveDataMap.Key":                                                                         "Guid",
	".worldSaveData.GuildExtraSaveDataMap.Value":                                                                       "StructProperty",
	".worldSaveData.EnemyCampSaveData.EnemyCampStatusMap.Value.TreasureBoxInfoMapBySpawnerName.Value":                  "StructProperty",
	".worldSaveData.DungeonSaveData.DungeonSaveData.RewardSaveDataMap.Key":                                             "Guid",
	".worldSaveData.DungeonSaveData.DungeonSaveData.RewardSaveDataMap.Value":                                           "StructProperty",
	".SaveData.Local_MaxFriendshipPalIds.Key":                                                                          "Guid",
	".worldSaveData.InvaderDeclarationSaveData.ValidatedStartPointIds.StructProperty":                                  "Guid",
	".SaveData.Local_MaxFriendshipPalIds.Value":                                                                        "StructProperty",
}

// palworldCustomProperties registers the read-side custom decoders for the four
// data categories in scope (character, group, item containers, character
// containers, dynamic items). Decoders for base_camp/foliage/map_object/work/
// etc. are intentionally omitted: those properties parse generically (their
// RawData stays as raw bytes) or are skipped by selective parsing.
var palworldCustomProperties = map[string]gvas.DecodeFunc{
	".worldSaveData.GroupSaveDataMap":                                     decodeGroup,
	".worldSaveData.CharacterSaveParameterMap.Value.RawData":              decodeCharacter,
	".worldSaveData.ItemContainerSaveData.Value.RawData":                  decodeItemContainer,
	".worldSaveData.ItemContainerSaveData.Value.Slots.Slots.RawData":      decodeItemContainerSlots,
	".worldSaveData.CharacterContainerSaveData.Value.Slots.Slots.RawData": decodeCharacterContainer,
	".worldSaveData.DynamicItemSaveData.DynamicItemSaveData.RawData":      decodeDynamicItem,
}

// worldSaveDataWanted is the selective-parse allow-list for the top-level
// worldSaveData struct (R4).
var worldSaveDataWanted = map[string]bool{
	"CharacterSaveParameterMap":  true,
	"GroupSaveDataMap":           true,
	"ItemContainerSaveData":      true,
	"CharacterContainerSaveData": true,
	"DynamicItemSaveData":        true,
}
