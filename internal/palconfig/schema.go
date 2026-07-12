// Package palconfig is the single source of truth for Palworld dedicated
// server configuration: the OptionSettings parameter registry, PalWorldSettings.ini
// read/write, and launch-argument handling.
package palconfig

// ParamType classifies how a parameter is edited and serialized into the
// OptionSettings line.
type ParamType string

const (
	TypeBool   ParamType = "bool"   // True/False, unquoted
	TypeInt    ParamType = "int"    // integer, unquoted
	TypeFloat  ParamType = "float"  // float, unquoted (6 decimals)
	TypeString ParamType = "string" // arbitrary text, double-quoted
	TypeEnum   ParamType = "enum"   // fixed set, unquoted
	TypeRaw    ParamType = "raw"    // opaque value (tuples/lists), passed through verbatim
)

// Category groups parameters for the UI, matching the official documentation.
const (
	CatPerformances     = "performances"
	CatServerManagement = "serverManagement"
	CatFeatures         = "features"
	CatGameBalances     = "gameBalances"
)

// ParamDef describes one OptionSettings parameter.
//
// Default is the fallback value in INI-native form (floats carry decimals,
// strings carry no surrounding quotes here — quoting is applied at
// serialization time). The authoritative per-version defaults come from the
// installed DefaultPalWorldSettings.ini when present; this registry is the
// fallback and drives the UI form + type coercion.
type ParamDef struct {
	Key      string    `json:"key"`
	Type     ParamType `json:"type"`
	Default  string    `json:"default"`
	Category string    `json:"category"`
	Options  []string  `json:"options,omitempty"`
}

// Params is the full OptionSettings registry (official documentation order,
// grouped by category).
var Params = []ParamDef{
	// ---- Performances ----
	{Key: "BaseCampMaxNum", Type: TypeInt, Default: "128", Category: CatPerformances},
	{Key: "BaseCampMaxNumInGuild", Type: TypeInt, Default: "4", Category: CatPerformances},
	{Key: "BaseCampWorkerMaxNum", Type: TypeInt, Default: "15", Category: CatPerformances},
	{Key: "ItemContainerForceMarkDirtyInterval", Type: TypeFloat, Default: "1.000000", Category: CatPerformances},
	{Key: "MaxBuildingLimitNum", Type: TypeInt, Default: "0", Category: CatPerformances},
	{Key: "PhysicsActiveDropItemMaxNum", Type: TypeInt, Default: "1000", Category: CatPerformances},
	{Key: "ServerReplicatePawnCullDistance", Type: TypeFloat, Default: "15000.000000", Category: CatPerformances},

	// ---- Server management ----
	{Key: "AdminPassword", Type: TypeString, Default: "", Category: CatServerManagement},
	{Key: "AllowConnectPlatform", Type: TypeRaw, Default: "Steam", Category: CatServerManagement},
	{Key: "bAllowClientMod", Type: TypeBool, Default: "False", Category: CatServerManagement},
	{Key: "bEnableBuildingPlayerUIdDisplay", Type: TypeBool, Default: "False", Category: CatServerManagement},
	{Key: "bIsShowJoinLeftMessage", Type: TypeBool, Default: "True", Category: CatServerManagement},
	{Key: "bIsUseBackupSaveData", Type: TypeBool, Default: "True", Category: CatServerManagement},
	{Key: "ChatPostLimitPerMinute", Type: TypeInt, Default: "10", Category: CatServerManagement},
	{Key: "CrossplayPlatforms", Type: TypeRaw, Default: "(Steam,Xbox,PS5,Mac)", Category: CatServerManagement},
	{Key: "LogFormatType", Type: TypeEnum, Default: "Text", Category: CatServerManagement, Options: []string{"Text", "Json"}},
	{Key: "PublicIP", Type: TypeString, Default: "", Category: CatServerManagement},
	{Key: "PublicPort", Type: TypeInt, Default: "8211", Category: CatServerManagement},
	{Key: "RCONEnabled", Type: TypeBool, Default: "False", Category: CatServerManagement},
	{Key: "RCONPort", Type: TypeInt, Default: "25575", Category: CatServerManagement},
	{Key: "RESTAPIEnabled", Type: TypeBool, Default: "False", Category: CatServerManagement},
	{Key: "RESTAPIPort", Type: TypeInt, Default: "8212", Category: CatServerManagement},
	{Key: "ServerDescription", Type: TypeString, Default: "", Category: CatServerManagement},
	{Key: "ServerName", Type: TypeString, Default: "Default Palworld Server", Category: CatServerManagement},
	{Key: "ServerPassword", Type: TypeString, Default: "", Category: CatServerManagement},
	{Key: "ServerPlayerMaxNum", Type: TypeInt, Default: "32", Category: CatServerManagement},

	// ---- Features ----
	{Key: "AutoResetGuildTimeNoOnlinePlayers", Type: TypeFloat, Default: "72.000000", Category: CatFeatures},
	{Key: "bAllowEnhanceStat_Attack", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bAllowEnhanceStat_Health", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bAllowEnhanceStat_Stamina", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bAllowEnhanceStat_Weight", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bAllowEnhanceStat_WorkSpeed", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bAllowGlobalPalboxExport", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bAllowGlobalPalboxImport", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bAutoResetGuildNoOnlinePlayers", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bBuildAreaLimit", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bCharacterRecreateInHardcore", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bDisplayPvPItemNumOnWorldMap_BaseCamp", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bDisplayPvPItemNumOnWorldMap_Player", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bEnableFastTravel", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bEnableFastTravelOnlyBaseCamp", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bEnableInvaderEnemy", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bEnableVoiceChat", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bExistPlayerAfterLogout", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bHardcore", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bInvisibleOtherGuildBaseCampAreaFX", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bIsPvP", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bIsRandomizerPalLevelRandom", Type: TypeBool, Default: "False", Category: CatFeatures},
	{Key: "bIsStartLocationSelectByMap", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "bShowPlayerList", Type: TypeBool, Default: "True", Category: CatFeatures},
	{Key: "RandomizerSeed", Type: TypeString, Default: "", Category: CatFeatures},
	{Key: "RandomizerType", Type: TypeEnum, Default: "None", Category: CatFeatures, Options: []string{"None", "Region", "All"}},
	{Key: "VoiceChatMaxVolumeDistance", Type: TypeFloat, Default: "1000.000000", Category: CatFeatures},
	{Key: "VoiceChatZeroVolumeDistance", Type: TypeFloat, Default: "3000.000000", Category: CatFeatures},

	// ---- Game balances ----
	{Key: "AdditionalDropItemNumWhenPlayerKillingInPvPMode", Type: TypeInt, Default: "1", Category: CatGameBalances},
	{Key: "AdditionalDropItemWhenPlayerKillingInPvPMode", Type: TypeRaw, Default: "None", Category: CatGameBalances},
	{Key: "bAdditionalDropItemWhenPlayerKillingInPvPMode", Type: TypeBool, Default: "False", Category: CatGameBalances},
	{Key: "BlockRespawnTime", Type: TypeInt, Default: "0", Category: CatGameBalances},
	{Key: "bPalLost", Type: TypeBool, Default: "False", Category: CatGameBalances},
	{Key: "BuildObjectDamageRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "BuildObjectDeteriorationDamageRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "CollectionDropRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "CollectionObjectHpRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "CollectionObjectRespawnSpeedRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "DayTimeSpeedRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "DeathPenalty", Type: TypeEnum, Default: "All", Category: CatGameBalances, Options: []string{"None", "Item", "ItemAndEquipment", "All"}},
	{Key: "DenyTechnologyList", Type: TypeRaw, Default: "", Category: CatGameBalances},
	{Key: "EnemyDropItemRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "EquipmentDurabilityDamageRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "ExpRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "GuildPlayerMaxNum", Type: TypeInt, Default: "20", Category: CatGameBalances},
	{Key: "GuildRejoinCooldownMinutes", Type: TypeInt, Default: "0", Category: CatGameBalances},
	{Key: "ItemCorruptionMultiplier", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "ItemWeightRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "MonsterFarmActionSpeedRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "NightTimeSpeedRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PalAutoHPRegeneRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PalAutoHpRegeneRateInSleep", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PalCaptureRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PalDamageRateAttack", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PalDamageRateDefense", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PalEggDefaultHatchingTime", Type: TypeFloat, Default: "72.000000", Category: CatGameBalances},
	{Key: "PalSpawnNumRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PalStaminaDecreaceRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PalStomachDecreaceRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PlayerAutoHPRegeneRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PlayerAutoHpRegeneRateInSleep", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PlayerDamageRateAttack", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PlayerDamageRateDefense", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PlayerStaminaDecreaceRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "PlayerStomachDecreaceRate", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "RespawnPenaltyDurationThreshold", Type: TypeFloat, Default: "0.000000", Category: CatGameBalances},
	{Key: "RespawnPenaltyTimeScale", Type: TypeFloat, Default: "1.000000", Category: CatGameBalances},
	{Key: "SupplyDropSpan", Type: TypeInt, Default: "180", Category: CatGameBalances},
}

// paramIndex maps Key -> ParamDef for O(1) lookups.
var paramIndex = func() map[string]ParamDef {
	m := make(map[string]ParamDef, len(Params))
	for _, p := range Params {
		m[p.Key] = p
	}
	return m
}()

// Lookup returns the ParamDef for a key and whether it is a registered param.
func Lookup(key string) (ParamDef, bool) {
	p, ok := paramIndex[key]
	return p, ok
}

// Defaults returns a fresh map of every registered parameter at its fallback value.
func Defaults() map[string]string {
	m := make(map[string]string, len(Params))
	for _, p := range Params {
		m[p.Key] = p.Default
	}
	return m
}
