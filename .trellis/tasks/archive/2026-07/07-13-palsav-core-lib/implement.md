# Implement — Palworld 存档核心解析库(纯Go)

执行顺序按依赖与风险排列。每阶段末有验证/门禁。所有命令在项目根运行。

## 阶段 0 — 脚手架 + 样本(去风险前置)

- [ ] 0.1 建包 `internal/palsave/`,`go.mod` 确认无新增 cgo 依赖。
- [ ] 0.2 收集样本:从 `Servers/1/.../backup/.../` 复制若干真实 `.sav` 到 `internal/palsave/testdata/`(小:`Players/*.sav`;中:`LevelMeta.sav`;大:`Level.sav`)。选最小可行样本,`.gitignore` 排除超大文件。
- [ ] 0.3 验证策略采 **oracle-by-parse**(见 design §2.1)——本机无法产外部 golden。留 `testdata/golden/` 目录:若日后有人放入 `<name>.gvas`,测试自动启用逐字节比对(存在即测)。
- [ ] **门禁**:样本齐备,记录各样本 header(uncompressed_len / save_type)。

## 阶段 1 — 压缩层

- [ ] 1.1 `sav.go`:parseHeader(12B/CNK 24B)、magic 分派、长度校验。
- [ ] 1.2 `compress_zlib.go`:PlZ 双层 / CNK 单层(`compress/zlib`)。
- [ ] 1.3 `oodle/` 端口(核心):bitreader → huffman → tans → kraken 主循环。分步:
  - 1.3a bit reader(前向/后向)+ 单元测试。
  - 1.3b Huffman 解码 + multi-array。
  - 1.3c TANS 解码。
  - 1.3d Kraken quantum/block 主循环 + LZ 复制(重叠安全)。
- [ ] 1.4 `oodle.Decompress(compressed, outLen)`,遇未支持 block 类型显式 error。
- [ ] **门禁(G1 可行性确认)**:`oodle.Decompress` 对最小样本 `Players/*.sav` 输出 `len==uncompressed_len` 且前 4 字节 == `GVAS`;对全部样本长度精确。有 golden 时额外逐字节比对(AC1)。这是本子任务最关键门禁。

## 阶段 2 — GVAS 读取器

- [ ] 2.1 `gvas/types.go` 值模型;`gvas/reader.go` 基础读 + fstring + guid(重排)+ tarray。
- [ ] 2.2 `gvas/header.go`:GvasHeader.Read(magic 校验 1396790855、版本==3、custom_version_format==3)。
- [ ] 2.3 `gvas/property.go`:15 种 Property + struct/map/array/set + custom 触发(nested_caller_path 语义)。
- [ ] 2.4 `PropertiesFiltered`(选择性跳过,按 size Seek)。
- [ ] **门禁**:对某样本走**全量**解析,trailer==`\x00\x00\x00\x00`(AC5);解析无 error。

## 阶段 3 — type_hints + rawdata 解码器

- [ ] 3.1 `paltypes.go`:移植四类数据相关的 TYPE_HINTS 条目 + custom registry。
- [ ] 3.2 `rawdata/character.go`、`character_container.go`。
- [ ] 3.3 `rawdata/item_container.go`、`item_container_slots.go`、`dynamic_item.go`(egg/armor/weapon/unknown 试探)。
- [ ] 3.4 `rawdata/group.go`(guild tail v2→v1 试探回退,EOF 判据)。
- [ ] **门禁**:5 个目标 Map 全部解码无 error;各 Map 计数 > 0(真实存档应有玩家/帕鲁/物品)。

## 阶段 4 — 语义映射 + 组装

- [ ] 4.1 `model.go` 结构体(Player/Pal/Guild/Item/Inventory,+ 可选 Raw)。
- [ ] 4.2 `extract.go`:角色分流 Player/Pal;字段映射(以 golden 解码校正 key 名);Guild 提取。
- [ ] 4.3 `LoadPlayer`:Players/*.sav 的 InventoryInfo 容器 ID。
- [ ] 4.4 `AttachInventories`:容器 GUID 交叉查表 → 背包物品 + dynamic 明细。
- [ ] **门禁(AC3)**:抽样断言 —— 已知玩家名 / 公会名 / 至少一只帕鲁 CharacterID / 背包若干 static_id 与参考一致。

## 阶段 5 — 验证 harness + 收尾

- [ ] 5.1 小 CLI 或 `Example`/表格测试:打印四类数据摘要。
- [ ] 5.2 性能实测:大 Level.sav 解析计时,记录到测试日志(AC4)。
- [ ] 5.3 `CGO_ENABLED=0 go build ./...` + `go vet ./internal/palsave/...`(AC2)。
- [ ] 5.4 清理:确认无外部原生依赖入产物;testdata golden 生成脚本标注"仅开发期"。

## 验证命令

```bash
# 纯 Go 构建门禁 (AC2)
CGO_ENABLED=0 go build ./...
go vet ./internal/palsave/...
# Oodle golden 逐字节 (AC1)
go test ./internal/palsave/oodle/ -run Golden -v
# 端到端解析 + 语义抽样 (AC3/AC5)
go test ./internal/palsave/ -run "TestLevel|TestPlayer|TestGuild|TestInventory" -v
# 性能 (AC4)
go test ./internal/palsave/ -run TestLevelPerf -v -timeout 120s
```

## Rollback / 回退点

- 每阶段独立提交(阶段间可回退)。
- Oodle 端口若阻塞(阶段 1 门禁不过)→ 上报父任务,评估临时 cgo 后备(build tag 隔离,需用户确认,违背 C1)。
- 语义 key 名不确定处先保留 Raw,后续增量校正,不阻塞整体链路跑通。

## 审查门禁(Review Gates)

- G1(阶段 1 后):AC1 golden 通过 —— **可行性确认点**,通过才继续。
- G2(阶段 3 后):目标 Map 解码无 error。
- G3(阶段 4 后):AC3 语义抽样通过。
- G4(阶段 5):AC2/AC4 + 全面 check。
