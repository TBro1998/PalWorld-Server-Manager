# 服务器备份管理

## Goal

为每个 Palworld 服务器提供存档备份闭环：用户可手动创建备份、配置定时自动备份、按可配置策略保留/清理历史备份、从某个备份恢复，并将备份打包下载到本地。

用户价值：避免存档损坏、回档、误操作导致的世界数据丢失，提供可自助运维的备份-恢复能力。

## Background / Confirmed Facts（代码已确认）

- 存档目录：`<installPath>/Pal/Saved/SaveGames/0/<worldid>/{Level.sav, Players/}`，由 `internal/palsave/locate.go:LocateWorld` 定位；多 world 时取第一个含 `Level.sav` 的目录。
- 配置目录：`<installPath>/Pal/Saved/Config/WindowsServer/`（`PalWorldSettings.ini`、`whitelist.txt` 等），见 `internal/palconfig/ini.go:ConfigDir`、`internal/api/whitelist_handlers.go:19`。
- 服务器模型：`internal/models/server.go` 一条 `servers` 记录，含 `install_path`、`status`(派生)、`pid`。
- 进程/状态：`internal/process/manager.go` — `IsRunning`、`DeriveStatus`、`StopServer`；运行态 = 内存 running map + DB pid。
- REST 代理：`internal/api/rest_handlers.go` 提供 `RestSave`（触发游戏内存档落盘），可在热备份前调用。
- 数据库：SQLite + GORM `AutoMigrate`（`internal/database/database.go`），新增表只需加入 `migrate()` 的 model 列表，additive/幂等。
- 路由/鉴权：`internal/api/router.go`，`protected` 组已挂 JWT；已有 `/servers/:id/{config,save,whitelist,stats,rest}` 子路由。
- 后台任务装配：`internal/server/server.go:setupRoutes` 在启动时做 reconcile + `Checker().StartBackgroundCheck()`，是注册备份调度器的位置。
- SSE 流式：`internal/logger` StreamManager（长任务进度可复用，但备份/恢复为同步短任务，MVP 用普通 JSON 响应）。
- 配置系统：`internal/config/config.go`，字段带 `yaml`+`env` 双 tag，`Load()` 优先级 env > yaml > default；已有 `log_dir`(默认 `./logs`) 的"全局根目录"范式。项目**无** `config.example.yaml`。
- 前端占位已存在：`ui/src/app/servers/manage/page.tsx` 有 `backup` tab；`ui/src/components/server-manage/BackupSection.tsx` 预留三面板（sync/auto/manage）；i18n 键在 `serverManage.backup`（zh/en/ja）。
- 游戏原生 `bIsUseBackupSaveData`（`internal/palconfig/schema.go:59`）是游戏自身开关，与本功能无关，不复用。
- 平台：以 Windows 为准（CLAUDE.md）；Linux 路径保留、不破坏。

## Requirements

- **R1 手动备份**：对指定服务器一键创建备份。备份范围可由用户选择：存档 world 目录 / 配置目录 / 二者。
- **R2 定时自动备份**：每服务器可独立配置固定间隔（分钟）+ 备份范围，可启用/停用；进程内调度器到期自动触发。手动与自动并存。
- **R3 保留策略可配置**：支持"保留最近 N 个"与"保留最近 N 天"，两者可分别设置（0=不限）；每次备份成功后按策略清理旧备份。
- **R4 恢复**：可从某历史备份恢复。恢复要求服务器已停止；恢复前自动对当前存档/配置做一次安全备份（来源标记 pre-restore）。
- **R5 存储与下载**：备份以单个 `.zip` 落盘在全局 `backup_dir`（默认 `./backups`），布局 `<backup_dir>/<serverID>/<backupID>.zip`；提供下载接口返回该 zip。
- **R6 元数据**：新增 `backups` DB 表为列表/查询的权威来源；启动时与磁盘对账（磁盘缺失→标记/清理孤儿记录，记录缺失的 zip→忽略或补录，见 design）。
- **R7 一致性**：创建备份时若服务器运行中且 REST 可用，先尽力调用 `RestSave` 落盘再打包，并在结果中标注 hot 备份；REST 不可用则直接热打包并告警。

## Decisions（原 Open Questions，已按推荐方案定稿）

- Q1 存储位置 → 新增全局配置 `backup_dir`（默认 `./backups`），布局 `<backup_dir>/<serverID>/<backupID>.zip`。独立于 `install_path`，避免随服务器重装/删除误删；对齐 `log_dir` 范式。
- Q2 打包格式 → 创建时即压缩为单个 `.zip` 落盘（非目录快照）。单文件、大小可知、下载零额外打包。
- Q3 调度形态 → 每服务器独立、固定间隔（分钟），进程内 goroutine ticker，在 `server.go` 启动装配。cron 表达式复杂度更高，留待后续。
- Q4 保留策略 → 数量(N 个) + 天数(N 天) 双维度，各自可关（0=不限），取"更严格者"清理。总容量上限 out of scope。
- Q5 恢复安全 → 强制要求服务器 stopped（运行中拒绝并提示先停服）；恢复前自动 pre-restore 安全备份。
- Q6 热备份一致性 → 见 R7。
- Q7 元数据存储 → DB 表 `backups` 为权威来源；zip 内附 `manifest.json`（便于脱离本工具辨识），启动对账。

## Acceptance Criteria

- [ ] AC1（R1）手动触发备份后，`<backup_dir>/<serverID>/<backupID>.zip` 生成，含用户所选范围文件，`backups` 表新增一条 source=manual 记录。
- [ ] AC2（R5/R6）列表 API 返回某服务器全部备份，含 id、创建时间、大小、范围(scope)、来源(manual/auto/pre-restore)、hot 标记。
- [ ] AC3（R2/R3）启用间隔自动备份，到期后自动生成 source=auto 备份；数量/天数策略生效并清理超限旧备份。
- [ ] AC4（R4）对运行中服务器发起恢复被拒绝并提示；停服后恢复成功，恢复前生成 pre-restore 备份，目标存档/配置被替换为备份内容。
- [ ] AC5（R5）下载接口返回该备份 zip，浏览器可保存，文件可正常解压。
- [ ] AC6（前端）BackupSection 三面板可完成创建/列表/下载/删除/恢复/自动备份配置；zh/en/ja i18n 完整。
- [ ] AC7（质量）Windows 下 `go build .` 与 `cd ui && bun run build` 通过；`internal/backup` 核心逻辑（打包、保留策略、对账）有单元测试，对齐现有 `_test.go` 约定。

## Out of Scope

- 远程/云存储（S3、对象存储、异地同步）。
- 增量/去重备份（初版全量 zip）。
- cron 表达式级调度、总容量上限策略。
- 跨服务器批量恢复。
