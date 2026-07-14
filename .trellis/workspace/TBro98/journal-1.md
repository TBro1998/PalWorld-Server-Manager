# Journal - TBro98 (Part 1)

> AI development session journal
> Started: 2026-07-12

---



## Session 1: 合并编辑与配置界面并移除RCON

**Date**: 2026-07-12
**Task**: 合并编辑与配置界面并移除RCON
**Branch**: `main`

### Summary

将服务器卡片的编辑+配置两个按钮/弹窗合并为单个'设置'齿轮按钮 + 单弹窗多标签(ServerSettingsDialog)。数据来源归一到实际生效的 launch_args+PalWorldSettings.ini，移除 servers 表惰性列 port/query_port/rcon_port/rcon_enabled(含幂等 dropColumnIfExists 迁移)。彻底移除已弃用 RCON(出 schema+DB+前端+i18n)，REST API 参数保留。名称统一到 servers.name 并在已安装时同步 INI ServerName。publicIP/publicPort/logFormat 归一到 INI。顺带迁移到 ESLint9 flat config 并修复暴露的8个 lint 错。全部构建/测试/lint 通过。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `d0d4e2e` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 2: 服务器 REST API 控制功能

**Date**: 2026-07-13
**Task**: 服务器 REST API 控制功能
**Branch**: `feat/rest-api-control`

### Summary

实现通过 Palworld 官方 REST API 控制服务器：后端新增 internal/palapi 客户端包（11 端点、Basic Auth、5s 超时、错误归一化、单测）与 /servers/:id/rest/* 代理端点（端口/密码即时读 INI，不入库，status 不泄露密码）；前端移除独立 REST 分区，能力分散到概览（只读+手动刷新）、玩家（表+踢/封/解封）、运维（公告/保存/优雅关服/立即停止），新增共享 useRestStatus hook 与 RestUnavailableNotice，破坏性操作二次确认，三语文案补齐。go build/test 与 bun lint/build 均通过。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `5d1fd0a` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete

---

## 2026-07-13 — 纯Go读取Palworld存档(palsav-core-lib)

### 概述
分析 `temp/palsav`(palworld-save-tools 的 Oodle 分支)完整解码链;建 Trellis 父任务 `07-13-palworld-sav-reader-go` + 子任务 A `palsav-core-lib`(核心库)/ B `palsav-api-ui`(REST+前端)。开始实现子任务 A。

### 关键结论
- 本项目真实存档为 **`PlM1`(0x31 = Oodle Kraken)**,非 zlib —— 必须纯 Go 移植 Oodle 解压(`palooz/ooz/dep/ooz/kraken.cpp`,~4.5k 行)。
- 环境探测:本机**无 C 编译器**、**github 不可达**、PyPI 无预编译 oodle 轮子 → 拿不到外部 golden。验证改为 **GVAS 解析器自校验(oracle-by-parse)**:解压长度精确 + 完整解析为合法 GVAS + 4 字节零尾 + 人读字符串抽检。

### 本次改动
- `internal/palsave/sav.go` — SAV 容器头解析(12B/CNK 24B)+ 压缩分派。
- `internal/palsave/compress_zlib.go` — PlZ 双层 / CNK 单层 zlib(标准库)。
- `internal/palsave/oodle/{oodle.go,kraken.go,bitreader.go}` — Oodle 包骨架 + **BitReader 全套 + Kraken 块/quantum 头解析**(已移植)。
- `internal/palsave/testdata/{Player,LevelMeta,Level}.sav` — 真实样本。
- `internal/palsave/sav_test.go` — 头部解析对真实存档校验。

### 测试
- `go test ./internal/palsave/ -run TestParseHeader` **PASS**(3 个真实存档 header 全部正确)。
- `CGO_ENABLED=0 go build ./internal/palsave/...` 通过;`go vet ./internal/palsave/oodle/` 干净。

### 状态
[进行中] Oodle Kraken 端口:BitReader+头解析已完成;**待移植** DecodeBytes/Huffman/TANS/MultiArray(1c)与 Quantum/LzRuns/UnpackOffsets/DecodeStep/Decompress(1d,门禁 G1)。

### 下一步
- 移植熵解码(Huffman 三流 + TANS + Golomb-Rice + MultiArray)。
- 移植 Kraken 主循环 + LZ 复制,跑 G1:LevelMeta.sav 解压 len==uncompressed_len 且首 4B=="GVAS"。

### 2026-07-14 更新 — 门禁 G1 达成(解压层完成)
- **关键更正**:实测存档块头 `8c 0a` → **decoder_type 10 = Mermaid**,非 Kraken。已同时移植 Kraken(6)+ Mermaid(10);LZNA/BitKnit/Leviathan(5/11/12)→ ErrUnsupported。
- 新增 `internal/palsave/oodle/{helpers,huffman,tans,decodebytes,mermaid}.go` + 重写 `kraken.go`(主循环/LZ/quantum/DecodeStep)。oodle 包 ~3500 行。
- **独立复核**:`CGO_ENABLED=0 go build ./...` 通过、`go vet` 干净;`TestDecompressAll` 三存档精确解压(2122/9662/528456)且以 `GVAS` 开头。Level.sav 解压 ~0.4ms。
- 端口经 trellis-implement 子代理完成(两次 resume 收尾)。下一步:阶段 2 GVAS 只读读取器。

### 2026-07-14 — 阶段 2–4 完成(核心库端到端打通)
- 新增 `internal/palsave/gvas/`(通用只读 GVAS 读取器:types/reader/header/property/gvasfile)+ `internal/palsave/{paltypes,rawdata,model,extract}.go`。
- **端到端真实数据验证(G4)**:玩家 "tbro";公会 "无名公会"(UTF-16 正确解码)、成员 1、baseCamp 1(guild tail v2 试探命中 EOF);背包 6 个容器 GUID 全部解析,GUID→slots→物品 join 打通(如 Money x935 / FragGrenade)。
- G2 trailer=00000000;G3 Character=1/Group=8/ItemContainer=27/CharContainer=2;性能 Level.sav 516µs。`CGO_ENABLED=0 go build ./...`、vet、gofmt 全干净;go.mod 无 cgo/oodle/purego 依赖。
- **已知验证缺口**:样本存档仅 1 角色(玩家)、0 帕鲁、个人背包空 → 帕鲁字段抽取与物品填充仅结构性验证;帕鲁字段名依 Palworld 通用 schema,待含帕鲁存档复核。
- 阶段 2–4 经 trellis-implement 子代理完成;已派 trellis-check 复核质量。

### 2026-07-14 — 阶段 5 / trellis-check 完成
- trellis-check(子代理)复核 ~24 文件:确认对参考实现高保真、真实存档字节精确;GUID/fstring(UTF-16 实测解出 `最大HP`)/Map 默认类型/custom nested_caller_path/guild v2→v1 EOF 回退/dynamic_item 试探/Mermaid 重叠 matchCopy 全部对齐。
- 修复:删 bitreader.go 3 个死函数;收紧 sav_test.go 过时的软 return。
- 保留(低优先,有理由):player.Level 缺失→0(生态惯例默认 1,属语义消费方决定,prd R5 允许留空);try_read_egg EOF-re-raise 差异(样本无蛋无法触发);fstring ASCII 路径(格式不产生 >127 正长度)。
- 复核后 `CGO_ENABLED=0 go build ./...`、vet、gofmt、`go test ./internal/palsave/...` 全绿。
- git:`Servers/`(真实存档)已被忽略;`internal/palsave/testdata/` 小样本(含真实玩家/公会名)未忽略未提交,待用户决定是否作为 fixture 入库。
- 子任务 A(核心库)实现 + 检查完成。待:commit(需用户批准)→ 规划/开始子任务 B(REST+前端)。
