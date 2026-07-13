# Palworld存档核心解析库(纯Go)

> 父任务:`07-13-palworld-sav-reader-go`。本子任务交付纯 Go 核心解析库,不含 REST/UI(见子任务 B)。

## Goal

在 `internal/palsave/` 提供纯 Go 库:输入 `.sav` 文件路径/字节,输出玩家、公会、背包、帕鲁的语义化 Go 结构体。全程无 cgo、可 `CGO_ENABLED=0` 构建。

## Requirements

- R1 **解压**:纯 Go 解压 `PlM`(Oodle)。**实测本项目存档块头 `8c 0a` = decoder_type 10 = Mermaid**(非 Kraken);已同时移植 Kraken(type 6)与 Mermaid(type 10),LZNA/BitKnit/Leviathan(5/11/12)返回 ErrUnsupported。兼容 `PlZ`(双层 zlib)/`CNK`(zlib,标准库 `compress/zlib`)。校验 uncompressed_len。
- R2 **GVAS 读取器**:头部解析 + 通用属性只读读取器(fstring ASCII/UTF-16、UUID 重排、Int/UInt16/32/64/Int64/FixedPoint64/Float/Str/Name/Enum/Bool/Byte/Array/Map/Set/Struct、struct 分派 Vector/Quat/Guid/LinearColor/Color/DateTime、tarray、packed/optional guid)。仅读取,无写入。
- R3 **type_hints + custom_properties** 机制:移植 `PALWORLD_TYPE_HINTS` 中与四类数据相关的条目;custom 解码器覆盖 character / group / item_container / item_container_slots / character_container / dynamic_item。
- R4 **选择性解析**:能只解析 `worldSaveData` 下的 `CharacterSaveParameterMap`、`GroupSaveDataMap`、`ItemContainerSaveData`、`CharacterContainerSaveData`、`DynamicItemSaveData`,跳过其余顶层属性(按 size 跳过)。
- R5 **语义映射**:把原始属性树映射为 Player / Pal / Guild / Item 结构体(字段清单见 design.md;未知字段可保留 raw 便于扩展)。
- R6 **跨文件组装**:解析 `Players/*.sav` 得到各库存容器 GUID,与 `Level.sav` 的 `ItemContainerSaveData` 关联,组装出玩家背包(装备/常规/饲料/武器槽等)。
- R7 **验证 harness**:提供对真实存档的解析测试/小 CLI,输出四类数据摘要,并支持与 golden 解压结果逐字节比对。

## Acceptance Criteria

- [ ] AC1 对真实 `Level.sav`、`Players/*.sav`(均为 `PlM1`)纯 Go 解压得到的 GVAS 字节 == golden(参考工具)结果,逐字节一致。
- [ ] AC2 `CGO_ENABLED=0 go build ./...` 通过;`go vet` 无异常;无第三方 cgo 依赖。
- [ ] AC3 抽样字段正确:玩家名、公会名与成员、帕鲁 CharacterID/昵称/等级、背包中至少若干物品 static_id 与数量,与参考 JSON 或游戏内一致。
- [ ] AC4 解析实测大 `Level.sav`(解压后数十 MB)无 panic,耗时可接受(目标 < 数秒,记录实测)。
- [ ] AC5 GVAS 解析后 trailer == `\x00\x00\x00\x00`(全量解析路径),或选择性解析路径正确落在目标 Map 边界内。

## Non-Goals

- 不实现存档**写入/编码**(FArchiveWriter、encode_*)。
- 不实现 foliage / map_object / base_camp / work / concrete_model 等非四类数据的解码器。
- 不做 REST/UI(子任务 B)。
- 不实现 Oodle **压缩**(仅解压)。

## Notes

- 参考源码:`temp/palsav/palsav/{core,gvas,archive,paltypes}.py`、`rawdata/{character,group,item_container,item_container_slots,character_container,dynamic_item,common}.py`、`palooz/ooz/dep/ooz/kraken.cpp`。
