# Design — Palworld 存档核心解析库(纯Go)

## 1. 包结构

```
internal/palsave/
├── sav.go            # 顶层入口: LoadLevel(path)/LoadPlayer(path); SAV 头解析 + 解压分派
├── compress_zlib.go  # PlZ/CNK: compress/zlib (标准库)
├── oodle/            # 纯 Go Oodle Kraken 解压 (移植 ooz kraken.cpp 解压路径)
│   ├── kraken.go     #   Kraken_Decompress 主循环 + quantum/block 解析
│   ├── huffman.go    #   Huffman 解码器 (NewHuff / Huff_ReadCodeLengths / bit reader)
│   ├── tans.go       #   TANS 解码器
│   ├── bitreader.go  #   BitReader / BitReader2 (前向+后向)
│   └── kraken_test.go#   golden fixture 逐字节测试
├── gvas/
│   ├── reader.go     # FArchiveReader (只读)
│   ├── header.go     # GvasHeader.Read
│   ├── property.go   # 15 种 Property 读取 + struct/map/array/set 分派
│   └── types.go      # Property/Struct/Map 的 Go 表示 (Value 树)
├── paltypes.go       # PALWORLD_TYPE_HINTS 子集 + custom_properties 注册表
├── rawdata/
│   ├── character.go  # 帕鲁/玩家角色 RawData 解码
│   ├── group.go      # 公会 RawData 解码 (含 v1/v2 tail 试探)
│   ├── item_container.go / item_container_slots.go
│   ├── character_container.go
│   └── dynamic_item.go
├── model.go          # 语义结构体: Player / Pal / Guild / Item / Inventory
├── extract.go        # 属性树 → 语义结构体映射 + 跨文件组装
└── testdata/         # golden fixtures (小样本 .sav + 期望解压 .gvas + 期望字段)
```

对外 API(供子任务 B 使用):
```go
type Level struct {
    Players []Player
    Pals    []Pal
    Guilds  []Guild
    // 容器索引(内部组装用,可导出只读视图)
}
func LoadLevel(path string) (*Level, error)
func LoadLevelBytes(b []byte) (*Level, error)
func LoadPlayer(path string) (*Player, error)   // 解析 Players/<uid>.sav
// 组装:给定 Level + 各 Player.sav 的 inventoryInfo,填充 Player.Inventory
func (l *Level) AttachInventories(players []*Player)
```

## 2. 压缩层(sav.go / compress_zlib.go / oodle)

SAV 头(参考 `compressor/__init__.py::_parse_sav_header`):
- `uncompressed_len u32 | compressed_len u32 | magic[3] | save_type u8`,data_offset=12。
- `CNK`(magic=="CNK")时前 12B 为占位,真实头在 12..24,data_offset=24。
- magic 分派:`PlZ`/`CNK` → zlib;`PlM` → oodle。

zlib(标准库):
- `PlZ`(save_type 0x32=50):**双层** zlib —— 外层 zlib.decompress 得到中间结果(长度须 == compressed_len),再内层 zlib.decompress 得到最终(长度 == uncompressed_len)。
- `CNK`:单层。

Oodle(`PlM` save_type 0x31=49):`compressed_data = data[12 : 12+compressed_len]`,调用 `oodle.Decompress(compressed, uncompressed_len)`,校验输出长度。

### 2.1 Oodle Kraken 端口(最高风险)

移植目标:`kraken.cpp::Kraken_Decompress(src, src_len, dst, dst_len)`(仅解压;忽略全部 `Compress*`)。Palworld 用 Kraken/Normal,但为稳妥应实现 Kraken 主路径 + 其依赖的熵解码(Huffman、TANS、multi-array)。Mermaid/Selkie/Leviathan/LZNA/BitKnit 若真实存档不涉及则**延后**,遇到未支持的 block 类型时明确报错而非静默出错。

移植要点(C++ → Go 的陷阱):
- 大量 `uint8_t*` 指针算术 → 改为 `[]byte` + 显式 offset;所有越界须显式检查(Go 无 UB,越界即 panic,需转成 error)。
- 位运算/移位保持 `uint32`/`uint64` 语义;注意 C++ 有符号右移与 Go 的差异。
- 前向 + 后向双向 bit reader(Kraken 特有)。
- `memcpy`/`memmove` 重叠拷贝语义(LZ 复制)→ 逐字节循环,不能用 `copy` 处理重叠。
- endianness:小端读取。

**验证策略(去风险关键)**:

> 环境探测结论(2026-07-13):本机**无 C 编译器**(无 gcc/clang/cl/pacman)、**github.com 不可达**(release dll 下不动)、PyPI 无可用预编译 oodle 轮子、`go-oodle` 因 `import "C"` 需 cgo。**故本机无法产出外部 golden 解压结果**。

主验证改为 **GVAS 解析器自校验(oracle-by-parse)**,理由:Kraken 为整缓冲编码,任何一字节解压错误都会在 GVAS 解析时立刻炸(fstring 长度前缀/属性 size 前缀高度自洽)。在数 MB 的 `Level.sav` 上完整解析并**精确落在 4 字节零尾**,几乎不可能在解压有误时发生。判据分层:
1. **长度**:`len(out) == uncompressed_len`(Kraken 自身也带 per-block 校验)。
2. **结构**:输出以 `GVAS` magic 开头;头部字段合法(save_game_version==3 等);属性流所有 size/长度自洽;以 `None` 结束;`trailer == \x00\x00\x00\x00`(AC5)。
3. **语义**:人读字符串抽检 —— 已知玩家目录名对应的 PlayerUId、公会名/玩家名/帕鲁 CharacterID 为合理可读值(AC3)。
4. 覆盖不同大小样本(小 `Players/*.sav`≈9KB、`LevelMeta.sav`、大 `Level.sav`)以触发单/多 block。

**开发期调试便利**:先攻最小样本(`Players/*.sav`),其解压首字节应为 GVAS 头(实测 PlM 流开头附近即出现 `GVAS`,提示首 block 近似 stored/literal,便于早期比对)。

**可选 golden 增强(非阻塞)**:若用户日后在具备构建条件的机器上用参考 `palsav`/`palooz` 产出 `<name>.gvas` 放入 `testdata/golden/`,`kraken_test.go` 将自动启用逐字节比对(存在即测,缺失即跳过)。

> 明确排除:cgo/ooz build-tag 后备**不采用**(违背 C1,用户已确认走纯 Go)。

## 3. GVAS 读取器(gvas/)

对照 `archive.py::FArchiveReader` + `gvas.py`。Go 表示:

```go
type Reader struct { data []byte; pos int; hints map[string]string; custom map[string]DecodeFunc }
```
- 基础读:`I16/U16/I32/U32/I64/U64/F32/F64/Byte/Bool`;`FString`(size<0 → UTF-16LE 去尾 \x00\x00,>0 → ASCII 去尾 \x00,==0 → "");`GUID`(读 16B,按 `archive.py` 的字节重排规则格式化为字符串);`OptionalGUID`;`TArray[T]`。
- Property 值统一用:
```go
type Property struct {
    Type string            // "IntProperty"...
    ID   *string           // optional guid
    Value any              // 具体见下
    ArrayType, KeyType, ValueType, StructType string // 视类型填充
    CustomType string
}
```
- `PropertiesUntilEnd(path)`:循环读 name==“None” 结束;每项 `name/type/size(u64)/property(...)`。
- Map 读取严格复刻:读 key_type/value_type、optional guid、u32(0)、count;`StructProperty` 键默认 `Guid`(`get_type_or(path+".Key","Guid")`),值默认按 hint。
- struct 分派:Vector/Quat/Guid/LinearColor/Color/DateTime,其余 → PropertiesUntilEnd。
- Set / Array(含 StructProperty 数组头:prop_name/prop_type/size/type_name/guid/1B skip)复刻。

**custom_property 触发**:`property()` 中若 `path in custom && path is not nested_caller_path` → 调用自定义解码器读取该属性的完整体(内部再 `internal copy` 出子 reader 解析字节块)。复刻 `nested_caller_path` 语义避免递归死循环。

### 3.1 选择性解析(R4)

`worldSaveData` 是一个大 StructProperty,其内部是 `PropertiesUntilEnd`。每个子属性有 `size(u64)` 前缀。实现 `PropertiesFiltered(wanted set)`:读 name/type/size,若 name 不在集合 → `Seek(+size)` 跳过整块;命中才真正解析。目标集合:
`CharacterSaveParameterMap, GroupSaveDataMap, ItemContainerSaveData, CharacterContainerSaveData, DynamicItemSaveData`。
注意:MapProperty 的 size 覆盖其全部内容(含 key_type/value_type 前导),跳过安全。全量路径仍保留用于 AC5 与调试。

## 4. rawdata 解码器(rawdata/)

均为 `RawData`(ArrayProperty[ByteProperty])字节块 → 子 reader 解析。逐一对照 Python:

- **character.go**:`object=PropertiesUntilEnd` + `unknown_bytes[4]` + `group_id guid` + `trailing_bytes[4]` + 余下 raw。
- **character_container.go**:`player_uid guid + instance_id guid + permission_tribe_id byte` + 余下。空字节块 → nil。
- **item_container.go**:`permission{type_a []byte, type_b []byte, item_static_ids []string}` + 余下。
- **item_container_slots.go**:`slot_index i32 + count i32 + item{static_id fstring, dynamic_id{created_world_id guid, local_id_in_created_world guid}}` + trailing。
- **dynamic_item.go**:`id{created_world_id, local_id_in_created_world, static_id}` + 试探 egg / armor(剩 12B:4B+float+4B)/ weapon(4B+float durability+i32 bullets+[]string passives+4B)/ unknown(raw)。复刻 `try_read_egg` 的回退(seek 恢复)。
- **group.go**:`group_id + group_name + individual_character_handle_ids[]{guid,instance_id}`;按 group_type 分支;`EPalGroupType::Guild` 读 guild 字段 + `_read_guild_tail`(**v2 试探→失败 seek 回退 v1**,以 EOF 命中为判据);`IndependentGuild` 分支。**必须复刻试探回退逻辑**。

## 5. 语义映射(model.go / extract.go)

> 属性 key 名以真实解码结果为准,下表为依据 palworld-save-tools/PalEdit 生态的**预期名**,实现时以 golden 解码校正。

角色(Level.sav `CharacterSaveParameterMap`):Key = struct{`PlayerUId` guid, `InstanceId` guid};Value.RawData.object 内含 `SaveParameter`(StructProperty)。SaveParameter 关注字段:
- 通用:`CharacterID`(NameProperty,物种;玩家为 Player 系列)、`NickName`(Str)、`Level`(Int)、`Exp`(Int)、`Gender`(Enum `EPalGenderType`)、`IsPlayer`(Bool)、HP(历史为 `HP` FixedPoint64 / 新版 `Hp`)。
- 帕鲁专有:`OwnerPlayerUId`(Struct Guid)、`OldOwnerPlayerUIds`、`Rank`(Int,星级)、`Rank_HP/Rank_Attack/Rank_Defence/Rank_CraftSpeed`、`Talent_HP/Talent_Melee/Talent_Shot/Talent_Defense`(IV,Int)、`PassiveSkillList`(Array Name)、`EquipWaza`(Array Enum)、`MasteredWaza`(Array Enum)、`SlotID`(容器槽)。
- `IsPlayer==true` → Player;否则 Pal。玩家 Player 结构额外取 `PlayerUId`(来自 Key)。

玩家档(`Players/<uid>.sav` `SaveData`):`PlayerUId`、`IndividualId{PlayerUId,InstanceId}`、`InventoryInfo`/`inventoryInfo`(各容器 ID:`CommonContainerId/DropSlotContainerId/EssentialContainerId/WeaponLoadOutContainerId/PlayerEquipArmorContainerId/FoodEquipContainerId` → 每个 `{ID: guid}`)。

背包组装(R6):Player 的容器 GUID → 在 Level.sav `ItemContainerSaveData`(Key=struct{ID guid})查到容器 → `.Slots.Slots.RawData`(item_container_slots)得到各槽 static_id+count+dynamic_id → 用 dynamic_id 到 `DynamicItemSaveData` 取耐久/子弹/蛋/被动。

公会(`GroupSaveDataMap` → group.go,`EPalGroupType::Guild`):`guild_name`、`base_camp_level`、`admin_player_uid`、`players[]{player_uid, player_info{last_online_real_time, player_name}, role?}`、`base_ids`、`individual_character_handle_ids`(公会拥有的角色 instance,可关联帕鲁归属)。

语义结构体(model.go)字段以 prd.md R4 为准;所有结构体保留 `Raw map[string]any`(可选)承载未映射字段,便于后续扩展与调试。

## 6. 数据流

```
LoadLevel(path)
  → 读文件 → parseHeader → 解压(oodle/zlib) → GVAS.Read(header)
  → reader.PropertiesFiltered(worldSaveData → 5 目标 Map)
  → 各 Map 用 custom decoder 展开
  → extract: 遍历角色分 Player/Pal;遍历 Group 取 Guild;建立 container 索引
LoadPlayer(path) → 同解压/GVAS → 取 SaveData.InventoryInfo 容器 ID
Level.AttachInventories(players) → 交叉查表组装背包
```

## 7. 兼容与取舍

- 平台:纯 Go,Windows/Linux 均可编译(C2)。
- 字符串编码:UTF-16LE / ASCII 按 size 符号判定,与 Python 一致;非法字节尽量保留(surrogate 情形记录 warning)。
- 浮点 NaN/Inf:默认保留(allow_nan=true 等价);语义层做数值时再处理。
- 错误处理:解析错误返回 error 并带 offset/path 上下文;绝不 panic 逃逸到调用方(内部 recover 转 error)。
- 只读:全程不写 `.sav`(C4)。

## 8. 风险与缓解

| 风险 | 缓解 |
|---|---|
| Oodle 端口正确性 | golden 逐字节测试;分阶段(先小文件单 block,再大文件多 block) |
| 无法本机产出 golden | 外部一次性 wrapper(go-oodle 下载 dll)或请用户协助;design 已列回退 |
| 属性 key 名/版本漂移 | 以真实 golden 解码校正;未知字段留 Raw;group tail v1/v2 已有试探 |
| 大文件性能 | 选择性解析跳过无关块;`[]byte` 零拷贝子切片;避免反射 |
| Kraken 之外的 block 类型 | 明确报错并记录类型,按需再补 Mermaid/Selkie |
