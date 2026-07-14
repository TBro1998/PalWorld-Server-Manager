# 纯Go读取Palworld存档(玩家/公会/背包/帕鲁)

## Goal

参考 `temp/palsav`(palworld-save-tools 的 Oodle 分支),用**纯 Go**实现 Palworld 存档解析，从 `Level.sav` 与 `Players/*.sav` 中读取**玩家、公会、背包、帕鲁**四类数据，并接入 REST API + 前端展示。

## Background / 参考实现结论

`temp/palsav` 的解码链分四层，已完成源码分析:

1. **压缩层** — SAV 头 12B(`PlZ`/`CNK`/`PlM` + save_type)。本项目真实存档为 **`PlM1`(0x31 = Oodle Kraken)**。
2. **GVAS 层** — magic `GVAS` + 版本/引擎/custom_versions/class 名，随后属性流以 `None` 结束。
3. **通用属性读取器**(`archive.py`,~818 行) — 基础类型 + `fstring`(ASCII/UTF-16)+ UUID 字节重排 + 15 种 Property + struct 分派;`type_hints` 决定 Map 键/值类型，`custom_properties` 对特定路径的 `RawData` 字节块用专用解码器。
4. **目标数据解码器**(`rawdata/`) — character / group / item_container(+slots)/ dynamic_item / character_container。

数据来源映射:
| 数据 | 来源属性 | 参考解码器 |
|---|---|---|
| 玩家 | `Level.sav::worldSaveData.CharacterSaveParameterMap`(`IsPlayer=true`) + `Players/*.sav::SaveData` | character |
| 帕鲁 | 同上(`IsPlayer` 缺失/false) | character |
| 公会 | `Level.sav::worldSaveData.GroupSaveDataMap`(`EPalGroupType::Guild`) | group |
| 背包 | `worldSaveData.ItemContainerSaveData`(+`.Slots`) + `DynamicItemSaveData` | item_container / item_container_slots / dynamic_item |

## Requirements

### 功能
- R1 纯 Go 解压 Oodle Kraken(`PlM`)存档;兼容 `PlZ`/`CNK`(zlib,标准库)。
- R2 纯 Go GVAS 头 + 通用属性读取器(仅**读取**;不实现写入/编码)。
- R3 支持"选择性解析":按顶层属性 size 跳过无关大块(植被/地图物件/建造/工作),仅解析四类目标数据所需的 Map。
- R4 四类数据的语义化 Go 结构体输出:
  - `Player{UID, Name, Level, Exp, HP, MaxHP, Location, GuildID, LastOnline, ...}`
  - `Pal{InstanceID, OwnerUID, Species(CharacterID), Nickname, Level, Exp, Gender, IsBoss/Rare, Rank, IVs(HP/Melee/Shot/Defense), PassiveSkills, EquipMoves, HP, ...}`
  - `Guild{GroupID, Name, BaseCampLevel, AdminUID, Members[]{UID,Name,LastOnline,Role}, BaseIDs, ...}`
  - `Item{SlotIndex, StaticID, Count, DynamicID, Durability?, RemainingBullets?, PassiveSkills?, EggCharacterID?}` + 容器归属
- R5 REST API 端点返回上述数据;前端页面展示。

### 约束
- C1 **纯 Go、单一二进制**:不得引入 cgo / 外部原生库 / 运行时下载 DLL(与项目"单一二进制"架构原则一致)。fixture 生成等**仅开发期**工具可临时用外部手段,不进入产物。
- C2 平台优先 Windows;保留跨平台可编译性(Oodle 端口须为可移植纯 Go)。
- C3 遵循现有分层:解析库置于 `internal/`,API 走 Gin `protected` 组,前端走静态导出 + `apiClient`。
- C4 只读存档,绝不写回 `.sav`。

## Acceptance Criteria

- [ ] AC1 纯 Go 解压真实 `Level.sav` / `Players/*.sav`(`PlM1`)得到的 GVAS 字节,与参考解压结果**逐字节一致**(golden fixture 对比)。
- [ ] AC2 `go build .` 全程无 cgo(`CGO_ENABLED=0` 可构建通过)。
- [ ] AC3 解析真实存档,玩家名/公会名/帕鲁昵称/背包物品等语义字段与游戏内/参考 JSON 一致(抽样校验)。
- [ ] AC4 REST 端点返回四类数据,前端页面可展示。
- [ ] AC5 大文件(实测 Level.sav 数十 MB 级解压后)解析在可接受时间内完成,无 panic。

## 子任务拆分(父任务持有整体需求与集成验收)

- **A `palsav-core-lib`** — 纯 Go 核心解析库:Oodle 端口 + GVAS 读取器 + 四类解码器 + 语义结构体 + 验证 harness。对应 R1–R4、AC1–AC3、AC5。风险最高,先做。
- **B `palsav-api-ui`** — REST 端点 + 前端展示,依赖 A 的输出结构体。对应 R5、AC4。A 跑通后再详细规划。

顺序:A → B(B 依赖 A 的公开结构体与 API)。

## Notes

- Oodle 无现成纯 Go 库,方案为**移植 `temp/palsav/palooz/ooz/dep/ooz/kraken.cpp`(仅解压路径)**。
- 验证策略见子任务 A 的 `design.md`:开发期用外部手段(参考工具 / cgo wrapper)一次性生成 golden 解压结果,Go 端口逐字节对齐后即丢弃外部依赖。
